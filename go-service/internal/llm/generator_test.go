package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/program"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/content"
)

func TestTopicFallsBackToStubOnLLMFailure(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = w.Write([]byte(`{"error":"gateway timeout"}`))
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 2*time.Second)
	g := NewGenerator(c).(*Generator)

	in := content.TopicInput{
		ProgramTitle: "Курс",
		Domain:       "IT",
		Module:       content.ModuleInfo{ID: "M1", Title: "Модуль"},
		Topic: content.TopicInfo{
			ID:    "T1.1",
			Title: "Тема",
			Hours: program.Hours{Lecture: 2, Practice: 2, Contact: 4, Total: 4},
		},
		Outcomes: []content.OutcomeInfo{{ID: "C1.K1", Text: "знать"}},
	}

	out, err := g.Topic(context.Background(), in)
	if err != nil {
		t.Fatalf("Topic: %v", err)
	}
	if len(out.Activities) == 0 {
		t.Fatal("expected stub activities")
	}
	foundStub := false
	for _, a := range out.Activities {
		if strings.Contains(a.Content, "[Шаблон]") {
			foundStub = true
			break
		}
	}
	if !foundStub {
		t.Fatalf("expected stub marker in activities: %+v", out.Activities)
	}

	warns := g.ConsumeWarnings()
	if len(warns) != 1 {
		t.Fatalf("warnings=%d want 1: %+v", len(warns), warns)
	}
	if warns[0].Severity != "warning" || warns[0].RuleID != "LLM_TOPIC_FALLBACK" {
		t.Fatalf("warning = %+v", warns[0])
	}
	if !strings.Contains(warns[0].Message, "T1.1") {
		t.Fatalf("message = %q", warns[0].Message)
	}
	// 504 is retryable → up to 3 attempts
	if hits.Load() < 2 {
		t.Fatalf("expected retries on 504, hits=%d", hits.Load())
	}
}

func TestRetryOn504BeforeTimeoutClassification(t *testing.T) {
	ResetSharedSemaphoreForTest()
	defer ResetSharedSemaphoreForTest()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusGatewayTimeout)
			_, _ = w.Write([]byte(`Gateway Timeout`))
			return
		}
		writeChatOK(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := testClient(srv.URL, 2, 5*time.Second)
	var dest map[string]any
	if err := c.ChatJSON(context.Background(), "sys", "user", &dest); err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}
	if hits.Load() != 3 {
		t.Fatalf("hits=%d want 3 (retry 504 twice then success)", hits.Load())
	}
}

func TestStubBoolParsing(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk")
	for _, v := range []string{"1", "yes", "true", "TRUE"} {
		t.Setenv("ALTDA_STUB", v)
		if !LoadConfig().Stub {
			t.Fatalf("ALTDA_STUB=%q should enable stub", v)
		}
	}
	t.Setenv("ALTDA_STUB", "")
	if LoadConfig().Stub {
		t.Fatal("empty ALTDA_STUB with key should be live mode")
	}
}
