package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
)

func testDrafter(c *Client, callTO, softMargin time.Duration) *Drafter {
	return &Drafter{client: c, callTO: callTO, softMargin: softMargin}
}

func TestDraftParallelVariantsPartialSuccess(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
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
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 5*time.Second)
	d := testDrafter(c, 5*time.Second, 15*time.Second)
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
	_, err := testDrafter(c, 2*time.Second, 15*time.Second).Draft(
		context.Background(), skeleton.Request{Title: "x", TotalHours: 16})
	if err == nil || !strings.Contains(err.Error(), "llm_error") {
		t.Fatalf("err=%v", err)
	}
}

func TestDraftSoftDeadlineReturnsFinishedVariants(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
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
		key := "A"
		if strings.Contains(user, `key="B"`) {
			key = "B"
		}
		if strings.Contains(user, `key="C"`) {
			key = "C"
		}
		// A is fast; B and C hang until the soft deadline cancels them.
		if key != "A" {
			time.Sleep(5 * time.Second)
		}
		writeChatOK(w, minimalVariantJSON(key))
	}))
	defer srv.Close()

	// Concurrency 3 so the fast variant is not blocked behind slow ones on the semaphore.
	c := testClient(srv.URL, 3, 5*time.Second)
	d := testDrafter(c, 5*time.Second, 400*time.Millisecond)

	// Endpoint-like deadline 1.2s; soft = 1.2s - 400ms ≈ 800ms from now.
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	start := time.Now()
	drafts, err := d.Draft(ctx, skeleton.Request{Title: "soft", TotalHours: 24})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Draft: %v (want finished variants, not error)", err)
	}
	if len(drafts) < 1 {
		t.Fatalf("want ≥1 finished variant, got 0")
	}
	for _, dr := range drafts {
		if dr.Key != "A" {
			t.Fatalf("unexpected key %q (slow variants should have been dropped)", dr.Key)
		}
	}
	if elapsed > 2500*time.Millisecond {
		t.Fatalf("took %v; soft deadline should have cut short", elapsed)
	}
}

func TestDraftSoftDeadlineNoneReadyIsTimeout(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		writeChatOK(w, minimalVariantJSON("A"))
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 5*time.Second)
	d := testDrafter(c, 5*time.Second, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	_, err := d.Draft(ctx, skeleton.Request{Title: "none", TotalHours: 16})
	if err == nil || !errors.Is(err, ErrTimeout) {
		t.Fatalf("err=%v, want ErrTimeout", err)
	}
}

func TestSkeletonCallTimeoutDefault(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk")
	t.Setenv("ALTDA_STUB", "")
	t.Setenv("ALTDA_SKELETON_CALL_TIMEOUT_SEC", "")
	cfg := LoadConfig()
	if cfg.SkeletonCallTimeout != 90*time.Second {
		t.Fatalf("SkeletonCallTimeout=%v want 90s", cfg.SkeletonCallTimeout)
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
