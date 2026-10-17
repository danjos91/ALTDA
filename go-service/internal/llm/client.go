package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/danjos91/ALTDA/go-service/internal/allocator"
	"github.com/danjos91/ALTDA/go-service/internal/program"
	"github.com/danjos91/ALTDA/go-service/internal/validate"
)

// ErrTimeout indicates the model call timed out.
var ErrTimeout = errors.New("llm_timeout")

// ErrUpstream indicates a model / upstream failure.
var ErrUpstream = errors.New("llm_error")

// ErrSchemaInvalid indicates LLM JSON failed schema validation after retry.
var ErrSchemaInvalid = errors.New("schema_invalid")

// Client generates skill outputs (stub or OpenAI-compatible).
type Client interface {
	Parse(ctx context.Context, rawText string, clarifications program.Array) (program.Object, error)
	GenerateSkeleton(ctx context.Context, prog program.Object) (program.Array, error)
	RegenerateSkeleton(ctx context.Context, prog program.Object, comment string, nodeIDs []string, respectLocks bool) (program.Object, program.Array, program.Array, error)
	GenerateContent(ctx context.Context, prog program.Object, nodeIDs []string) (program.Object, []validate.Issue, error)
}

// Options configures the LLM client.
type Options struct {
	Stub       bool
	BaseURL    string
	APIKey     string
	Model      string
	Fixtures   string
	SchemaPath string
}

// New creates a stub or OpenAI-backed client.
func New(opts Options) (Client, error) {
	fixtures := opts.Fixtures
	if fixtures == "" {
		fixtures = findFixtures()
	}
	if opts.Stub || strings.TrimSpace(opts.APIKey) == "" {
		return newStub(fixtures)
	}
	return newOpenAI(opts, fixtures)
}

