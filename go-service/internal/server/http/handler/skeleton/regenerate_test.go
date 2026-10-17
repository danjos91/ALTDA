package skeleton

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	skeletonservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
)

func postRegenerate(t *testing.T, svc skeletonservice.Service, body string) (int, string) {
	t.Helper()
	rec := postTo(t, svc, "/v1/skeleton/regenerate", body)
	return rec.Code, rec.Body.String()
}

// approvedProgramJSON is the program Java holds after approving variant A.
func approvedProgramJSON(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	const params = `{"programType": "DOOP_GENERAL", "title": "Анализ данных на Python", "domain": "Анализ данных",
		"totalHours": 72, "studyForm": "очная"}`

	var request program.Program
	if err := json.Unmarshal([]byte(`{"id": "p", "request": {"rawText": "x", "params": `+params+`}}`), &request); err != nil {
		t.Fatal(err)
	}
	variants, err := realService().Generate(context.Background(), &request)
	if err != nil {
		t.Fatal(err)
	}
	v := variants[0]

	var paramsMap map[string]any
	_ = json.Unmarshal([]byte(params), &paramsMap)
	doc := map[string]any{
		"id": "p", "status": "skeleton_approved", "programType": "DOOP_GENERAL",
		"request":      map[string]any{"rawText": "x", "params": paramsMap},
		"header":       map[string]any{"title": "Анализ данных на Python", "itDirection": "Анализ данных", "totalHours": 72, "studyForm": "очная"},
		"competencies": v.Competencies,
		"curriculum":   map[string]any{"modules": v.Modules, "totals": v.Totals},
		"finalAttestation": map[string]any{"form": v.FinalAttestation.Form, "description": "x",
			"outcomeIds": v.FinalAttestation.OutcomeIDs, "hours": v.FinalAttestation.Hours},
	}
	if mutate != nil {
		// Round trip through JSON so the mutation edits plain maps.
		data, _ := json.Marshal(doc)
		doc = map[string]any{}
		_ = json.Unmarshal(data, &doc)
		mutate(doc)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func lockFirstTopicOfModule2(doc map[string]any) {
	modules := doc["curriculum"].(map[string]any)["modules"].([]any)
	topic := modules[1].(map[string]any)["topics"].([]any)[0].(map[string]any)
	topic["_meta"] = map[string]any{"locked": true}
}

type regenerateBody struct {
	SchemaVersion string           `json:"schemaVersion"`
	Program       map[string]any   `json:"program"`
	ChangedPaths  []string         `json:"changedPaths"`
	Variants      []map[string]any `json:"variants"`
}

func decodeRegenerate(t *testing.T, status int, raw string) regenerateBody {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %.400s", status, raw)
	}
	var body regenerateBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestRegenerateBeforeThePlanReturnsVariants(t *testing.T) {
	status, raw := postRegenerate(t, realService(),
		`{"schemaVersion": "1.1", "comment": "Хочу больше практики", "nodeIds": null, "respectLocks": true, "program": `+program72+`}`)
	body := decodeRegenerate(t, status, raw)

	if body.Program != nil {
		t.Error("program must be null in variants mode")
	}
	if len(body.Variants) != 3 {
		t.Errorf("variants = %d, want 3", len(body.Variants))
	}
	if body.ChangedPaths == nil || len(body.ChangedPaths) != 0 {
		t.Errorf("changedPaths = %v, want []", body.ChangedPaths)
	}
	if !strings.Contains(raw, `"program":null`) {
		t.Errorf("the response must carry an explicit null program: %.200s", raw)
	}
}

func TestRegenerateAfterThePlanReturnsTheProgram(t *testing.T) {
	status, raw := postRegenerate(t, realService(),
		`{"schemaVersion": "1.1", "comment": "Упрости модуль 2", "nodeIds": ["M2"], "respectLocks": true, "program": `+approvedProgramJSON(t, nil)+`}`)
	body := decodeRegenerate(t, status, raw)

	if body.Program == nil {
		t.Fatal("program must be set once the plan exists")
	}
	if body.Variants != nil || !strings.Contains(raw, `"variants":null`) {
		t.Errorf("variants must be null in program mode: %.200s", raw)
	}
	if len(body.ChangedPaths) != 1 || body.ChangedPaths[0] != "/curriculum/modules/1" {
		t.Errorf("changedPaths = %v", body.ChangedPaths)
	}
	totals := body.Program["curriculum"].(map[string]any)["totals"].(map[string]any)
	if totals["total"] != 72.0 {
		t.Errorf("totals.total = %v, want 72", totals["total"])
	}
}

func TestRegenerateRefusesLockedNodes(t *testing.T) {
	locked := approvedProgramJSON(t, lockFirstTopicOfModule2)

	tests := []struct {
		name   string
		fields string
		status int
		code   string
	}{
		{"respectLocks true", `"nodeIds": ["M2"], "respectLocks": true`, 409, "node_locked"},
		{"respectLocks omitted means true", `"nodeIds": ["M2"]`, 409, "node_locked"},
		{"respectLocks false overrides", `"nodeIds": ["M2"], "respectLocks": false`, 200, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, raw := postRegenerate(t, realService(),
				`{"schemaVersion": "1.1", "comment": "", `+tt.fields+`, "program": `+locked+`}`)

			if status != tt.status {
				t.Fatalf("status = %d, want %d, body = %.300s", status, tt.status, raw)
			}
			if tt.code != "" {
				var body map[string]string
				if err := json.Unmarshal([]byte(raw), &body); err != nil {
					t.Fatal(err)
				}
				if body["error"] != tt.code || !strings.Contains(body["message"], "M2") {
					t.Errorf("body = %v", body)
				}
			}
		})
	}
}

func TestRegenerateErrors(t *testing.T) {
	program := approvedProgramJSON(t, nil)
	tests := []struct {
		name   string
		svc    skeletonservice.Service
		body   string
		status int
		code   string
	}{
		{"invalid json", realService(), `{not json`, 400, "bad_request"},
		{"wrong schemaVersion", realService(), `{"schemaVersion": "0.9", "program": ` + program + `}`, 400, "bad_request"},
		{"missing program", realService(), `{"schemaVersion": "1.1"}`, 400, "bad_request"},
		{"unknown node", realService(), `{"schemaVersion": "1.1", "nodeIds": ["M99"], "program": ` + program + `}`, 400, "bad_request"},
		{"no hours in the request", realService(), `{"schemaVersion": "1.1", "program": {"id": "p", "request": {"rawText": "x", "params": {"title": "Курс"}}}}`, 400, "bad_request"},
		{"nothing usable proposed", failingService{skeletonservice.ErrNoValidVariant}, `{"schemaVersion": "1.1", "program": {"id": "p"}}`, 502, "llm_error"},
		{"unexpected failure", failingService{errors.New("boom")}, `{"schemaVersion": "1.1", "program": {"id": "p"}}`, 500, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, raw := postRegenerate(t, tt.svc, tt.body)

			if status != tt.status {
				t.Fatalf("status = %d, want %d, body = %.300s", status, tt.status, raw)
			}
			var body map[string]string
			if err := json.Unmarshal([]byte(raw), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] != tt.code || body["message"] == "" {
				t.Errorf("body = %v", body)
			}
		})
	}
}
