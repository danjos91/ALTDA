package validate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	validateservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
)

const examplePath = "../../../../../../schema/example_program.json"

func post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(validateservice.NewService()).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/validate", strings.NewReader(body)))
	return rec
}

func example(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("schema example not found: %v", err)
	}
	return string(data)
}

type validateBody struct {
	SchemaVersion    string `json:"schemaVersion"`
	ValidationReport struct {
		GeneratedAt string `json:"generatedAt"`
		Summary     struct {
			Errors, Warnings, Passed int
		} `json:"summary"`
		Checks []map[string]any `json:"checks"`
	} `json:"validationReport"`
	Issues []map[string]any `json:"issues"`
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) validateBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var body validateBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestValidProgramHasOnlyItsKnownWarning(t *testing.T) {
	body := decode(t, post(t, `{"schemaVersion": "1.1", "program": `+example(t)+`}`))

	if body.SchemaVersion != "1.1" {
		t.Errorf("schemaVersion = %q", body.SchemaVersion)
	}
	// The example has one unverified source, and nothing else to report.
	if len(body.Issues) != 1 || body.Issues[0]["ruleId"] != "SOURCE_UNVERIFIED" || body.Issues[0]["severity"] != "warning" {
		t.Errorf("issues = %v, want only the SOURCE_UNVERIFIED warning", body.Issues)
	}
	if body.ValidationReport.GeneratedAt == "" || len(body.ValidationReport.Checks) == 0 {
		t.Errorf("report is incomplete: %+v", body.ValidationReport)
	}
	if body.ValidationReport.Summary.Errors != 0 || body.ValidationReport.Summary.Passed == 0 {
		t.Errorf("summary = %+v", body.ValidationReport.Summary)
	}
}

func TestBrokenProgramListsIssues(t *testing.T) {
	// The example asks for 16 h in header.totalHours; ask for 20 instead.
	var prog map[string]any
	if err := json.Unmarshal([]byte(example(t)), &prog); err != nil {
		t.Fatal(err)
	}
	prog["header"].(map[string]any)["totalHours"] = 20
	broken, err := json.Marshal(prog)
	if err != nil {
		t.Fatal(err)
	}

	body := decode(t, post(t, `{"schemaVersion": "1.1", "program": `+string(broken)+`}`))

	if len(body.Issues) != 2 { // the broken hours, plus the example's unverified source
		t.Fatalf("issues = %v, want two", body.Issues)
	}
	issue := body.Issues[0]
	if issue["ruleId"] != "HOURS_EQ_REQUESTED" || issue["severity"] != "error" ||
		issue["path"] != "/header/totalHours" || issue["message"] == "" {
		t.Errorf("issue = %v", issue)
	}
	if body.ValidationReport.Summary.Errors != 1 || body.ValidationReport.Summary.Warnings != 1 {
		t.Errorf("summary = %+v, want 1 error and 1 warning", body.ValidationReport.Summary)
	}
}

func TestBeforeASkeletonThereIsNothingToCheck(t *testing.T) {
	body := decode(t, post(t, `{"schemaVersion": "1.1", "program": {"id": "p", "status": "intake", "request": {"rawText": "x"}}}`))

	if body.Issues == nil || len(body.Issues) != 0 {
		t.Errorf("issues = %v, want an empty array", body.Issues)
	}
	if body.ValidationReport.Checks == nil {
		t.Error("checks must be an empty array, not null")
	}
}

func TestValidateBadRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", `{not json`},
		{"wrong schemaVersion", `{"schemaVersion": "0.9", "program": {}}`},
		{"missing program", `{"schemaVersion": "1.1"}`},
		{"null program", `{"schemaVersion": "1.1", "program": null}`},
		{"unknown status", `{"schemaVersion": "1.1", "program": {"id": "p", "status": "weird"}}`},
		{"no curriculum after skeleton", `{"schemaVersion": "1.1", "program": {"status": "skeleton_proposed", "header": {"totalHours": 16}}}`},
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
