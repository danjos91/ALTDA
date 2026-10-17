package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/danjos91/ALTDA/go-service/internal/api"
	"github.com/danjos91/ALTDA/go-service/internal/config"
	"github.com/danjos91/ALTDA/go-service/internal/llm"
)

func main() {
	cfg := config.Load()
	if cfg.HealthcheckOnly {
		if err := api.ProbeHealth(cfg.Addr); err != nil {
			log.Printf("healthcheck failed: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	fixtures := cfg.FixturesDir
	if fixtures == "" {
		fixtures = defaultFixtures()
	}
	client, err := llm.New(llm.Options{
		Stub:       cfg.StubMode,
		BaseURL:    cfg.OpenAIBaseURL,
		APIKey:     cfg.OpenAIAPIKey,
		Model:      cfg.OpenAIModel,
		Fixtures:   fixtures,
		SchemaPath: cfg.SchemaPath,
	})
	if err != nil {
		log.Fatalf("llm client: %v", err)
	}

	mode := "openai"
	if cfg.StubMode {
		mode = "stub"
	}
	log.Printf("altda go-service listening on %s (mode=%s writeTimeout=%s)", cfg.Addr, mode, cfg.WriteTimeout)

	h := &api.Handler{LLM: client}
	srv := api.NewServer(cfg, h)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func defaultFixtures() string {
	candidates := []string{
		"testdata",
		filepath.Join("go-service", "testdata"),
		"/app/testdata",
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "example_program.json")); err == nil && !st.IsDir() {
			return c
		}
	}
	return "testdata"
}
