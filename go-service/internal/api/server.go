package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danjos91/ALTDA/go-service/internal/config"
)

// NewServer builds an http.Server with timeouts suitable for /v1/content.
func NewServer(cfg config.Config, h *Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           h.routes(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       120 * time.Second,
	}
}

// ProbeHealth checks GET /v1/health on addr (for Docker HEALTHCHECK / -healthcheck).
func ProbeHealth(addr string) error {
	if addr == "" {
		addr = ":8081"
	}
	url := "http://127.0.0.1" + addr + "/v1/health"
	if addr[0] != ':' {
		url = "http://" + addr + "/v1/health"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health status %d", resp.StatusCode)
	}
	return nil
}
