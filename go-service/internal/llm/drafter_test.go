package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
)

func TestDraftParallelVariantsPartialSuccess(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		user := ""
		if len(body.Messages) > 0 {
			user = body.Messages[len(body.Messages)-1].Content
		}
		// Fail variant B once (all attempts for that key).
		if strings.Contains(user, `key="B"`) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"fail B"}`))
			return
		}
		key := "A"
		if strings.Contains(user, `key="C"`) {
			key = "C"
		}
		writeChatOK(w, minimalVariantJSON(key))
		_ = n
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 5*time.Second)
	d := NewDrafter(c)
	drafts, err := d.Draft(context.Background(), skeleton.Request{
		Title: "Test", Domain: "IT", ProgramType: "DOOP_GENERAL", TotalHours: 24,
	})
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if len(drafts) < 1 {
		t.Fatalf("want ≥1 draft, got %d", len(drafts))
	}
	for _, d := range drafts {
		if d.Key == "B" {
			t.Fatal("variant B should have failed")
		}
	}
	if hits.Load() < 3 {
		t.Fatalf("expected parallel calls for A/B/C, hits=%d", hits.Load())
	}
}

func TestDraftAllVariantsFail(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 2*time.Second)
	_, err := NewDrafter(c).Draft(context.Background(), skeleton.Request{Title: "x", TotalHours: 16})
	if err == nil || !strings.Contains(err.Error(), "llm_error") {
		t.Fatalf("err=%v", err)
	}
}

func minimalVariantJSON(key string) string {
	return `{
  "variant": {
    "key": "` + key + `",
    "rationale": "r",
    "pros": ["p"],
    "cons": ["c"],
    "goal": "g",
    "competencies": [{
      "id": "C1", "title": "t", "activityType": null, "profStandardLinks": [],
      "knowledge": [{"id": "C1.K1", "text": "знать"}],
      "skills": [{"id": "C1.U1", "text": "уметь"}],
      "abilities": [{"id": "C1.N1", "text": "навык"}]
    }],
    "modules": [{
      "id": "M1", "title": "m", "isIntro": true, "weight": 1,
      "topics": [{
        "id": "T1.1", "title": "t", "weight": 1,
        "kinds": ["lecture", "practice"],
        "outcomeIds": ["C1.K1", "C1.U1", "C1.N1"],
        "prerequisites": []
      }]
    }],
    "finalAttestation": {"form": "зачёт", "description": "d", "outcomeIds": ["C1.K1"]}
  }
}`
}
