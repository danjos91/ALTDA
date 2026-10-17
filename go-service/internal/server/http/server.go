package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const shutdownTimeout = 10 * time.Second

type Registrar interface {
	Register(mux *http.ServeMux)
}

type Server struct {
	config   *Config
	handlers []Registrar
}

func NewServer(config *Config, handlers ...Registrar) *Server {
	return &Server{config: config, handlers: handlers}
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	for _, handler := range s.handlers {
		handler.Register(mux)
	}

	srv := &http.Server{
		Addr:              s.config.Address(),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		// Long enough for /v1/content (default 16m > Java content-ms 900s).
		WriteTimeout: s.config.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "http server listening",
			slog.String("addr", srv.Addr),
			slog.Duration("write_timeout", srv.WriteTimeout),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
