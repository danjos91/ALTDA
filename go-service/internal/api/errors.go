package api

import (
	"encoding/json"
	"net/http"
)

// Error codes from docs/API.md.
const (
	CodeBadRequest     = "bad_request"
	CodeNodeLocked     = "node_locked"
	CodeLLMTimeout     = "llm_timeout"
	CodeLLMError       = "llm_error"
	CodeSchemaInvalid  = "schema_invalid"
)

// ErrorBody is the fixed Go → Java error shape.
type ErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorBody{Error: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func statusForCode(code string) int {
	switch code {
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeNodeLocked:
		return http.StatusConflict
	case CodeSchemaInvalid:
		return http.StatusUnprocessableEntity
	case CodeLLMTimeout:
		return http.StatusGatewayTimeout
	case CodeLLMError:
		return http.StatusBadGateway
	default:
		return http.StatusBadGateway
	}
}
