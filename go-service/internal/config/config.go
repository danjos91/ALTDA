package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration for the Go service.
type Config struct {
	Addr              string
	OpenAIBaseURL     string
	OpenAIAPIKey      string
	OpenAIModel       string
	StubMode          bool
	SchemaPath        string
	FixturesDir       string
	WriteTimeout      time.Duration
	ReadHeaderTimeout time.Duration
	HealthcheckOnly   bool
}

// Load parses flags and environment variables.
func Load() Config {
	cfg := Config{
		Addr:              envOr("PORT", ":8081"),
		OpenAIBaseURL:     envOr("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:       envOr("OPENAI_MODEL", "gpt-4o-mini"),
		SchemaPath:        envOr("ALTDA_SCHEMA_PATH", ""),
		FixturesDir:       envOr("ALTDA_FIXTURES_DIR", ""),
		WriteTimeout:      12 * time.Minute,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if !strings.HasPrefix(cfg.Addr, ":") && !strings.Contains(cfg.Addr, ":") {
		cfg.Addr = ":" + cfg.Addr
	}

	stubFlag := flag.Bool("stub", false, "force stub mode (no LLM)")
	addrFlag := flag.String("addr", cfg.Addr, "listen address (default :8081)")
	healthcheck := flag.Bool("healthcheck", false, "probe GET /v1/health on -addr and exit")
	flag.Parse()

	cfg.Addr = *addrFlag
	cfg.HealthcheckOnly = *healthcheck
	if *stubFlag || strings.TrimSpace(cfg.OpenAIAPIKey) == "" {
		cfg.StubMode = true
	}
	if v := os.Getenv("ALTDA_STUB"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.StubMode = b
		}
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
