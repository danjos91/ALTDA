package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbe(t *testing.T) {
	t.Run("a healthy service", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/health" {
				t.Errorf("probed %q, want /v1/health", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		if err := Probe(context.Background(), ts.URL); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})

	t.Run("an unhealthy service", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		if err := Probe(context.Background(), ts.URL); err == nil {
			t.Error("a 500 must fail the probe")
		}
	})

	t.Run("nothing listening", func(t *testing.T) {
		ts := httptest.NewServer(http.NotFoundHandler())
		url := ts.URL
		ts.Close()

		if err := Probe(context.Background(), url); err == nil {
			t.Error("a closed port must fail the probe")
		}
	})
}
