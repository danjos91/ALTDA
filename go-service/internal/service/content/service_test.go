package content

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
)

// The core guarantee: for any volume and any skeleton variant, the content the
// template writes passes the whole validator. The only findings are the two
// template sections no template can honestly fill.
func TestGeneratedContentPassesTheValidator(t *testing.T) {
	cases := []struct {
		name   string
		hours  float64
		params string
	}{
		{"16 h", 16, ""},
		{"17.5 h", 17.5, ""},
		{"40 h", 40, ""},
		{"72 h", 72, ""},
		{"144 h", 144, ""},
		{"500 h", 500, ""},
		{"distance form", 72, `, "studyForm": "заочная"`},
		{"internship", 72, `, "isInternship": true`},
		{"practice share", 72, `, "minPracticeShare": 0.6`},
		{"session of 3 hours", 72, `, "sessionHours": 3, "sessionsPerWeek": 3`},
	}

	for _, tc := range cases {
		for variant := 0; variant < 3; variant++ {
			t.Run(fmt.Sprintf("%s/variant %d", tc.name, variant), func(t *testing.T) {
				p := approvedProgram(t, tc.hours, variant, tc.params)

				issues := generate(t, p)

				if !onlyMissingSections(issues) {
					t.Errorf("issues = %+v, want only the two SECTION_MISSING (sources, normative basis)", issues)
				}
				if p.V.Status != "content_ready" {
					t.Errorf("status = %q, want content_ready", p.V.Status)
				}
			})
		}
	}
}

func TestEveryTopicGetsItsContent(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	lectures := map[string]bool{}
	for _, l := range p.V.Lectures {
		lectures[l.V.TopicID] = true
	}
	tasks := map[string]bool{}
	for _, task := range p.V.Assessment.V.SeminarTasks {
		tasks[task.V.TopicID] = true
	}

	for _, ref := range topicRefs(&p.V) {
		topic := ref.topic
		if len(topic.Activities) == 0 {
			t.Errorf("%s has no activities", topic.ID)
		}
		if topic.Hours.Lecture > 0 && !lectures[topic.ID] {
			t.Errorf("%s has lecture hours but no lecture", topic.ID)
		}
		if topic.Hours.Lecture == 0 && lectures[topic.ID] {
			t.Errorf("%s has a lecture but no lecture hours", topic.ID)
		}
		if topic.Hours.Practice+topic.Hours.Lab > 0 && !tasks[topic.ID] {
			t.Errorf("%s has practice hours but no seminar task", topic.ID)
		}
	}
}

func TestIDsAreStableAndUnique(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	seen := map[string]string{}
	register := func(kind, id string) {
		if prev, dup := seen[id]; dup {
			t.Errorf("%s ID %q is already used by %s", kind, id, prev)
		}
		seen[id] = kind
	}
	for _, l := range p.V.Lectures {
		register("lecture", l.V.ID)
	}
	for _, task := range p.V.Assessment.V.SeminarTasks {
		register("task", task.V.ID)
	}
	for _, test := range p.V.Assessment.V.Tests {
		register("test", test.V.ID)
		for _, q := range test.V.Questions {
			register("question", q.V.ID)
		}
	}

	if topicByID(p, "T1.1") == nil || p.V.Lectures[0].V.ID != "L1.1" {
		t.Errorf("the first lecture should be L1.1, got %q", p.V.Lectures[0].V.ID)
	}
}

func TestGenerationIsDeterministicAndIdempotent(t *testing.T) {
	first := approvedProgram(t, 72, 1, "")
	generate(t, first)
	before := toJSON(t, first)

	generate(t, first) // regenerate everything over an already complete program
	if after := toJSON(t, first); after != before {
		t.Error("regenerating the whole program changed it")
	}

	second := approvedProgram(t, 72, 1, "")
	generate(t, second)
	if toJSON(t, second) != before {
		t.Error("two runs over the same input gave different programs")
	}
}

func TestFieldsGoDoesNotKnowSurviveGeneration(t *testing.T) {
	p := approvedProgram(t, 40, 0, "")
	// Fields other services own, which the content step must carry through.
	data := toJSON(t, p)
	var raw map[string]any
	if err := jsonUnmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["lineage"] = map[string]any{"operation": "new"}
	raw["header"].(map[string]any)["authors"] = []any{map[string]any{"role": "Методист", "fullName": nil}}
	modules := raw["curriculum"].(map[string]any)["modules"].([]any)
	modules[0].(map[string]any)["annotation"] = "аннотация модуля"
	modules[0].(map[string]any)["topics"].([]any)[0].(map[string]any)["doubts"] = []any{"версия?"}

	var q program.Program
	if err := jsonUnmarshal(mustJSON(t, raw), &q); err != nil {
		t.Fatal(err)
	}
	generate(t, &q)

	var out map[string]any
	if err := jsonUnmarshal(toJSON(t, &q), &out); err != nil {
		t.Fatal(err)
	}
	if out["lineage"] == nil || out["header"].(map[string]any)["authors"] == nil {
		t.Error("top-level or header fields were lost")
	}
	m0 := out["curriculum"].(map[string]any)["modules"].([]any)[0].(map[string]any)
	if m0["annotation"] != "аннотация модуля" || m0["topics"].([]any)[0].(map[string]any)["doubts"] == nil {
		t.Errorf("module or topic fields were lost: %v", m0)
	}
}

