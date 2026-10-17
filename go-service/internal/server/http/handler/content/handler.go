package content

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/contract"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/generr"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/dto"
	httperror "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/error"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/response"
	contentservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/content"
)

const maxBodyBytes = 32 << 20

const defaultContentDeadline = 870 * time.Second

type Handler struct {
	contentService contentservice.Service
	deadline       time.Duration
}

func NewHandler(contentService contentservice.Service, deadline ...time.Duration) *Handler {
	d := defaultContentDeadline
	if len(deadline) > 0 && deadline[0] > 0 {
		d = deadline[0]
	}
	return &Handler{contentService: contentService, deadline: d}
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

	ctx, cancel := context.WithTimeout(r.Context(), h.deadline)
	defer cancel()

	checks, err := h.contentService.Generate(ctx, req.Program, req.NodeIDs)
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
	case errors.Is(err, generr.ErrTimeout), errors.Is(err, contentservice.ErrGeneratorTimeout), errors.Is(err, context.DeadlineExceeded):
		return httperror.Wrap(httperror.LLMTimeout, err.Error(), err)
	case errors.Is(err, generr.ErrSchemaInvalid):
		return httperror.Wrap(httperror.SchemaInvalid, err.Error(), err)
	case errors.Is(err, generr.ErrUpstream), errors.Is(err, contentservice.ErrGeneratorFailed):
		return httperror.Wrap(httperror.LLMError, err.Error(), err)
	default:
		return err
	}
}
