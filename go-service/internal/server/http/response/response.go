package response

import (
	"encoding/json"
	"log/slog"
	"net/http"

	httperror "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/error"
)

type errorBody struct {
	Error   httperror.Code `json:"error"`
	Message string         `json:"message"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes err as {error, message}. Anything that is not an
// *httperror.Error is logged and answered as a generic 500.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	appErr := httperror.From(err)
	if appErr.Code == httperror.Internal {
		slog.ErrorContext(r.Context(), "internal error", slog.Any("error", err))
	}
	JSON(w, statusOf(appErr.Code), errorBody{Error: appErr.Code, Message: appErr.Message})
}

func statusOf(code httperror.Code) int {
	switch code {
	case httperror.BadRequest:
		return http.StatusBadRequest
	case httperror.NodeLocked:
		return http.StatusConflict
	case httperror.SchemaInvalid:
		return http.StatusUnprocessableEntity
	case httperror.LLMError:
		return http.StatusBadGateway
	case httperror.LLMTimeout:
		return http.StatusGatewayTimeout
	case httperror.NotImplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