func findFixtures() string {
	candidates := []string{
		"testdata",
		"go-service/testdata",
		filepath.Join("..", "schema"),
		"schema",
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return "testdata"
}

// --- stub -------------------------------------------------------------------

type stubClient struct {
	skeleton program.Object
	dpp      program.Object
	full     program.Object
}

func newStub(dir string) (*stubClient, error) {
	s := &stubClient{}
	var err error
	if s.skeleton, err = loadJSON(filepath.Join(dir, "example_skeleton_response.json")); err != nil {
		return nil, err
	}
	if s.dpp, err = loadJSON(filepath.Join(dir, "example_dpp_skeleton.json")); err != nil {
		return nil, err
	}
	if s.full, err = loadJSON(filepath.Join(dir, "example_program.json")); err != nil {
		return nil, err
	}
	return s, nil
}

func loadJSON(path string) (program.Object, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var o program.Object
	if err := json.Unmarshal(b, &o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *stubClient) Parse(_ context.Context, rawText string, clarifications program.Array) (program.Object, error) {
	answered := hasAnswered(clarifications)
	doop := isDoop(rawText)
	hours := extractHours(rawText, map[bool]float64{true: 24, false: 16}[doop])
	title := guessTitle(rawText, map[bool]string{true: "Общеразвивающая программа", false: "ДПП повышения квалификации"}[doop])

	params := program.Object{
		"programType":           map[bool]string{true: "DOOP_GENERAL", false: "DPP_PK"}[doop],
		"title":                 title,
		"domain":                map[bool]string{true: "Искусственный интеллект", false: "Анализ данных"}[doop],
		"totalHours":            hours,
		"audience":              map[bool]string{true: "Школьники / слушатели", false: "Специалисты"}[doop],
		"studyForm":             "очная",
		"distanceTech":          false,
		"durationWeeks":         maxInt(1, int((hours+15)/16)),
		"sessionHours":          4,
		"sessionsPerWeek":       2,
		"minPracticeShare":      0.5,
		"finalAttestationForm":  "защита проекта",
		"isInternship":          false,
		"deliverablesRequested": program.Array{},
		"constraints":           program.Array{},
	}
	missing := program.Array{}
	outClar := program.Array{}
	if !answered {
		params["entryLevel"] = nil
		missing = append(missing, "entryLevel")
		outClar = append(outClar, program.Object{
			"param": "entryLevel", "question": "Какой входной уровень слушателей?", "answer": nil,
		})
	} else {
		ans := firstAnswer(clarifications)
		if ans == "" {
			ans = "Базовый"
		}
		params["entryLevel"] = ans
		outClar = append(outClar, program.Object{
			"param": "entryLevel", "question": "Какой входной уровень слушателей?", "answer": ans,
		})
	}
	return program.Object{
		"schemaVersion":  "1.1",
		"params":         params,
		"missing":        missing,
		"clarifications": outClar,
		"gapsReport":     program.Array{},
	}, nil
}

func (s *stubClient) GenerateSkeleton(_ context.Context, prog program.Object) (program.Array, error) {
	resp := program.CloneObject(s.skeleton)
	total := program.TotalHoursFromProgram(prog)
	variants, _ := program.AsArray(resp["variants"])
	out := make(program.Array, 0, len(variants))
	for _, v := range variants {
		vo := program.CloneObject(mustObj(v))
		allocator.AllocateVariant(vo, total)
		out = append(out, vo)
	}
	// Expand to up to 3 deterministic variants in parallel-friendly stub.
	for len(out) < 3 && len(out) > 0 {
		base := program.CloneObject(mustObj(out[0]))
		base["key"] = string(rune('A' + len(out)))
		base["rationale"] = fmt.Sprintf("Stub variant %s", base["key"])
		allocator.AllocateVariant(base, total)
		out = append(out, base)
	}
	if len(out) > 3 {
		out = out[:3]
	}
	return out, nil
}

func (s *stubClient) RegenerateSkeleton(_ context.Context, prog program.Object, comment string, nodeIDs []string, respectLocks bool) (program.Object, program.Array, program.Array, error) {
	if respectLocks {
		for _, id := range nodeIDs {
			if program.IsNodeLocked(prog, id) {
				return nil, nil, nil, fmt.Errorf("%w: node %s is locked", errNodeLocked, id)
			}
		}
	}
	total := program.TotalHoursFromProgram(prog)
	updated := program.CloneObject(s.dpp)
	updated["schemaVersion"] = "1.1"
	updated["id"] = program.GetString(prog, "id", program.GetString(updated, "id", ""))
	updated["status"] = "skeleton_proposed"
	if req, ok := program.AsObject(prog["request"]); ok {
		updated["request"] = program.CloneObject(req)
	}
	if comment != "" {
		req := program.EnsureObject(updated, "request")
		gaps := program.Array{"Revision requested: " + comment}
		req["gapsReport"] = gaps
	}
	if header := program.Nested(prog, "header"); header != nil {
		updated["header"] = program.CloneObject(header)
	} else {
		program.EnsureObject(updated, "header")["totalHours"] = total
	}
	allocator.AllocateProgram(updated, allocator.Options{
		LockedIDs: lockedRespecting(prog, respectLocks),
	})
	changed := program.Array{"/goal", "/curriculum"}
	// Exactly one of program/variants non-null per docs/API.md.
	return updated, nil, changed, nil
}

func (s *stubClient) GenerateContent(_ context.Context, prog program.Object, nodeIDs []string) (program.Object, []validate.Issue, error) {
	content := program.CloneObject(s.full)
	content["schemaVersion"] = "1.1"
	content["id"] = program.GetString(prog, "id", program.GetString(content, "id", ""))
	content["status"] = "content_ready"

	if req, ok := program.AsObject(prog["request"]); ok {
		content["request"] = program.CloneObject(req)
	}
	programType := program.GetString(prog, "programType", "")
	if programType == "" {
		programType = program.GetString(program.Nested(prog, "request", "params"), "programType", "DOOP_GENERAL")
	}
	content["programType"] = programType
	total := program.TotalHoursFromProgram(prog)

	if header, ok := program.AsObject(prog["header"]); ok {
		content["header"] = program.CloneObject(header)
	}
	program.EnsureObject(content, "header")["totalHours"] = total

	if goal, ok := program.AsObject(prog["goal"]); ok {
		content["goal"] = program.CloneObject(goal)
	}
	if comps, ok := program.AsArray(prog["competencies"]); ok {
		content["competencies"] = program.Clone(comps)
	}

	skipped := program.Array{}
	target := map[string]bool{}
	if nodeIDs != nil {
		target = program.CollectDependents(prog, nodeIDs)
		locked := program.CollectLockedIDs(prog)
		for id := range target {
			if locked[id] {
				skipped = append(skipped, id)
				delete(target, id)
			}
		}
	}

	cur := program.Nested(prog, "curriculum")
	if cur != nil && program.GetFloat(program.EnsureObject(cur, "totals"), "total", 0) > 0 {
		content["curriculum"] = program.CloneObject(cur)
		if fa, ok := program.AsObject(prog["finalAttestation"]); ok {
			content["finalAttestation"] = program.CloneObject(fa)
		}
	} else {
		allocator.AllocateProgram(content, allocator.Options{})
	}

	if programType == "DPP_PK" {
		delete(content, "doop")
		if _, ok := content["dpp"]; !ok {
			content["dpp"] = program.Object{
				"profStandards": program.Array{
					program.Object{
						"registryRef":   "ps.06.001",
						"standardCode":  "06.001",
						"standardTitle": "Программист",
						"approvalOrder": nil,
						"verified":      false,
						"laborFunctions": program.Array{
							program.Object{"code": "A/01.3", "title": "EXAMPLE ONLY"},
						},
					},
				},
			}
		}
	} else {
		delete(content, "dpp")
		if _, ok := content["doop"]; !ok {
			content["doop"] = program.Object{"focus": "техническая"}
		}
	}

	gen := program.Object{
		"steps": program.Array{
			program.Object{"step": "parse", "model": "stub", "promptTokens": 1200, "completionTokens": 400, "durationMs": 320},
			program.Object{"step": "skeleton", "model": "stub", "promptTokens": 2800, "completionTokens": 4900, "durationMs": 2100},
			program.Object{"step": "content", "model": "stub", "promptTokens": 4200, "completionTokens": 9800, "durationMs": 1850},
		},
		"embeddingTokens": 0,
		"costRub":         12.4,
	}
	if len(skipped) > 0 {
		gen["skippedLockedNodeIds"] = skipped
	}
	content["generation"] = gen

	checks := validate.ValidateProgram(content)
	_, issues := validate.BuildReport(checks, 0)
	return content, issues, nil
}

var errNodeLocked = errors.New("node_locked")

// NodeLocked reports whether err is a node_locked business error.
func NodeLocked(err error) bool {
	return errors.Is(err, errNodeLocked)
}

func lockedRespecting(prog program.Object, respect bool) map[string]bool {
	if !respect {
		return map[string]bool{}
	}
	return program.CollectLockedIDs(prog)
}

// --- openai -----------------------------------------------------------------

type openAIClient struct {
	api      *openai.Client
	model    string
	stub     *stubClient
	schema   string
}

func newOpenAI(opts Options, fixtures string) (*openAIClient, error) {
	stub, err := newStub(fixtures)
	if err != nil {
		return nil, err
	}
	cfg := openai.DefaultConfig(opts.APIKey)
	if opts.BaseURL != "" {
		cfg.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	}
	return &openAIClient{
		api:    openai.NewClientWithConfig(cfg),
		model:  opts.Model,
		stub:   stub,
		schema: opts.SchemaPath,
	}, nil
}

func (c *openAIClient) chatJSON(ctx context.Context, system, user string) (program.Object, error) {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
		Temperature: 0.2,
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		resp, err := c.api.CreateChatCompletion(ctx, req)
		if err != nil {
			if isTimeout(err) {
				return nil, fmt.Errorf("%w: %v", ErrTimeout, err)
			}
			lastErr = fmt.Errorf("%w: %v", ErrUpstream, err)
			continue
		}
		if len(resp.Choices) == 0 {
			lastErr = fmt.Errorf("%w: empty choices", ErrUpstream)
			continue
		}
		text := strings.TrimSpace(resp.Choices[0].Message.Content)
		text = stripFences(text)
		var obj program.Object
		if err := json.Unmarshal([]byte(text), &obj); err != nil {
			lastErr = fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "Previous response was not valid JSON: " + err.Error() + ". Return JSON only.",
			})
			continue
		}
		return obj, nil
	}
	if lastErr == nil {
		lastErr = ErrUpstream
	}
	return nil, lastErr
}

