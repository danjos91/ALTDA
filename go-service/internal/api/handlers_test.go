package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/danjos91/ALTDA/go-service/internal/llm"
	"github.com/danjos91/ALTDA/go-service/internal/program"
)

func TestHealth(t *testing.T) {
	h := testHandler(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	h.routes().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status=%d", rr.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Fatalf("body=%v", body)
	}
}

func TestParseAndSkeletonStub(t *testing.T) {
	h := testHandler(t)
	rr := post(t, h, "/v1/parse", map[string]any{
		"schemaVersion":  "1.1",
		"rawText":        "ДПП ПК Анализ данных на Python, 72 ч",
		"clarifications": []any{},
	})
	if rr.Code != 200 {
		t.Fatalf("parse status=%d body=%s", rr.Code, rr.Body.String())
	}
	var parsed map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &parsed)
	if parsed["schemaVersion"] != "1.1" {
		t.Fatal("missing schemaVersion")
	}

	// Answer clarification
	rr = post(t, h, "/v1/parse", map[string]any{
		"schemaVersion": "1.1",
		"rawText":       "ДПП ПК Анализ данных на Python, 72 ч",
		"clarifications": []any{
			map[string]any{"param": "entryLevel", "question": "q", "answer": "Базовый Python"},
		},
	})
	if rr.Code != 200 {
		t.Fatalf("parse2=%d %s", rr.Code, rr.Body.String())
	}

	prog := map[string]any{
		"schemaVersion": "1.1",
		"id":            "p1",
		"status":        "clarifying",
		"request": map[string]any{
			"rawText": "ДПП ПК Анализ данных на Python, 72 ч",
			"params": map[string]any{
				"programType": "DPP_PK", "totalHours": 72.0, "title": "Анализ данных",
			},
		},
		"header": map[string]any{"totalHours": 72.0},
	}
	rr = post(t, h, "/v1/skeleton/generate", map[string]any{
		"schemaVersion": "1.1", "program": prog, "selectedVariantKey": nil,
	})
	if rr.Code != 200 {
		t.Fatalf("skeleton=%d %s", rr.Code, rr.Body.String())
	}
	var skel map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &skel)
	variants, _ := skel["variants"].([]any)
	if len(variants) == 0 || len(variants) > 3 {
		t.Fatalf("variants len=%d", len(variants))
	}
	v0 := variants[0].(map[string]any)
	totals := v0["totals"].(map[string]any)
	if totals["total"].(float64) != 72 {
		t.Fatalf("allocated total=%v", totals["total"])
	}
}

func TestNodeLocked409(t *testing.T) {
	h := testHandler(t)
	prog := map[string]any{
		"schemaVersion": "1.1",
		"id":            "p1",
		"status":        "skeleton_proposed",
		"header":        map[string]any{"totalHours": 16.0},
		"curriculum": map[string]any{
			"modules": []any{
				map[string]any{
					"id": "M1", "title": "M", "weight": 1.0, "isIntro": false,
					"hours": map[string]any{"lecture": 8, "practice": 6, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 14, "total": 14},
					"topics": []any{
						map[string]any{
							"id": "T1.1", "title": "t", "weight": 1.0,
							"_meta": map[string]any{"locked": true},
							"hours": map[string]any{"lecture": 8, "practice": 6, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 14, "total": 14},
							"outcomeIds": []any{"C1.K1"},
						},
					},
				},
			},
			"totals": map[string]any{"lecture": 8, "practice": 6, "lab": 0, "selfStudy": 0, "assessment": 2, "contact": 14, "total": 16},
		},
		"finalAttestation": map[string]any{
			"form": "зачёт", "description": "d",
			"hours": map[string]any{"lecture": 0, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 2, "contact": 0, "total": 2},
		},
	}
	rr := post(t, h, "/v1/skeleton/regenerate", map[string]any{
		"schemaVersion": "1.1",
		"program":       prog,
		"comment":       "fix",
		"nodeIds":       []any{"T1.1"},
		"respectLocks":  true,
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var errBody ErrorBody
	_ = json.Unmarshal(rr.Body.Bytes(), &errBody)
	if errBody.Error != CodeNodeLocked {
		t.Fatalf("error=%q", errBody.Error)
	}
}

func TestAllocateAndValidate(t *testing.T) {
	h := testHandler(t)
	prog := map[string]any{
		"schemaVersion": "1.1",
		"status":        "skeleton_proposed",
		"header":        map[string]any{"totalHours": 24.0},
		"competencies": []any{
			map[string]any{
				"id": "C1", "title": "c", "kind": "новая",
				"profStandardLinks": []any{},
				"knowledge":         []any{map[string]any{"id": "C1.K1", "text": "k"}},
				"skills":            []any{},
				"abilities":         []any{},
			},
		},
		"curriculum": map[string]any{
			"modules": []any{
				map[string]any{
					"id": "M1", "title": "M", "weight": 1.0, "isIntro": false, "assessmentForm": "нет",
					"topics": []any{
						map[string]any{
							"id": "T1.1", "title": "t", "weight": 1.0,
							"kinds": []any{"lecture", "practice"}, "outcomeIds": []any{"C1.K1"},
						},
					},
				},
			},
		},
		"finalAttestation": map[string]any{"form": "зачёт", "description": "d", "outcomeIds": []any{"C1.K1"}},
		"goal":             map[string]any{"text": "g"},
	}
	rr := post(t, h, "/v1/allocate", map[string]any{"schemaVersion": "1.1", "program": prog})
	if rr.Code != 200 {
		t.Fatalf("allocate=%d %s", rr.Code, rr.Body.String())
	}
	var alloc map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &alloc)
	allocated := alloc["program"].(map[string]any)
	totals := allocated["curriculum"].(map[string]any)["totals"].(map[string]any)
	if totals["total"].(float64) != 24 {
		t.Fatalf("totals=%v", totals["total"])
	}

	rr = post(t, h, "/v1/validate", map[string]any{"schemaVersion": "1.1", "program": allocated})
	if rr.Code != 200 {
		t.Fatalf("validate=%d %s", rr.Code, rr.Body.String())
	}
	var val map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &val)
	if val["issues"] == nil {
		t.Fatal("issues must not be null")
	}
	if _, ok := val["validationReport"].(map[string]any); !ok {
		t.Fatal("validationReport missing")
	}
}

