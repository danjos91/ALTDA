package skeleton

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

func programWith(t *testing.T, params string) *program.Program {
	t.Helper()
	var p program.Program
	in := `{"id": "p", "status": "clarifying", "request": {"rawText": "x", "params": ` + params + `}}`
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func paramsFor(hours float64, extra string) string {
	return fmt.Sprintf(`{"programType": "DPP_PK", "title": "Анализ данных на Python", "domain": "Анализ данных",
		"totalHours": %v, "audience": "Аналитики", "entryLevel": "Базовый", "studyForm": "очная"%s}`, hours, extra)
}

func newService() Service {
	return NewService(NewTemplateDrafter(), NewTemplateRegenerator(), allocator.DefaultConfig())
}

func generate(t *testing.T, params string) []Variant {
	t.Helper()
	variants, err := newService().Generate(context.Background(), programWith(t, params))
	if err != nil {
		t.Fatal(err)
	}
	return variants
}

func checkRow(t *testing.T, label string, h allocator.Hours) {
	t.Helper()
	if h.Contact != h.Lecture+h.Practice+h.Lab {
		t.Errorf("%s: contact %v != lecture+practice+lab", label, h.Contact)
	}
	if h.Total != h.Contact+h.SelfStudy+h.Assessment {
		t.Errorf("%s: total %v != contact+selfStudy+assessment", label, h.Total)
	}
}

// asProgram builds the program Java would build when this variant is
// approved, so the validator can check it.
func asProgram(t *testing.T, v Variant, hours float64) *program.Program {
	t.Helper()
	doc := map[string]any{
		"id": "p", "status": "skeleton_approved",
		"request":          map[string]any{"rawText": "x"},
		"header":           map[string]any{"totalHours": hours, "studyForm": "очная"},
		"competencies":     v.Competencies,
		"curriculum":       map[string]any{"modules": v.Modules, "totals": v.Totals},
		"finalAttestation": map[string]any{"form": v.FinalAttestation.Form, "hours": v.FinalAttestation.Hours},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var p program.Program
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func TestEveryVolumeGetsThreeExactVariants(t *testing.T) {
	validator := validate.NewService()

	for _, hours := range []float64{16, 17.5, 24, 32, 40, 72, 100, 144, 250, 500} {
		t.Run(fmt.Sprint(hours), func(t *testing.T) {
			variants := generate(t, paramsFor(hours, ""))

			if len(variants) != 3 {
				t.Fatalf("variants = %d, want 3", len(variants))
			}
			for _, v := range variants {
				if v.Totals.Total != hours {
					t.Errorf("variant %s: totals.total = %v, want exactly %v", v.Key, v.Totals.Total, hours)
				}
				checkRow(t, v.Key+" totals", v.Totals)
				checkRow(t, v.Key+" attestation", v.FinalAttestation.Hours)
				for _, m := range v.Modules {
					checkRow(t, m.ID, m.Hours)
					for _, topic := range m.Topics {
						checkRow(t, topic.ID, topic.Hours)
						if topic.Hours.Total < 1 {
							t.Errorf("%s has %v h", topic.ID, topic.Hours.Total)
						}
					}
				}

				// What Java assembles from this variant must pass the hours rules.
				report, err := validator.Validate(context.Background(), asProgram(t, v, hours))
				if err != nil {
					t.Fatal(err)
				}
				if issues := report.Issues(); len(issues) != 0 {
					t.Errorf("variant %s fails validation: %+v", v.Key, issues)
				}
			}
		})
	}
}

func TestVariantsDifferInStructure(t *testing.T) {
	variants := generate(t, paramsFor(72, ""))

	keys := map[string]bool{}
	titles := map[string]bool{}
	for _, v := range variants {
		keys[v.Key] = true
		titles[v.Modules[0].Title] = true
	}
	if len(keys) != 3 || len(titles) != 3 {
		t.Errorf("variants must have distinct keys and structure: keys %v, first modules %v", keys, titles)
	}
}

func TestVariantsAreHonestAboutBeingTemplates(t *testing.T) {
	for _, v := range generate(t, paramsFor(72, "")) {
		if v.Rationale == "" || v.Goal == "" || len(v.Cons) == 0 {
			t.Errorf("variant %s is missing rationale/goal/cons", v.Key)
		}
		for _, c := range v.Competencies {
			if len(c.ProfStandardLinks) != 0 {
				t.Errorf("variant %s invents professional standards: %+v", v.Key, c.ProfStandardLinks)
			}
		}
	}
}

func TestJSONNeverContainsNullLists(t *testing.T) {
	data, err := json.Marshal(generate(t, paramsFor(72, "")))
	if err != nil {
		t.Fatal(err)
	}

	var variants []map[string]any
	if err := json.Unmarshal(data, &variants); err != nil {
		t.Fatal(err)
	}
	for _, v := range variants {
		for _, key := range []string{"pros", "cons", "competencies", "modules"} {
			if v[key] == nil {
				t.Errorf("variant %v: %q is null", v["key"], key)
			}
		}
		for _, c := range v["competencies"].([]any) {
			comp := c.(map[string]any)
			for _, key := range []string{"profStandardLinks", "knowledge", "skills", "abilities"} {
				if comp[key] == nil {
					t.Errorf("competency %v: %q is null", comp["id"], key)
				}
			}
		}
		for _, m := range v["modules"].([]any) {
			for _, tp := range m.(map[string]any)["topics"].([]any) {
				topic := tp.(map[string]any)
				for _, key := range []string{"kinds", "outcomeIds", "prerequisites"} {
					if topic[key] == nil {
						t.Errorf("topic %v: %q is null", topic["id"], key)
					}
				}
			}
		}
	}
}

func TestRequestShapesTheVariants(t *testing.T) {
	t.Run("internship is all self-study", func(t *testing.T) {
		for _, v := range generate(t, paramsFor(72, `, "isInternship": true`)) {
			if v.Totals.Lecture+v.Totals.Practice+v.Totals.Lab != 0 {
				t.Errorf("variant %s has contact hours in an internship: %+v", v.Key, v.Totals)
			}
		}
	})

	t.Run("distance forms add self-study", func(t *testing.T) {
		params := `{"title": "Курс", "domain": "ИИ", "totalHours": 72, "studyForm": "заочная"}`
		for _, v := range generate(t, params) {
			if v.Totals.SelfStudy == 0 {
				t.Errorf("variant %s has no self-study hours for a distance form", v.Key)
			}
		}
	})

	t.Run("requested practice share is reached", func(t *testing.T) {
		for _, v := range generate(t, paramsFor(72, `, "minPracticeShare": 0.6`)) {
			if share := (v.Totals.Practice + v.Totals.Lab) / v.Totals.Total; share < 0.6 {
				t.Errorf("variant %s: practice share %.2f < 0.6", v.Key, share)
			}
		}
	})

	t.Run("requested attestation form is used", func(t *testing.T) {
		for _, v := range generate(t, paramsFor(72, `, "finalAttestationForm": "экзамен"`)) {
			if v.FinalAttestation.Form != "экзамен" {
				t.Errorf("variant %s: form = %q", v.Key, v.FinalAttestation.Form)
			}
		}
	})

	t.Run("the subject appears in the goal", func(t *testing.T) {
		for _, v := range generate(t, paramsFor(72, "")) {
			if !containsText(v.Goal, "Анализ данных") {
				t.Errorf("variant %s: goal %q does not mention the domain", v.Key, v.Goal)
			}
		}
	})
}

func containsText(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		prog string
	}{
		{"no request", `{"id": "p"}`},
		{"no params", `{"id": "p", "request": {"rawText": "x"}}`},
		{"no hours", `{"id": "p", "request": {"rawText": "x", "params": {"title": "Курс"}}}`},
		{"zero hours", `{"id": "p", "request": {"rawText": "x", "params": {"totalHours": 0}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p program.Program
			if err := json.Unmarshal([]byte(tt.prog), &p); err != nil {
				t.Fatal(err)
			}

			_, err := newService().Generate(context.Background(), &p)
			if !errors.Is(err, ErrInvalidProgram) {
				t.Errorf("err = %v, want ErrInvalidProgram", err)
			}
		})
	}
}

func TestHoursFallBackToTheHeader(t *testing.T) {
	var p program.Program
	in := `{"id": "p", "request": {"rawText": "x", "params": {"title": "Курс"}}, "header": {"totalHours": 40}}`
	if err := json.Unmarshal([]byte(in), &p); err != nil {
		t.Fatal(err)
	}

	variants, err := newService().Generate(context.Background(), &p)
	if err != nil {
		t.Fatal(err)
	}
	if variants[0].Totals.Total != 40 {
		t.Errorf("total = %v, want 40", variants[0].Totals.Total)
	}
}

// fakeDrafter lets a test hand the service any drafts.
type fakeDrafter struct{ drafts []skeleton.Draft }

func (f fakeDrafter) Draft(context.Context, Request) ([]skeleton.Draft, error) { return f.drafts, nil }

func goodDraft(key string) skeleton.Draft {
	draft := build(shapes[0], Request{Title: "Курс", StudyForm: "очная", TotalHours: 72})
	draft.Key = key
	return draft
}

func TestUnusableDraftsAreDropped(t *testing.T) {
	badWeights := goodDraft("B")
	badWeights.Modules[0].Weight = 0.9 // module weights no longer add up to 1

	unknownOutcome := goodDraft("C")
	unknownOutcome.Modules[0].Topics[0].OutcomeIDs = []string{"C9.K9"}

	tooManyTopics := goodDraft("D")
	for i := 0; i < 80; i++ {
		topic := tooManyTopics.Modules[0].Topics[0]
		topic.ID = fmt.Sprintf("T1.%d", 100+i)
		tooManyTopics.Modules[0].Topics = append(tooManyTopics.Modules[0].Topics, topic)
	}
	// Keep the weights valid so only the hours make it unusable.
	w := 1 / float64(len(tooManyTopics.Modules[0].Topics))
	for i := range tooManyTopics.Modules[0].Topics {
		tooManyTopics.Modules[0].Topics[i].Weight = w
		tooManyTopics.Modules[0].Topics[i].Prerequisites = []string{}
	}

	svc := NewService(fakeDrafter{[]skeleton.Draft{badWeights, goodDraft("A"), unknownOutcome, tooManyTopics}}, NewTemplateRegenerator(), allocator.DefaultConfig())
	variants, err := svc.Generate(context.Background(), programWith(t, paramsFor(72, "")))
	if err != nil {
		t.Fatal(err)
	}

	if len(variants) != 1 || variants[0].Key != "A" {
		keys := []string{}
		for _, v := range variants {
			keys = append(keys, v.Key)
		}
		t.Errorf("variants = %v, want only A", keys)
	}
}

func TestNoUsableDraftIsAnError(t *testing.T) {
	bad := goodDraft("A")
	bad.Modules[0].Weight = 0.9

	svc := NewService(fakeDrafter{[]skeleton.Draft{bad}}, NewTemplateRegenerator(), allocator.DefaultConfig())
	_, err := svc.Generate(context.Background(), programWith(t, paramsFor(72, "")))
	if !errors.Is(err, ErrNoValidVariant) {
		t.Errorf("err = %v, want ErrNoValidVariant", err)
	}
}

func TestAtMostThreeVariants(t *testing.T) {
	drafts := []skeleton.Draft{goodDraft("A"), goodDraft("B"), goodDraft("C"), goodDraft("D")}

	svc := NewService(fakeDrafter{drafts}, NewTemplateRegenerator(), allocator.DefaultConfig())
	variants, err := svc.Generate(context.Background(), programWith(t, paramsFor(72, "")))
	if err != nil {
		t.Fatal(err)
	}
	if len(variants) != 3 {
		t.Errorf("variants = %d, want 3 (the contract's maximum)", len(variants))
	}
}
