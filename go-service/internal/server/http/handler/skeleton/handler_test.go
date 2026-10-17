package skeleton

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
	skeletonservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
)

func post(t *testing.T, svc skeletonservice.Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	return postTo(t, svc, "/v1/skeleton/generate", body)
}

func postTo(t *testing.T, svc skeletonservice.Service, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

func realService() skeletonservice.Service {
	return skeletonservice.NewService(skeletonservice.NewTemplateDrafter(), skeletonservice.NewTemplateRegenerator(), allocator.DefaultConfig())
}

const program72 = `{"id": "p", "status": "clarifying", "request": {"rawText": "x", "params": {
	"programType": "DPP_PK", "title": "Анализ данных на Python", "domain": "Анализ данных",
	"totalHours": 72, "studyForm": "очная"}}}`

func TestGenerateOK(t *testing.T) {
	rec := post(t, realService(), `{"schemaVersion": "1.1", "program": `+program72+`, "selectedVariantKey": null}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	var body struct {
		SchemaVersion string `json:"schemaVersion"`
		Variants      []struct {
			Key     string
			Totals  map[string]float64
			Modules []struct {
				ID     string
				Hours  map[string]float64
				Topics []struct {
					ID    string
					Hours map[string]float64
				}
			}
		} `json:"variants"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.SchemaVersion != "1.1" || len(body.Variants) != 3 {
		t.Fatalf("schemaVersion = %q, variants = %d", body.SchemaVersion, len(body.Variants))
	}
	for _, v := range body.Variants {
		if v.Totals["total"] != 72 {
			t.Errorf("variant %s: totals.total = %v, want 72", v.Key, v.Totals["total"])
		}
		if len(v.Modules) == 0 || v.Modules[0].Hours["total"] == 0 || v.Modules[0].Topics[0].Hours["total"] == 0 {
			t.Errorf("variant %s: hours were not allocated", v.Key)
		}
	}
}

type failingService struct{ err error }

func (f failingService) Generate(context.Context, *program.Program) ([]skeletonservice.Variant, error) {
	return nil, f.err
}

func (f failingService) Regenerate(context.Context, *program.Program, skeletonservice.RegenerateParams) (*skeletonservice.RegenerateResult, error) {
	return nil, f.err
}

func TestGenerateErrors(t *testing.T) {
	tests := []struct {
		name   string
		svc    skeletonservice.Service
		body   string
		status int
		code   string
	}{
		{"invalid json", realService(), `{not json`, 400, "bad_request"},
		{"wrong schemaVersion", realService(), `{"schemaVersion": "0.9", "program": ` + program72 + `}`, 400, "bad_request"},
		{"missing program", realService(), `{"schemaVersion": "1.1"}`, 400, "bad_request"},
		{"no hours in the request", realService(), `{"schemaVersion": "1.1", "program": {"id": "p", "request": {"rawText": "x", "params": {"title": "Курс"}}}}`, 400, "bad_request"},
		{"nothing usable produced", failingService{skeletonservice.ErrNoValidVariant}, `{"schemaVersion": "1.1", "program": ` + program72 + `}`, 502, "llm_error"},
		{"unexpected failure", failingService{errors.New("boom")}, `{"schemaVersion": "1.1", "program": ` + program72 + `}`, 500, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, tt.svc, tt.body)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tt.status, rec.Body)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] != tt.code || body["message"] == "" {
				t.Errorf("body = %v", body)
			}
		})
	}
}
