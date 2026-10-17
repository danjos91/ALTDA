package parse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	parseservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/parse"
)

func post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(parseservice.NewService()).Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/parse", strings.NewReader(body)))
	return rec
}

func TestParseOK(t *testing.T) {
	rec := post(t, `{"schemaVersion":"1.1","rawText":"ДПП ПК Анализ данных на Python, 72 ч","attachments":[],"clarifications":[]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["schemaVersion"] != "1.1" {
		t.Errorf("schemaVersion = %v", body["schemaVersion"])
	}
	for _, key := range []string{"params", "missing", "clarifications", "gapsReport"} {
		if body[key] == nil {
			t.Errorf("%s must be present and not null", key)
		}
	}
	params := body["params"].(map[string]any)
	if params["totalHours"] != 72.0 || params["programType"] != "DPP_PK" {
		t.Errorf("params = %v", params)
	}
}

func TestParseWithAnswersClearsMissing(t *testing.T) {
	rec := post(t, `{"schemaVersion":"1.1","rawText":"ДПП ПК Анализ данных, 72 ч","clarifications":[
		{"param":"domain","question":"","answer":"Анализ данных"},
		{"param":"audience","question":"","answer":"Аналитики"},
		{"param":"entryLevel","question":"","answer":"Базовый"}]}`)

	var body struct {
		Missing []string `json:"missing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Missing) != 0 {
		t.Errorf("missing = %v, want none", body.Missing)
	}
}

func TestParseBadRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", `{not json`},
		{"wrong schemaVersion", `{"schemaVersion":"0.9","rawText":"x"}`},
		{"missing schemaVersion", `{"rawText":"x"}`},
		{"empty rawText", `{"schemaVersion":"1.1","rawText":"   "}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
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