func TestContentIssuesAlwaysArray(t *testing.T) {
	h := testHandler(t)
	prog := map[string]any{
		"schemaVersion": "1.1",
		"id":            "x",
		"status":        "skeleton_approved",
		"programType":   "DOOP_GENERAL",
		"header":        map[string]any{"totalHours": 16.0, "title": "t", "itDirection": "IT", "studyForm": "очная", "audience": map[string]any{"description": "a", "entryLevel": "e"}, "issuedDocument": "Сертификат"},
		"request":       map[string]any{"rawText": "дооп 16 ч", "params": map[string]any{"programType": "DOOP_GENERAL", "totalHours": 16.0}},
		"goal":          map[string]any{"text": "g"},
		"competencies":  []any{},
		"curriculum": map[string]any{
			"modules": []any{},
			"totals":  map[string]any{"lecture": 0, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 0, "contact": 0, "total": 16},
		},
		"finalAttestation": map[string]any{"form": "зачёт", "description": "d", "hours": map[string]any{"lecture": 0, "practice": 0, "lab": 0, "selfStudy": 0, "assessment": 1, "contact": 0, "total": 1}},
	}
	rr := post(t, h, "/v1/content", map[string]any{"schemaVersion": "1.1", "program": prog, "nodeIds": nil})
	if rr.Code != 200 {
		t.Fatalf("content=%d %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	issues, ok := body["issues"].([]any)
	if !ok || issues == nil {
		t.Fatalf("issues not array: %T", body["issues"])
	}
}

func TestBadRequest(t *testing.T) {
	h := testHandler(t)
	rr := post(t, h, "/v1/parse", map[string]any{"schemaVersion": "1.1"})
	if rr.Code != 400 {
		t.Fatalf("status=%d", rr.Code)
	}
	var errBody ErrorBody
	_ = json.Unmarshal(rr.Body.Bytes(), &errBody)
	if errBody.Error != CodeBadRequest {
		t.Fatalf("code=%s", errBody.Error)
	}
}

func TestStubEndToEndPipeline(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	_ = ctx
	raw := "ДООП Робототехника для начинающих, 24 ч"
	rr := post(t, h, "/v1/parse", map[string]any{
		"schemaVersion": "1.1", "rawText": raw,
		"clarifications": []any{map[string]any{"param": "entryLevel", "question": "q", "answer": "нулевой"}},
	})
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var parsed map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &parsed)
	params := parsed["params"].(map[string]any)

	prog := map[string]any{
		"schemaVersion": "1.1", "id": "e2e", "status": "clarifying",
		"programType": params["programType"],
		"request":     map[string]any{"rawText": raw, "params": params, "clarifications": parsed["clarifications"], "missing": []any{}, "gapsReport": []any{}},
		"header":      map[string]any{"totalHours": params["totalHours"]},
	}
	rr = post(t, h, "/v1/skeleton/generate", map[string]any{"schemaVersion": "1.1", "program": prog})
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var skel map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &skel)
	variant := skel["variants"].([]any)[0].(map[string]any)

	// Merge-lite for content input
	prog["status"] = "skeleton_approved"
	prog["goal"] = map[string]any{"text": variant["goal"]}
	prog["competencies"] = variant["competencies"]
	prog["curriculum"] = map[string]any{"modules": variant["modules"], "totals": variant["totals"]}
	prog["finalAttestation"] = variant["finalAttestation"]

	rr = post(t, h, "/v1/content", map[string]any{"schemaVersion": "1.1", "program": prog, "nodeIds": nil})
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var content map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &content)
	if content["issues"] == nil {
		t.Fatal("issues null")
	}
	outProg := content["program"].(map[string]any)

	rr = post(t, h, "/v1/validate", map[string]any{"schemaVersion": "1.1", "program": outProg})
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
}

func testHandler(t *testing.T) *Handler {
	t.Helper()
	dir := fixturesDir(t)
	client, err := llm.New(llm.Options{Stub: true, Fixtures: dir})
	if err != nil {
		t.Fatal(err)
	}
	return &Handler{LLM: client}
}

func fixturesDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "testdata")
	if _, err := os.Stat(filepath.Join(dir, "example_program.json")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func post(t *testing.T, h *Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	h.routes().ServeHTTP(rr, req)
	return rr
}

// silence unused import if program not used in some builds
var _ = program.Object{}
