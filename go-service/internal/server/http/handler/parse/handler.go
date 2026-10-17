package parse

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/contract"
	httperror "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/error"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/response"
	parseservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/parse"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type Handler struct {
	parseService parseservice.Service
}

func NewHandler(parseService parseservice.Service) *Handler {
	return &Handler{parseService: parseService}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/parse", h.Parse)
}

func (h *Handler) Parse(w http.ResponseWriter, r *http.Request) {
	var req parseRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		response.Error(w, r, httperror.Wrap(httperror.BadRequest, "request body is not valid JSON", err))
		return
	}
	if req.SchemaVersion != contract.SchemaVersion {
		response.Error(w, r, httperror.New(httperror.BadRequest, "schemaVersion must be "+contract.SchemaVersion))
		return
	}
	if strings.TrimSpace(req.RawText) == "" {
		response.Error(w, r, httperror.New(httperror.BadRequest, "rawText is required"))
		return
	}

	result, err := h.parseService.Parse(r.Context(), parseservice.Input{
		RawText:        req.RawText,
		Clarifications: req.Clarifications,
	})
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, parseResponse{
		SchemaVersion:  contract.SchemaVersion,
		Params:         result.Params,
		Missing:        result.Missing,
		Clarifications: result.Clarifications,
		GapsReport:     result.GapsReport,
	})
}
