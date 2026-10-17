package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/danjos91/ALTDA/go-service/internal/allocator"
	"github.com/danjos91/ALTDA/go-service/internal/llm"
	"github.com/danjos91/ALTDA/go-service/internal/program"
	"github.com/danjos91/ALTDA/go-service/internal/validate"
)

// Handler serves the Go HTTP contract.
type Handler struct {
	LLM llm.Client
}

func (h *Handler) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", h.health)
	mux.HandleFunc("POST /v1/parse", h.parse)
	mux.HandleFunc("POST /v1/skeleton/generate", h.skeletonGenerate)
	mux.HandleFunc("POST /v1/skeleton/regenerate", h.skeletonRegenerate)
	mux.HandleFunc("POST /v1/allocate", h.allocate)
	mux.HandleFunc("POST /v1/content", h.content)
	mux.HandleFunc("POST /v1/validate", h.validate)
	return mux
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) parse(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeBody(w, r)
	if !ok {
		return
	}
	raw := program.GetString(body, "rawText", "")
	if raw == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "rawText is required")
		return
	}
	clar, _ := program.AsArray(body["clarifications"])
	if clar == nil {
		clar = program.Array{}
	}
	out, err := h.LLM.Parse(r.Context(), raw, clar)
	if err != nil {
		writeLLMError(w, err)
		return
	}
	out["schemaVersion"] = "1.1"
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) skeletonGenerate(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeBody(w, r)
	if !ok {
		return
	}
	prog, _ := program.AsObject(body["program"])
	if prog == nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "program is required")
		return
	}

	type result struct {
		variants program.Array
		err      error
	}
	ch := make(chan result, 1)
	go func() {
		// Generate up to 3 variants (implementation may parallelize internally).
		v, err := h.LLM.GenerateSkeleton(r.Context(), prog)
		ch <- result{v, err}
	}()
	select {
	case <-r.Context().Done():
		writeError(w, http.StatusGatewayTimeout, CodeLLMTimeout, "request cancelled")
		return
	case res := <-ch:
		if res.err != nil {
			writeLLMError(w, res.err)
			return
		}
		variants := res.variants
		if variants == nil {
			variants = program.Array{}
		}
		// Ensure hours allocated on each variant (deterministic allocator).
		total := program.TotalHoursFromProgram(prog)
		var wg sync.WaitGroup
		out := make(program.Array, len(variants))
		for i, v := range variants {
			wg.Add(1)
			go func(i int, v any) {
				defer wg.Done()
				vo := program.CloneObject(mustObj(v))
				allocator.AllocateVariant(vo, total)
				out[i] = vo
			}(i, v)
		}
		wg.Wait()
		if len(out) > 3 {
			out = out[:3]
		}
		writeJSON(w, http.StatusOK, program.Object{
			"schemaVersion": "1.1",
			"variants":      out,
		})
	}
}

func (h *Handler) skeletonRegenerate(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeBody(w, r)
	if !ok {
		return
	}
	prog, _ := program.AsObject(body["program"])
	if prog == nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "program is required")
		return
	}
	comment := program.GetString(body, "comment", "")
	respectLocks := true
	if v, ok := body["respectLocks"].(bool); ok {
		respectLocks = v
	}
	var nodeIDs []string
	if body["nodeIds"] != nil {
		if arr, ok := program.AsArray(body["nodeIds"]); ok {
			for _, n := range arr {
				if s, ok := n.(string); ok {
					nodeIDs = append(nodeIDs, s)
				}
			}
		}
	}
	if respectLocks {
		for _, id := range nodeIDs {
			if program.IsNodeLocked(prog, id) {
				writeError(w, http.StatusConflict, CodeNodeLocked, "node "+id+" is locked")
				return
			}
		}
	}

	updated, variants, changed, err := h.LLM.RegenerateSkeleton(r.Context(), prog, comment, nodeIDs, respectLocks)
	if err != nil {
		if llm.NodeLocked(err) {
			writeError(w, http.StatusConflict, CodeNodeLocked, err.Error())
			return
		}
		writeLLMError(w, err)
		return
	}

	// Exactly one of program / variants is non-null.
	resp := program.Object{
		"schemaVersion": "1.1",
		"changedPaths":  changed,
	}
	if updated != nil {
		allocator.AllocateProgram(updated, allocator.Options{
			LockedIDs: func() map[string]bool {
				if respectLocks {
					return program.CollectLockedIDs(prog)
				}
				return map[string]bool{}
			}(),
		})
		resp["program"] = updated
		resp["variants"] = nil
	} else {
		total := program.TotalHoursFromProgram(prog)
		for i, v := range variants {
			vo := mustObj(v)
			allocator.AllocateVariant(vo, total)
			variants[i] = vo
		}
		resp["program"] = nil
		resp["variants"] = variants
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) allocate(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeBody(w, r)
	if !ok {
		return
	}
	prog, _ := program.AsObject(body["program"])
	if prog == nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "program is required")
		return
	}
	copy := program.CloneObject(prog)
	allocator.AllocateProgram(copy, allocator.Options{
		LockedIDs: program.CollectLockedIDs(copy),
	})
	copy["schemaVersion"] = "1.1"
	writeJSON(w, http.StatusOK, program.Object{
		"schemaVersion": "1.1",
		"program":       copy,
	})
}

