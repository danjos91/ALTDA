package llm

import (
	"log/slog"
	"strings"
	"time"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/env"
)

// Config is process-wide LLM and gateway settings.
type Config struct {
	Stub bool

	BaseURL string
	APIKey  string
	Model   string

	// Concurrency is the process-wide LLM semaphore size (default 2).
	Concurrency int
	// ContentConcurrency is per-topic parallelism for content (≤ Concurrency).
	ContentConcurrency int

	CallTimeout time.Duration // default content/general LLM call (110s)
	MaxTokens   int

	// SkeletonCallTimeout is the per-variant LLM call cap (default 90s).
	SkeletonCallTimeout time.Duration
	// SkeletonSoftMargin is how far before the endpoint deadline we stop
	// waiting for stragglers and return finished variants (default 15s).
	SkeletonSoftMargin time.Duration

	// Endpoint deadlines (slightly below Java read timeouts).
	SkeletonDeadline time.Duration
	ContentDeadline  time.Duration

	WriteTimeout time.Duration
}

// LoadConfig reads OPENAI_* / ALTDA_* from the environment.
func LoadConfig() Config {
	stubFlag := env.Bool("ALTDA_STUB", false)
	apiKey := strings.TrimSpace(env.String("OPENAI_API_KEY", ""))
	stub := stubFlag || apiKey == ""

	concurrency := env.Int("ALTDA_LLM_CONCURRENCY", 2)
	if concurrency < 1 {
		concurrency = 2
	}

	contentConc := env.Int("ALTDA_CONTENT_CONCURRENCY", 2)
	if contentConc < 1 {
		contentConc = 2
	}
	if contentConc > concurrency {
		contentConc = concurrency
	}

	maxTokens := env.Int("ALTDA_LLM_MAX_TOKENS", 4096)
	if maxTokens < 1000 {
		maxTokens = 1000
	}

	callSec := env.Int("ALTDA_LLM_CALL_TIMEOUT_SEC", 110)
	if callSec < 1 {
		callSec = 110
	}

	skelCallSec := env.Int("ALTDA_SKELETON_CALL_TIMEOUT_SEC", 90)
	if skelCallSec < 1 {
		skelCallSec = 90
	}

	skelSec := env.Int("ALTDA_SKELETON_DEADLINE_SEC", 280)
	if skelSec < 1 {
		skelSec = 280
	}
	softMarginSec := env.Int("ALTDA_SKELETON_SOFT_MARGIN_SEC", 15)
	if softMarginSec < 1 {
		softMarginSec = 15
	}
	contentSec := env.Int("ALTDA_CONTENT_DEADLINE_SEC", 870)
	if contentSec < 1 {
		contentSec = 870
	}

	writeMin := env.Int("ALTDA_HTTP_WRITE_TIMEOUT_MIN", 16)
	if writeMin < 1 {
		writeMin = 16
	}

	cfg := Config{
		Stub:               stub,
		BaseURL:            strings.TrimRight(env.String("OPENAI_BASE_URL", ""), "/"),
		APIKey:             apiKey,
		Model:              env.String("OPENAI_MODEL", "GigaChat/GigaChat-3-Pro"),
		Concurrency:        concurrency,
		ContentConcurrency: contentConc,
		CallTimeout:         time.Duration(callSec) * time.Second,
		MaxTokens:           maxTokens,
		SkeletonCallTimeout: time.Duration(skelCallSec) * time.Second,
		SkeletonSoftMargin:  time.Duration(softMarginSec) * time.Second,
		SkeletonDeadline:    time.Duration(skelSec) * time.Second,
		ContentDeadline:     time.Duration(contentSec) * time.Second,
		WriteTimeout:        time.Duration(writeMin) * time.Minute,
	}

	if cfg.Stub {
		reason := "no OPENAI_API_KEY"
		if stubFlag {
			reason = "ALTDA_STUB=true"
		}
		slog.Info("llm mode: stub (template stand-ins)", slog.String("reason", reason))
	} else {
		slog.Info("llm mode: openai-compatible",
			slog.String("base_url", cfg.BaseURL),
			slog.String("model", cfg.Model),
			slog.Int("concurrency", cfg.Concurrency),
			slog.Int("content_concurrency", cfg.ContentConcurrency),
			slog.Int("max_tokens", cfg.MaxTokens),
			slog.Duration("call_timeout", cfg.CallTimeout),
		)
	}
	return cfg
}
