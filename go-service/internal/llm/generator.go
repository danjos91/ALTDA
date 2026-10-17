package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/content"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
)

// Generator is an LLM-backed content.Generator (one topic per call).
// On LLM failure/timeout after retries it falls back to the stub generator
// for that topic and records a warning for the methodologist.
type Generator struct {
	client *Client
	stub   content.Generator

	mu       sync.Mutex
	warnings []validate.Check
}

func NewGenerator(client *Client) content.Generator {
	return &Generator{client: client, stub: content.NewTemplateGenerator()}
}

func (g *Generator) Placeholder() bool { return false }

// ConsumeWarnings returns and clears soft-fail warnings from topic fallbacks.
func (g *Generator) ConsumeWarnings() []validate.Check {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := g.warnings
	g.warnings = nil
	return out
}

type topicResponse struct {
	Activities []program.Activity `json:"activities"`
	Lecture    *program.Lecture   `json:"lecture"`
	Questions  []topicQuestion    `json:"questions"`
	Task       *topicTask         `json:"task"`
}

type topicQuestion struct {
	Type        string           `json:"type"`
	Text        string           `json:"text"`
	Options     []program.Option `json:"options"`
	Correct     []string         `json:"correct"`
	Explanation string           `json:"explanation"`
	OutcomeIDs  []string         `json:"outcomeIds"`
	Difficulty  string           `json:"difficulty"`
}

type topicTask struct {
	Title         string              `json:"title"`
	Description   string              `json:"description"`
	Deliverable   string              `json:"deliverable"`
	DurationHours float64             `json:"durationHours"`
	OutcomeIDs    []string            `json:"outcomeIds"`
	Criteria      []program.Criterion `json:"criteria"`
	MaxScore      int                 `json:"maxScore"`
}

func (g *Generator) Topic(ctx context.Context, in content.TopicInput) (content.TopicContent, error) {
	start := time.Now()
	out, err := g.topicLLM(ctx, in)
	elapsed := time.Since(start)
	if err == nil {
		slog.InfoContext(ctx, "topic llm ok",
			slog.String("topicId", in.Topic.ID),
			slog.Duration("duration", elapsed),
		)
		return out, nil
	}

	slog.WarnContext(ctx, "topic llm failed; using stub",
		slog.String("topicId", in.Topic.ID),
		slog.Duration("duration", elapsed),
		slog.Any("error", err),
	)
	stubOut, stubErr := g.stub.Topic(ctx, in)
	if stubErr != nil {
		return content.TopicContent{}, stubErr
	}
	g.addFallbackWarning(in.Topic.ID, err)
	return stubOut, nil
}

func (g *Generator) topicLLM(ctx context.Context, in content.TopicInput) (content.TopicContent, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return content.TopicContent{}, fmt.Errorf("%w: marshal topic: %v", ErrUpstream, err)
	}
	user := "Заполни содержание темы. JSON входа:\n" + string(body)

	var raw topicResponse
	if err := g.client.ChatJSON(ctx, topicSystemPrompt, user, &raw); err != nil {
		return content.TopicContent{}, err
	}
	if err := validateTopicResponse(in, raw); err != nil {
		return content.TopicContent{}, err
	}

	out := content.TopicContent{
		Activities: raw.Activities,
		Lecture:    raw.Lecture,
	}
	for _, q := range raw.Questions {
		out.Questions = append(out.Questions, program.Question{
			Type:        q.Type,
			Text:        q.Text,
			Options:     nodesOf(q.Options),
			Correct:     q.Correct,
			Explanation: q.Explanation,
			OutcomeIDs:  q.OutcomeIDs,
			Difficulty:  q.Difficulty,
		})
	}
	if raw.Task != nil {
		out.Task = &program.SeminarTask{
			Title:         raw.Task.Title,
			Description:   raw.Task.Description,
			Deliverable:   raw.Task.Deliverable,
			DurationHours: raw.Task.DurationHours,
			OutcomeIDs:    raw.Task.OutcomeIDs,
			Criteria:      criterionNodes(raw.Task.Criteria),
			MaxScore:      raw.Task.MaxScore,
		}
	}
	return out, nil
}

func (g *Generator) addFallbackWarning(topicID string, cause error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.warnings = append(g.warnings, validate.Check{
		RuleID:     "LLM_TOPIC_FALLBACK",
		Category:   validate.CategoryConsistency,
		Severity:   validate.SeverityWarning,
		Passed:     false,
		Message:    fmt.Sprintf("Тема %s: LLM недоступен (%v); подставлен шаблонный текст — перегенерируйте тему", topicID, cause),
		Path:       "/curriculum/modules/*/topics/" + topicID,
		RelatedIDs: []string{topicID},
	})
}

func validateTopicResponse(in content.TopicInput, raw topicResponse) error {
	if len(raw.Activities) == 0 {
		h := in.Topic.Hours
		if h.Lecture+h.Practice+h.Lab+h.SelfStudy+h.Assessment > 0 {
			return fmt.Errorf("%w: missing activities", ErrSchemaInvalid)
		}
	}
	for _, a := range raw.Activities {
		if a.Kind == "" || a.Content == "" {
			return fmt.Errorf("%w: activity missing kind/content", ErrSchemaInvalid)
		}
	}
	return nil
}

func nodesOf(opts []program.Option) []program.Node[program.Option] {
	out := make([]program.Node[program.Option], 0, len(opts))
	for _, o := range opts {
		out = append(out, program.Node[program.Option]{V: o})
	}
	return out
}

func criterionNodes(c []program.Criterion) []program.Node[program.Criterion] {
	out := make([]program.Node[program.Criterion], 0, len(c))
	for _, x := range c {
		out = append(out, program.Node[program.Criterion]{V: x})
	}
	return out
}