func TestSourcesAndNormativeBasisAreNeverInvented(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	if p.V.Sources == nil || len(p.V.Sources) != 0 || p.V.NormativeBasis == nil || len(p.V.NormativeBasis) != 0 {
		t.Errorf("sources = %v, normativeBasis = %v: both must be present and empty", p.V.Sources, p.V.NormativeBasis)
	}
	data := toJSON(t, p)
	var out map[string]any
	if err := jsonUnmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if list, ok := out["sources"].([]any); !ok || len(list) != 0 {
		t.Errorf("sources in the JSON = %v, want []", out["sources"])
	}
}

func TestEveryTemplateTextIsMarked(t *testing.T) {
	p := approvedProgram(t, 72, 0, "")
	generate(t, p)

	for _, ref := range topicRefs(&p.V) {
		for _, a := range ref.topic.Activities {
			if len(a.V.Content) < len(templateMark) || a.V.Content[:len(templateMark)] != templateMark {
				t.Errorf("%s activity %q is not marked as template text", ref.topic.ID, a.V.Content)
			}
		}
	}
}

func TestInvalidPrograms(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*program.Program)
	}{
		{"no header", func(p *program.Program) { p.V.Header = nil }},
		{"no curriculum", func(p *program.Program) { p.V.Curriculum = nil }},
		{"no modules", func(p *program.Program) { p.V.Curriculum.V.Modules = nil }},
		{"no final attestation", func(p *program.Program) { p.V.FinalAttestation = nil }},
		{"module without topics", func(p *program.Program) { p.V.Curriculum.V.Modules[0].V.Topics = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := approvedProgram(t, 40, 0, "")
			tt.mutate(p)

			_, err := newService().Generate(context.Background(), p, nil)
			if !errors.Is(err, ErrInvalidProgram) {
				t.Errorf("err = %v, want ErrInvalidProgram", err)
			}
		})
	}
}

// countingGenerator wraps the template generator to observe how it is called.
type countingGenerator struct {
	Generator
	running, peak atomic.Int32
	delay         time.Duration
	fail          error
}

func (g *countingGenerator) Topic(ctx context.Context, in TopicInput) (TopicContent, error) {
	now := g.running.Add(1)
	defer g.running.Add(-1)
	for {
		peak := g.peak.Load()
		if now <= peak || g.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	time.Sleep(g.delay)
	if g.fail != nil {
		return TopicContent{}, g.fail
	}
	return g.Generator.Topic(ctx, in)
}

func TestParallelismIsBoundedAndDoesNotChangeTheResult(t *testing.T) {
	run := func(maxParallel int) (string, int32) {
		gen := &countingGenerator{Generator: NewTemplateGenerator(), delay: 5 * time.Millisecond}
		svc := NewService(gen, validate.NewService(), Config{MaxParallel: maxParallel, SessionHours: 4, SessionsPerWeek: 2})
		p := approvedProgram(t, 144, 0, "")
		if _, err := svc.Generate(context.Background(), p, nil); err != nil {
			t.Fatal(err)
		}
		return toJSON(t, p), gen.peak.Load()
	}

	serial, serialPeak := run(1)
	parallel, parallelPeak := run(8)

	if serialPeak != 1 {
		t.Errorf("MaxParallel 1: peak concurrency = %d, want 1", serialPeak)
	}
	if parallelPeak < 2 || parallelPeak > 8 {
		t.Errorf("MaxParallel 8: peak concurrency = %d, want between 2 and 8", parallelPeak)
	}
	if serial != parallel {
		t.Error("the program depends on how many topics ran at once")
	}
}

func TestGeneratorFailuresAreClassified(t *testing.T) {
	tests := []struct {
		name string
		fail error
		want error
	}{
		{"failure", errors.New("model returned garbage"), ErrGeneratorFailed},
		{"timeout", context.DeadlineExceeded, ErrGeneratorTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &countingGenerator{Generator: NewTemplateGenerator(), fail: tt.fail}
			svc := NewService(gen, validate.NewService(), DefaultConfig())

			_, err := svc.Generate(context.Background(), approvedProgram(t, 40, 0, ""), nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestACancelledContextStopsGeneration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newService().Generate(ctx, approvedProgram(t, 40, 0, ""), nil)
	if err == nil {
		t.Error("a cancelled context must not produce content")
	}
}