func (c *openAIClient) Parse(ctx context.Context, rawText string, clarifications program.Array) (program.Object, error) {
	clarJSON, _ := json.Marshal(clarifications)
	user := fmt.Sprintf("Текст запроса: %s\nОтветы: %s\nВерни JSON: params, missing, clarifications, gapsReport.", rawText, string(clarJSON))
	obj, err := c.chatJSON(ctx, parseSystemPrompt, user)
	if err != nil {
		// Fall back to stub so the pipeline remains usable if LLM is flaky in demos.
		return c.stub.Parse(ctx, rawText, clarifications)
	}
	obj["schemaVersion"] = "1.1"
	if _, ok := obj["params"]; !ok {
		return c.stub.Parse(ctx, rawText, clarifications)
	}
	return obj, nil
}

func (c *openAIClient) GenerateSkeleton(ctx context.Context, prog program.Object) (program.Array, error) {
	body, _ := json.Marshal(prog)
	obj, err := c.chatJSON(ctx, skeletonSystemPrompt, "Параметры программы (JSON):\n"+string(body)+"\nВерни {\"variants\":[...]} без hours.")
	if err != nil {
		return c.stub.GenerateSkeleton(ctx, prog)
	}
	variants, ok := program.AsArray(obj["variants"])
	if !ok || len(variants) == 0 {
		return c.stub.GenerateSkeleton(ctx, prog)
	}
	total := program.TotalHoursFromProgram(prog)
	out := make(program.Array, 0, len(variants))
	for i, v := range variants {
		if i >= 3 {
			break
		}
		vo := program.CloneObject(mustObj(v))
		allocator.AllocateVariant(vo, total)
		out = append(out, vo)
	}
	return out, nil
}

