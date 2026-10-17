package skeleton

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/contract"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	httperror "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/error"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/response"
	allocateservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocate"
	skeletonservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
)

const maxBodyBytes = 32 << 20

type Handler struct {
	skeletonService skeletonservice.Service
}

func NewHandler(skeletonService skeletonservice.Service) *Handler {
	return &Handler{skeletonService: skeletonService}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/skeleton/generate", h.Generate)
	mux.HandleFunc("POST /v1/skeleton/regenerate", h.Regenerate)
}

type generateRequest struct {
	SchemaVersion string           `json:"schemaVersion"`
	Program       *program.Program `json:"program"`
	// SelectedVariantKey is part of the contract but does not change what is generated.
	SelectedVariantKey *string `json:"selectedVariantKey"`
}

type generateResponse struct {
	SchemaVersion string                    `json:"schemaVersion"`
	Variants      []skeletonservice.Variant `json:"variants"`
}

func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	var req generateRequest
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

	variants, err := h.skeletonService.Generate(r.Context(), req.Program)
	if err != nil {
		response.Error(w, r, toHTTPError(err))
		return
	}

	response.JSON(w, http.StatusOK, generateResponse{
		SchemaVersion: contract.SchemaVersion,
		Variants:      variants,
	})
}

func toHTTPError(err error) error {
	switch {
	case errors.Is(err, skeletonservice.ErrInvalidProgram),
		errors.Is(err, skeletonservice.ErrUnknownNode),
		errors.Is(err, allocateservice.ErrNotEnoughHours),
		errors.Is(err, allocateservice.ErrNothingToBalance):
		return httperror.Wrap(httperror.BadRequest, err.Error(), err)
	case errors.Is(err, skeletonservice.ErrNodeLocked):
		return httperror.Wrap(httperror.NodeLocked, err.Error(), err)
	case errors.Is(err, skeletonservice.ErrNoValidVariant):
		return httperror.Wrap(httperror.LLMError, err.Error(), err)
	default:
		return err
	}
}
