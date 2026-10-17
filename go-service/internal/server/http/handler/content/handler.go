package content

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/contract"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/dto"
	httperror "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/error"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/response"
	contentservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/content"
)

const maxBodyBytes = 32 << 20

type Handler struct {
	contentService contentservice.Service
}

func NewHandler(contentService contentservice.Service) *Handler {
	return &Handler{contentService: contentService}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/content", h.Generate)
}

type generateRequest struct {
	SchemaVersion string           `json:"schemaVersion"`
	Program       *program.Program `json:"program"`
	// NodeIDs null (or empty) means the whole program; otherwise only those
	// nodes and their dependents are regenerated, locked ones skipped.
	NodeIDs []string `json:"nodeIds"`
}

type generateResponse struct {
	SchemaVersion string          `json:"schemaVersion"`
	Program       program.Program `json:"program"`
	Issues        []dto.Issue     `json:"issues"`
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

	checks, err := h.contentService.Generate(r.Context(), req.Program, req.NodeIDs)
	if err != nil {
		response.Error(w, r, toHTTPError(err))
		return
	}

	response.JSON(w, http.StatusOK, generateResponse{
		SchemaVersion: contract.SchemaVersion,
		Program:       *req.Program,
		Issues:        dto.IssuesFrom(checks),
	})
}

func toHTTPError(err error) error {
	switch {
	case errors.Is(err, contentservice.ErrInvalidProgram), errors.Is(err, contentservice.ErrUnknownNode):
		return httperror.Wrap(httperror.BadRequest, err.Error(), err)
	case errors.Is(err, contentservice.ErrGeneratorTimeout):
		return httperror.Wrap(httperror.LLMTimeout, err.Error(), err)
	case errors.Is(err, contentservice.ErrGeneratorFailed):
		return httperror.Wrap(httperror.LLMError, err.Error(), err)
	default:
		return err
	}
}
