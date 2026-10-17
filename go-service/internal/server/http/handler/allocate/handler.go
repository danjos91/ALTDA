package allocate

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/contract"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	httperror "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/error"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/response"
	allocateservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocate"
)

const maxBodyBytes = 32 << 20 // a full program with content is a few MiB

type Handler struct {
	allocateService allocateservice.Service
}

func NewHandler(allocateService allocateservice.Service) *Handler {
	return &Handler{allocateService: allocateService}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/allocate", h.Allocate)
}

type allocateRequest struct {
	SchemaVersion string           `json:"schemaVersion"`
	Program       *program.Program `json:"program"`
}

type allocateResponse struct {
	SchemaVersion string          `json:"schemaVersion"`
	Program       program.Program `json:"program"`
}

func (h *Handler) Allocate(w http.ResponseWriter, r *http.Request) {
	var req allocateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		response.Error(w, r, httperror.Wrap(httperror.BadRequest, "request body is not valid JSON", err))
		return
	}
	if req.SchemaVersion != contract.SchemaVersion {
		response.Error(w, r, httperror.New(httperror.BadRequest, "schemaVersion must be "+contract.SchemaVersion))
		return
	}
	if req.Program == nil {
		response.Error(w, r, httperror.New(httperror.BadRequest, "program is required"))
		return
	}

	if err := h.allocateService.Allocate(r.Context(), req.Program); err != nil {
		response.Error(w, r, toHTTPError(err))
		return
	}

	response.JSON(w, http.StatusOK, allocateResponse{
		SchemaVersion: contract.SchemaVersion,
		Program:       *req.Program,
	})
}

// toHTTPError turns the service's errors into contract errors. The contract
// has no dedicated code for "these hours cannot be balanced", so those are
// reported as bad_request with the reason in the message.
func toHTTPError(err error) error {
	switch {
	case errors.Is(err, allocateservice.ErrInvalidProgram),
		errors.Is(err, allocateservice.ErrNotEnoughHours),
		errors.Is(err, allocateservice.ErrNothingToBalance):
		return httperror.Wrap(httperror.BadRequest, err.Error(), err)
	default:
		return err
	}
}
