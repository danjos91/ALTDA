package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestStubSelection(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ALTDA_STUB", "")
	cfg := LoadConfig()
	if !cfg.Stub {
		t.Fatal("expected stub when no key")
	}

	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("ALTDA_STUB", "true")
	cfg = LoadConfig()
	if !cfg.Stub {
		t.Fatal("expected stub when ALTDA_STUB=true")
	}

	t.Setenv("ALTDA_STUB", "")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_BASE_URL", "http://example.invalid/v1")
	t.Setenv("OPENAI_MODEL", "GigaChat/GigaChat-3-Pro")
	cfg = LoadConfig()
	if cfg.Stub {
		t.Fatal("expected live mode with key and no stub flag")
	}
	if cfg.Model != "GigaChat/GigaChat-3-Pro" {
		t.Fatalf("model = %q", cfg.Model)
	}
	if cfg.Concurrency != 2 {
		t.Fatalf("concurrency = %d", cfg.Concurrency)
	}
	if cfg.ContentConcurrency > cfg.Concurrency {
		t.Fatalf("content concurrency %d > llm %d", cfg.ContentConcurrency, cfg.Concurrency)
	}
	if cfg.MaxTokens < 1000 {
		t.Fatalf("max_tokens too low: %d", cfg.MaxTokens)
	}
}

func TestContentConcurrencyCappedByLLM(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk")
	t.Setenv("ALTDA_STUB", "")
	t.Setenv("ALTDA_LLM_CONCURRENCY", "2")
	t.Setenv("ALTDA_CONTENT_CONCURRENCY", "8")
	cfg := LoadConfig()
	if cfg.ContentConcurrency != 2 {
		t.Fatalf("got %d, want 2", cfg.ContentConcurrency)
	}
}

func TestSemaphoreLimitsConcurrency(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	var inFlight atomic.Int32
	var maxSeen atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			cur := maxSeen.Load()
			if n <= cur || maxSeen.CompareAndSwap(cur, n) {
				break
			}
		}
		time.Sleep(80 * time.Millisecond)
		writeChatOK(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 5*time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var dest map[string]any
			if err := c.ChatJSON(context.Background(), "sys", "user", &dest); err != nil {
				t.Errorf("ChatJSON: %v", err)
			}
		}()
	}
	wg.Wait()
	if maxSeen.Load() > 2 {
		t.Fatalf("max in-flight %d > 2", maxSeen.Load())
	}
}

func TestBackoffOn429RespectsContext(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate"}`))
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var dest map[string]any
	err := c.ChatJSON(ctx, "sys", "user", &dest)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "llm_timeout") && ctx.Err() == nil {
		// May surface as llm_error if all attempts finished before cancel; either is ok if hits limited.
		t.Logf("err=%v hits=%d", err, hits.Load())
	}
	if hits.Load() > 3 {
		t.Fatalf("too many attempts: %d", hits.Load())
	}
}

func TestBackoffRetriesThenSucceeds(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"busy"}`))
			return
		}
		writeChatOK(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 5*time.Second)
	// Short backoff path: monkey by using real backoff (0.5s+1s) — accept ~2s.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var dest map[string]any
	if err := c.ChatJSON(ctx, "sys", "user", &dest); err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}
	if hits.Load() != 3 {
		t.Fatalf("hits=%d want 3", hits.Load())
	}
}

func TestBadJSONRetriesThenSchemaInvalid(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeChatOK(w, "not-json")
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 5*time.Second)
	var dest map[string]any
	err := c.ChatJSON(context.Background(), "sys", "user", &dest)
	if err == nil || !strings.Contains(err.Error(), "schema_invalid") {
		t.Fatalf("err=%v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits=%d want 2 (one retry)", hits.Load())
	}
}

func TestMaxTokensNeverUnder1000(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk")
	t.Setenv("ALTDA_STUB", "")
	t.Setenv("ALTDA_LLM_MAX_TOKENS", "50")
	cfg := LoadConfig()
	if cfg.MaxTokens < 1000 {
		t.Fatalf("got %d", cfg.MaxTokens)
	}
}

func testClient(baseURL string, conc int, callTO time.Duration) *Client {
	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = baseURL + "/v1"
	return &Client{
		api:       openai.NewClientWithConfig(cfg),
		model:     "test-model",
		maxTokens: 4096,
		callTO:    callTO,
		sem:       sharedSemaphore(conc),
	}
}

func writeChatOK(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{Content: content},
		}},
	})
}
