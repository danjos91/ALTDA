package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	httpserver "github.com/VladimirAzanza/ALTDA/go-service/internal/server/http"
)

const probeTimeout = 3 * time.Second

// HealthCheck asks the service running on this machine whether it is up. It
// backs the "healthcheck" command of the binary: the container image has no
// shell or curl, so Docker runs the binary itself to check it.
func HealthCheck(ctx context.Context) error {
	return Probe(ctx, "http://127.0.0.1:"+httpserver.NewConfig().Port)
}

// Probe calls GET /v1/health on baseURL and succeeds only on a 200.
func Probe(ctx context.Context, baseURL string) error {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/health", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check answered %d", resp.StatusCode)
	}
	return nil
}
