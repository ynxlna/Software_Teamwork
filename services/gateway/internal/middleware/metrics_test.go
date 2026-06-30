package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/metrics"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/gateway/internal/middleware"
)

func TestMetricsMiddlewarePassesThrough(t *testing.T) {
	reg := metrics.New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Chain(mux, middleware.Metrics(reg, mux))

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestMetricsMiddlewareCapturesNonOKStatus(t *testing.T) {
	reg := metrics.New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /fail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	handler := middleware.Chain(mux, middleware.Metrics(reg, mux))

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestMetricsMiddlewareNilRegistryIsNoOp(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Chain(mux, middleware.Metrics(nil, mux))

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}
