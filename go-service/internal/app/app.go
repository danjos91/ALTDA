package app

import (
	"context"
	"log/slog"
	"os"
	"strings"

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

	parseService := parseservice.NewService()
	allocateService := allocateservice.NewService(allocatorConfig)
	validateService := validateservice.NewService()
	skeletonService := skeletonservice.NewService(skeletonservice.NewTemplateDrafter(), skeletonservice.NewTemplateRegenerator(), allocatorConfig)
	contentService := contentservice.NewService(contentservice.NewTemplateGenerator(), validateService, contentservice.DefaultConfig())

	server := httpserver.NewServer(
		httpserver.NewConfig(),
		healthhandler.NewHandler(),
		parsehandler.NewHandler(parseService),
		allocatehandler.NewHandler(allocateService),
		validatehandler.NewHandler(validateService),
		skeletonhandler.NewHandler(skeletonService),
		contenthandler.NewHandler(contentService),
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