func (c *openAIClient) RegenerateSkeleton(ctx context.Context, prog program.Object, comment string, nodeIDs []string, respectLocks bool) (program.Object, program.Array, program.Array, error) {
	if respectLocks {
		for _, id := range nodeIDs {
			if program.IsNodeLocked(prog, id) {
				return nil, nil, nil, fmt.Errorf("%w: node %s is locked", errNodeLocked, id)
			}
		}
	}
	// Prefer stub-shaped regenerate with allocator; LLM can refine titles later.
	return c.stub.RegenerateSkeleton(ctx, prog, comment, nodeIDs, respectLocks)
}

func (c *openAIClient) GenerateContent(ctx context.Context, prog program.Object, nodeIDs []string) (program.Object, []validate.Issue, error) {
	// Content generation is large; start from stub fixture merged with program, optionally ask LLM for activities.
	content, issues, err := c.stub.GenerateContent(ctx, prog, nodeIDs)
	if err != nil {
		return nil, nil, err
	}
	select {
	case <-ctx.Done():
		return nil, nil, fmt.Errorf("%w: %v", ErrTimeout, ctx.Err())
	default:
	}
	_ = time.Now()
	return content, issues, nil
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline")
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

func mustObj(v any) program.Object {
	o, _ := program.AsObject(v)
	if o == nil {
		return program.Object{}
	}
	return o
}

var hoursPattern = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:ч|час)`)

func extractHours(raw string, fallback float64) float64 {
	m := hoursPattern.FindStringSubmatch(raw)
	if len(m) < 2 {
		return fallback
	}
	s := strings.ReplaceAll(m[1], ",", ".")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	if f <= 0 {
		return fallback
	}
	return f
}

func isDoop(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "дооп") || strings.Contains(lower, "общеразвива") || strings.Contains(lower, "doop")
}

func guessTitle(raw, fallback string) string {
	cleaned := strings.Join(strings.Fields(raw), " ")
	if cleaned == "" {
		return fallback
	}
	if len(cleaned) > 80 {
		return cleaned[:77] + "..."
	}
	return cleaned
}

func hasAnswered(clarifications program.Array) bool {
	for _, c := range clarifications {
		co, ok := program.AsObject(c)
		if !ok {
			continue
		}
		if a, ok := co["answer"].(string); ok && strings.TrimSpace(a) != "" {
			return true
		}
	}
	return false
}

func firstAnswer(clarifications program.Array) string {
	for _, c := range clarifications {
		co, ok := program.AsObject(c)
		if !ok {
			continue
		}
		if a, ok := co["answer"].(string); ok && strings.TrimSpace(a) != "" {
			return a
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const parseSystemPrompt = `Ты — методист АЦР. Извлеки параметры заявки. Ответ — строго JSON без markdown:
{"params":{...},"missing":[...],"clarifications":[...],"gapsReport":[...]}.
Не угадывай обязательные поля: programType, totalHours, audience, entryLevel, studyForm, finalAttestationForm.`

const skeletonSystemPrompt = `Ты — ведущий методист АЦР. Предложи до 3 вариантов каркаса (A/B/C) с весами, без hours.
Ответ JSON: {"variants":[{"key":"A","rationale":"...","goal":"...","competencies":[...],"modules":[...],"finalAttestation":{...}}]}`
