package skeleton

import (
	"encoding/json"
	"net/http"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/contract"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	httperror "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/error"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/response"
	skeletonservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
)

type regenerateRequest struct {
	SchemaVersion string           `json:"schemaVersion"`
	Program       *program.Program `json:"program"`
	Comment       string           `json:"comment"`
	NodeIDs       []string         `json:"nodeIds"`
	// RespectLocks defaults to true when absent: overwriting a person's work
	// must be asked for.
	RespectLocks *bool `json:"respectLocks"`
}

// regenerateResponse carries exactly one of Program and Variants; the other is null.
type regenerateResponse struct {
	SchemaVersion string                    `json:"schemaVersion"`
	Program       *program.Program          `json:"program"`
	ChangedPaths  []string                  `json:"changedPaths"`
	Variants      []skeletonservice.Variant `json:"variants"`
}

func (h *Handler) Regenerate(w http.ResponseWriter, r *http.Request) {
	var req regenerateRequest
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

	respectLocks := req.RespectLocks == nil || *req.RespectLocks
	result, err := h.skeletonService.Regenerate(r.Context(), req.Program, skeletonservice.RegenerateParams{
		Comment:      req.Comment,
		NodeIDs:      req.NodeIDs,
		RespectLocks: respectLocks,
	})
	if err != nil {
		response.Error(w, r, toHTTPError(err))
		return
	}

	response.JSON(w, http.StatusOK, regenerateResponse{
		SchemaVersion: contract.SchemaVersion,
		Program:       result.Program,
		ChangedPaths:  result.ChangedPaths,
		Variants:      result.Variants,
	})
}
