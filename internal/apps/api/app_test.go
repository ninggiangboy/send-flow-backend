package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
)

func TestHealthz(t *testing.T) {
	svc := platformhealth.NewService(platformhealth.Options{
		AppName: "sendflow",
		PostgresCheck: func(context.Context) error {
			return nil
		},
		RedisCheck: func(context.Context) error {
			return nil
		},
	})
	metrics := observability.NewHTTPMetrics(nil)
	logger := slog.Default()
	router := newRouter(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "http://localhost:3000", metrics, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestReadyzDegraded(t *testing.T) {
	svc := platformhealth.NewService(platformhealth.Options{
		AppName: "sendflow",
		PostgresCheck: func(context.Context) error {
			return context.DeadlineExceeded
		},
		RedisCheck: func(context.Context) error {
			return nil
		},
	})
	metrics := observability.NewHTTPMetrics(nil)
	logger := slog.Default()
	router := newRouter(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "http://localhost:3000", metrics, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/readyz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestEventsStream(t *testing.T) {
	svc := platformhealth.NewService(platformhealth.Options{
		AppName: "sendflow",
		PostgresCheck: func(context.Context) error {
			return nil
		},
		RedisCheck: func(context.Context) error {
			return nil
		},
	})
	metrics := observability.NewHTTPMetrics(nil)
	logger := slog.Default()
	router := newRouter(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "http://localhost:3000", metrics, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		router.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for stream handler to stop")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), "event: connected\n") {
		t.Fatalf("expected connected event line, got %q", rec.Body.String())
	}
}

func TestCORSPreflightAllowsFrontendOrigin(t *testing.T) {
	svc := platformhealth.NewService(platformhealth.Options{
		AppName: "sendflow",
		PostgresCheck: func(context.Context) error {
			return nil
		},
		RedisCheck: func(context.Context) error {
			return nil
		},
	})
	metrics := observability.NewHTTPMetrics(nil)
	logger := slog.Default()
	router := newRouter(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "http://localhost:3000", metrics, logger)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/providers", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected frontend origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodGet) {
		t.Fatalf("expected allowed methods to include GET, got %q", got)
	}
}

func TestCORSDoesNotAllowUnknownOrigin(t *testing.T) {
	svc := platformhealth.NewService(platformhealth.Options{
		AppName: "sendflow",
		PostgresCheck: func(context.Context) error {
			return nil
		},
		RedisCheck: func(context.Context) error {
			return nil
		},
	})
	metrics := observability.NewHTTPMetrics(nil)
	logger := slog.Default()
	router := newRouter(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "http://localhost:3000", metrics, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allowed origin, got %q", got)
	}
}

func TestOpenAPIJSONReachable(t *testing.T) {
	router, _ := setupAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var spec map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}
	info, _ := spec["info"].(map[string]any)
	if got := info["title"]; got != "Sendflow API" {
		t.Fatalf("expected OpenAPI title, got %#v", got)
	}
}

func TestDocsNotExposed(t *testing.T) {
	router, _ := setupAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOpenAPIDocumentsRoute(t *testing.T) {
	router, _ := setupAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var spec struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}
	if _, ok := spec.Paths["/api/v1/auth/providers"]; !ok {
		t.Fatalf("expected /api/v1/auth/providers in OpenAPI paths")
	}
}

func TestOpenAPIDocumentsApplicationErrorCodes(t *testing.T) {
	router, _ := setupAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var spec struct {
		Paths map[string]struct {
			Post struct {
				Responses map[string]struct {
					Description string `json:"description"`
					Content     map[string]struct {
						Examples map[string]any `json:"examples"`
					} `json:"content"`
				} `json:"responses"`
			} `json:"post"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}
	login := spec.Paths["/api/v1/auth/login"].Post
	unauthorized := login.Responses["401"]
	if !strings.Contains(unauthorized.Description, "auth.invalid_credentials") {
		t.Fatalf("expected auth.invalid_credentials in 401 description, got %q", unauthorized.Description)
	}
	if _, ok := unauthorized.Content["application/json"].Examples["auth_invalid_credentials"]; !ok {
		t.Fatalf("expected auth_invalid_credentials example in 401 response")
	}
}
