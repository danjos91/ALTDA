package app

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/VladimirAzanza/ALTDA/go-service/internal/llm"
	httpserver "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http"
	allocatehandler "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/handler/allocate"
	contenthandler "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/handler/content"
	healthhandler "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/handler/health"
	parsehandler "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/handler/parse"
	skeletonhandler "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/handler/skeleton"
	validatehandler "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http/handler/validate"
	allocateservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocate"
	"github.com/VladimirAzanza/ALTDA/go-service/internal/service/allocator"
	contentservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/content"
	parseservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/parse"
	skeletonservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/skeleton"
	validateservice "github.com/VladimirAzanza/ALTDA/go-service/internal/service/validate"
)

type App struct {
	config *Config
}

func New() *App {
	config := NewConfig()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(config.LogLevel),
	})))

	return &App{config: config}
}

func (app *App) Run(ctx context.Context) error {
	allocatorConfig := allocator.DefaultConfig()
	llmCfg := llm.LoadConfig()

	parseService := parseservice.NewService()
	allocateService := allocateservice.NewService(allocatorConfig)
	validateService := validateservice.NewService()

	var drafter skeletonservice.Drafter = skeletonservice.NewTemplateDrafter()
	var regenerator skeletonservice.Regenerator = skeletonservice.NewTemplateRegenerator()
	var generator contentservice.Generator = contentservice.NewTemplateGenerator()

	if !llmCfg.Stub {
		client := llm.NewClient(llmCfg)
		drafter = llm.NewDrafter(client)
		regenerator = llm.NewRegenerator(client)
		generator = llm.NewGenerator(client)
	}

	skeletonService := skeletonservice.NewService(drafter, regenerator, allocatorConfig)
	contentCfg := contentservice.DefaultConfig()
	contentCfg.MaxParallel = llmCfg.ContentConcurrency
	contentService := contentservice.NewService(generator, validateService, contentCfg)

	httpCfg := httpserver.NewConfig()
	httpCfg.WriteTimeout = llmCfg.WriteTimeout
	httpCfg.SkeletonDeadline = llmCfg.SkeletonDeadline
	httpCfg.ContentDeadline = llmCfg.ContentDeadline

	server := httpserver.NewServer(
		httpCfg,
		healthhandler.NewHandler(),
		parsehandler.NewHandler(parseService),
		allocatehandler.NewHandler(allocateService),
		validatehandler.NewHandler(validateService),
		skeletonhandler.NewHandler(skeletonService, httpCfg.SkeletonDeadline),
		contenthandler.NewHandler(contentService, httpCfg.ContentDeadline),
	)

	return server.Run(ctx)
}

func parseLogLevel(level string) slog.Level {
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(strings.ToUpper(level))); err != nil {
		return slog.LevelInfo
	}
	return parsed
}
