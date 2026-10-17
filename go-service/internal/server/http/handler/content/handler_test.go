package content

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
	contentservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/content"
	skeletonservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/skeleton"
)

var _ = skeleton.Draft{}

func realService() contentservice.Service {
	return contentservice.NewService(contentservice.NewTemplateGenerator(), validate.NewService(), contentservice.DefaultConfig())
}

func post(t *testing.T, svc contentservice.Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/content", strings.NewReader(body)))
	return rec
}

// approvedProgramJSON is a program as Java holds it after approving a skeleton.
func approvedProgramJSON(t *testing.T) string {
	t.Helper()
	const params = `{"programType": "DOOP_GENERAL", "title": "Анализ данных на Python", "domain": "Анализ данных",
		"totalHours": 72, "studyForm": "очная"}`

	var request program.Program
	if err := json.Unmarshal([]byte(`{"id": "p", "request": {"rawText": "x", "params": `+params+`}}`), &request); err != nil {
		t.Fatal(err)
	}
	variants, err := skeletonservice.NewService(skeletonservice.NewTemplateDrafter(), skeletonservice.NewTemplateRegenerator(), allocator.DefaultConfig()).Generate(context.Background(), &request)
	if err != nil {
		t.Fatal(err)
	}
	v := variants[0]

	var paramsMap map[string]any
	_ = json.Unmarshal([]byte(params), &paramsMap)
	data, err := json.Marshal(map[string]any{
		"id": "p", "status": "skeleton_approved", "programType": "DOOP_GENERAL",
		"request":      map[string]any{"rawText": "x", "params": paramsMap},
		"header":       map[string]any{"title": "Анализ данных на Python", "itDirection": "Анализ данных", "totalHours": 72, "studyForm": "очная"},
		"goal":         map[string]any{"text": v.Goal},
		"competencies": v.Competencies,
		"curriculum":   map[string]any{"modules": v.Modules, "totals": v.Totals},
		"finalAttestation": map[string]any{
			"form": v.FinalAttestation.Form, "description": "x",
			"outcomeIds": v.FinalAttestation.OutcomeIDs, "hours": v.FinalAttestation.Hours,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type contentBody struct {
	SchemaVersion string `json:"schemaVersion"`
	Program       struct {
		Status           string           `json:"status"`
		Lectures         []map[string]any `json:"lectures"`
		Sources          []any            `json:"sources"`
		NormativeBasis   []any            `json:"normativeBasis"`
		TemplateSections []map[string]any `json:"templateSections"`
		Calendar         struct {
			Sessions []map[string]any `json:"sessions"`
		} `json:"calendar"`
		Assessment struct {
			Tests        []map[string]any `json:"tests"`
			SeminarTasks []map[string]any `json:"seminarTasks"`
		} `json:"assessment"`
	} `json:"program"`
	Issues []map[string]any `json:"issues"`
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) contentBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %.600s", rec.Code, rec.Body)
	}
	var body contentBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestGenerateWholeProgram(t *testing.T) {
	body := decode(t, post(t, realService(), `{"schemaVersion": "1.1", "program": `+approvedProgramJSON(t)+`, "nodeIds": null}`))

	if body.SchemaVersion != "1.1" || body.Program.Status != "content_ready" {
		t.Errorf("schemaVersion = %q, status = %q", body.SchemaVersion, body.Program.Status)
	}
	if len(body.Program.Lectures) == 0 || len(body.Program.Assessment.Tests) == 0 ||
		len(body.Program.Assessment.SeminarTasks) == 0 || len(body.Program.Calendar.Sessions) == 0 {
		t.Errorf("content blocks are missing: %+v", body.Program)
	}
	if body.Program.Sources == nil || body.Program.NormativeBasis == nil {
		t.Error("sources and normativeBasis must be present (as empty lists)")
	}
	if len(body.Program.TemplateSections) == 0 {
		t.Error("templateSections are missing")
	}

	// The two sections no template can fill are reported, and nothing else.
	if len(body.Issues) != 2 {
		t.Fatalf("issues = %v, want the two SECTION_MISSING", body.Issues)
	}
	for _, issue := range body.Issues {
		if issue["ruleId"] != "SECTION_MISSING" || issue["severity"] != "error" || issue["message"] == "" {
			t.Errorf("issue = %v", issue)
		}
	}
}

func TestGenerateSomeNodes(t *testing.T) {
	body := decode(t, post(t, realService(), `{"schemaVersion": "1.1", "program": `+approvedProgramJSON(t)+`, "nodeIds": ["M1"]}`))

	if len(body.Program.Lectures) == 0 {
		t.Error("the targeted module got no content")
	}
	if body.Issues == nil {
		t.Error("issues must be an array, never null")
	}
}

func TestIssuesAreNeverNull(t *testing.T) {
	body := decode(t, post(t, stubService{}, `{"schemaVersion": "1.1", "program": {"id": "p"}}`))
	if body.Issues == nil {
		t.Error("issues = null, want []")
	}
}

type stubService struct{ err error }

func (s stubService) Generate(context.Context, *program.Program, []string) ([]validate.Check, error) {
	return nil, s.err
}

func TestGenerateErrors(t *testing.T) {
	program := approvedProgramJSON(t)
	tests := []struct {
		name   string
		svc    contentservice.Service
		body   string
		status int
		code   string
	}{
		{"invalid json", realService(), `{not json`, 400, "bad_request"},
		{"wrong schemaVersion", realService(), `{"schemaVersion": "0.9", "program": ` + program + `}`, 400, "bad_request"},
		{"missing program", realService(), `{"schemaVersion": "1.1"}`, 400, "bad_request"},
		{"program without a plan", realService(), `{"schemaVersion": "1.1", "program": {"id": "p", "header": {"totalHours": 16}}}`, 400, "bad_request"},
		{"unknown node", realService(), `{"schemaVersion": "1.1", "program": ` + program + `, "nodeIds": ["M99"]}`, 400, "bad_request"},
		{"generator failed", stubService{contentservice.ErrGeneratorFailed}, `{"schemaVersion": "1.1", "program": {"id": "p"}}`, 502, "llm_error"},
		{"generator timed out", stubService{contentservice.ErrGeneratorTimeout}, `{"schemaVersion": "1.1", "program": {"id": "p"}}`, 504, "llm_timeout"},
		{"unexpected failure", stubService{errors.New("boom")}, `{"schemaVersion": "1.1", "program": {"id": "p"}}`, 500, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, tt.svc, tt.body)

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d, body = %.300s", rec.Code, tt.status, rec.Body)
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
