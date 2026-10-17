package content

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
	skeletonservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
)

// approvedProgram builds the program Java holds once a skeleton variant is
// approved: the variant's structure and hours merged into the request.
func approvedProgram(t *testing.T, hours float64, variantIndex int, extraParams string) *program.Program {
	t.Helper()

	params := fmt.Sprintf(`{"programType": "DOOP_GENERAL", "title": "Анализ данных на Python", "domain": "Анализ данных",
		"totalHours": %v, "audience": "Аналитики", "entryLevel": "Базовый", "studyForm": "очная"%s}`, hours, extraParams)

	var request program.Program
	if err := json.Unmarshal([]byte(`{"id": "p", "request": {"rawText": "x", "params": `+params+`}}`), &request); err != nil {
		t.Fatal(err)
	}
	skeleton := skeletonservice.NewService(skeletonservice.NewTemplateDrafter(), skeletonservice.NewTemplateRegenerator(), allocator.DefaultConfig())
	variants, err := skeleton.Generate(context.Background(), &request)
	if err != nil {
		t.Fatal(err)
	}
	v := variants[variantIndex]

	var paramsMap map[string]any
	_ = json.Unmarshal([]byte(params), &paramsMap)

	doc := map[string]any{
		"schemaVersion": "1.1", "id": "p", "status": "skeleton_approved", "programType": "DOOP_GENERAL",
		"request": map[string]any{"rawText": "x", "params": paramsMap},
		"header": map[string]any{
			"title": "Анализ данных на Python", "itDirection": "Анализ данных",
			"totalHours": hours, "studyForm": paramsMap["studyForm"],
		},
		"goal":         map[string]any{"text": v.Goal, "tasks": []any{}},
		"competencies": v.Competencies,
		"curriculum":   map[string]any{"modules": v.Modules, "totals": v.Totals},
		"finalAttestation": map[string]any{
			"form": v.FinalAttestation.Form, "description": v.FinalAttestation.Description,
			"outcomeIds": v.FinalAttestation.OutcomeIDs, "hours": v.FinalAttestation.Hours,
		},
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

func newService() Service {
	return NewService(NewTemplateGenerator(), validate.NewService(), DefaultConfig())
}

func generate(t *testing.T, p *program.Program, nodeIDs ...string) []validate.Check {
	t.Helper()
	issues, err := newService().Generate(context.Background(), p, nodeIDs)
	if err != nil {
		t.Fatal(err)
	}
	return issues
}

func toJSON(t *testing.T, p *program.Program) string {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func topicByID(p *program.Program, id string) *program.Topic {
	for mi := range p.V.Curriculum.V.Modules {
		topics := p.V.Curriculum.V.Modules[mi].V.Topics
		for ti := range topics {
			if topics[ti].V.ID == id {
				return &topics[ti].V
			}
		}
	}
	return nil
}

func ruleIDs(issues []validate.Check) []string {
	ids := make([]string, len(issues))
	for i, c := range issues {
		ids[i] = c.RuleID
	}
	return ids
}

// onlyMissingSections reports whether the only issues are the two sections the
// template cannot fill: sources and the normative basis come from a verified
// catalog, and nothing is invented.
func onlyMissingSections(issues []validate.Check) bool {
	for _, c := range issues {
		if c.RuleID != "SECTION_MISSING" {
			return false
		}
	}
	return len(issues) == 2
}

func jsonUnmarshal(data string, v any) error { return json.Unmarshal([]byte(data), v) }

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// generate2 validates without regenerating: the issues of the program as it is.
func generate2(t *testing.T, p *program.Program) []validate.Check {
	t.Helper()
	report, err := validate.NewService().Validate(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return report.Issues()
}