func (h *Handler) content(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeBody(w, r)
	if !ok {
		return
	}
	prog, _ := program.AsObject(body["program"])
	if prog == nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "program is required")
		return
	}
	var nodeIDs []string
	if body["nodeIds"] != nil {
		arr, ok := program.AsArray(body["nodeIds"])
		if !ok {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "nodeIds must be an array or null")
			return
		}
		for _, n := range arr {
			if s, ok := n.(string); ok {
				nodeIDs = append(nodeIDs, s)
			}
		}
	} else {
		nodeIDs = nil
	}

	out, issues, err := h.LLM.GenerateContent(r.Context(), prog, nodeIDs)
	if err != nil {
		writeLLMError(w, err)
		return
	}
	if issues == nil {
		issues = []validate.Issue{}
	}
	// Re-run validation internally; issues always an array.
	checks := validate.ValidateProgram(out)
	_, issues = validate.BuildReport(checks, 0)
	writeJSON(w, http.StatusOK, program.Object{
		"schemaVersion": "1.1",
		"program":       out,
		"issues":        issues,
	})
}

func (h *Handler) validate(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeBody(w, r)
	if !ok {
		return
	}
	prog, _ := program.AsObject(body["program"])
	if prog == nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "program is required")
		return
	}
	checks := validate.ValidateProgram(prog)
	report, issues := validate.BuildReport(checks, 0)
	writeJSON(w, http.StatusOK, program.Object{
		"schemaVersion":    "1.1",
		"validationReport": report,
		"issues":           issues,
	})
}

func decodeBody(w http.ResponseWriter, r *http.Request) (program.Object, bool) {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, 32<<20)
	dec := json.NewDecoder(limited)
	dec.UseNumber()
	var body program.Object
	if err := dec.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "malformed JSON: "+err.Error())
		return nil, false
	}
	// Normalize json.Number → float64 for easier handling.
	normalizeNumbers(body)
	if sv, ok := body["schemaVersion"].(string); ok && sv != "" && sv != "1.1" {
		writeError(w, http.StatusUnprocessableEntity, CodeSchemaInvalid, "schemaVersion must be 1.1")
		return nil, false
	}
	return body, true
}

func normalizeNumbers(v any) {
	switch t := v.(type) {
	case program.Object:
		for k, val := range t {
			switch n := val.(type) {
			case json.Number:
				if f, err := n.Float64(); err == nil {
					t[k] = f
				}
			default:
				normalizeNumbers(val)
			}
		}
	case map[string]any:
		normalizeNumbers(program.Object(t))
	case program.Array:
		for i, val := range t {
			if n, ok := val.(json.Number); ok {
				if f, err := n.Float64(); err == nil {
					t[i] = f
				}
			} else {
				normalizeNumbers(val)
			}
		}
	case []any:
		normalizeNumbers(program.Array(t))
	}
}

func writeLLMError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, llm.ErrTimeout):
		writeError(w, http.StatusGatewayTimeout, CodeLLMTimeout, err.Error())
	case errors.Is(err, llm.ErrSchemaInvalid):
		writeError(w, http.StatusUnprocessableEntity, CodeSchemaInvalid, err.Error())
	case errors.Is(err, llm.ErrUpstream):
		writeError(w, http.StatusBadGateway, CodeLLMError, err.Error())
	case llm.NodeLocked(err):
		writeError(w, http.StatusConflict, CodeNodeLocked, err.Error())
	default:
		writeError(w, http.StatusBadGateway, CodeLLMError, err.Error())
	}
}

func mustObj(v any) program.Object {
	o, _ := program.AsObject(v)
	if o == nil {
		return program.Object{}
	}
	return o
}
