package allocate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocate"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
)

const sampleProgram = `{
	"id": "p1", "status": "skeleton_proposed", "request": {"rawText": "x"},
	"competencies": [{"id": "C1", "title": "Анализ"}],
	"header": {"title": "Курс", "totalHours": 16, "studyForm": "очная"},
	"curriculum": {"modules": [{"id": "M1", "title": "Основы", "weight": 1, "hours": {},
		"topics": [{"id": "T1.1", "title": "Тема", "weight": 1, "outcomeIds": ["C1.K1"], "hours": {}}]}],
		"totals": {}},
	"finalAttestation": {"form": "зачёт", "description": "x", "hours": {}}
}`

func post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(allocate.NewService(allocator.DefaultConfig())).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/allocate", strings.NewReader(body)))
	return rec
}

func TestAllocateOK(t *testing.T) {
	rec := post(t, `{"schemaVersion": "1.1", "program": `+sampleProgram+`}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	var body struct {
		SchemaVersion string         `json:"schemaVersion"`
		Program       map[string]any `json:"program"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SchemaVersion != "1.1" {
		t.Errorf("schemaVersion = %q", body.SchemaVersion)
	}

	totals := body.Program["curriculum"].(map[string]any)["totals"].(map[string]any)
	if totals["total"] != 16.0 {
		t.Errorf("totals.total = %v, want 16", totals["total"])
	}
	// Fields the allocator does not own come back intact.
	if body.Program["id"] != "p1" || body.Program["competencies"] == nil {
		t.Errorf("program lost fields: %v", body.Program)
	}
}

func TestAllocateBadRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", `{not json`},
		{"wrong schemaVersion", `{"schemaVersion": "0.9", "program": ` + sampleProgram + `}`},
		{"missing program", `{"schemaVersion": "1.1"}`},
		{"null program", `{"schemaVersion": "1.1", "program": null}`},
		{"program without curriculum", `{"schemaVersion": "1.1", "program": {"id": "p", "header": {"totalHours": 16}}}`},
		{"program without hours", `{"schemaVersion": "1.1", "program": {"id": "p", "header": {"totalHours": 0}, "curriculum": {"modules": [{"id": "M1"}]}, "finalAttestation": {}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] != "bad_request" || body["message"] == "" {
				t.Errorf("body = %v", body)
			}
		})
	}
}
