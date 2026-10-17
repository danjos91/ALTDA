package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	httperror "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/error"
)

func TestErrorStatusAndBody(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"bad request", httperror.New(httperror.BadRequest, "x"), 400, "bad_request"},
		{"node locked", httperror.New(httperror.NodeLocked, "x"), 409, "node_locked"},
		{"schema invalid", httperror.New(httperror.SchemaInvalid, "x"), 422, "schema_invalid"},
		{"llm error", httperror.New(httperror.LLMError, "x"), 502, "llm_error"},
		{"llm timeout", httperror.New(httperror.LLMTimeout, "x"), 504, "llm_timeout"},
		{"not implemented", httperror.New(httperror.NotImplemented, "x"), 501, "not_implemented"},
		{"unknown error", errors.New("boom"), 500, "internal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Error(rec, httptest.NewRequest(http.MethodPost, "/", nil), tt.err)

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
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

func TestInternalErrorDoesNotLeakCause(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, httptest.NewRequest(http.MethodPost, "/", nil), errors.New("db password is hunter2"))

	if got := rec.Body.String(); contains(got, "hunter2") {
		t.Errorf("cause leaked to client: %s", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
