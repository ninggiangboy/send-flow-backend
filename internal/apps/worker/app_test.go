package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
)

func TestRuntimeRoutesUseAPIPrefix(t *testing.T) {
	svc := platformhealth.NewService(platformhealth.Options{
		AppName: "sendflow-worker",
		PostgresCheck: func(context.Context) error {
			return nil
		},
		RedisCheck: func(context.Context) error {
			return nil
		},
	})
	router := newRouter(svc, observability.NewHTTPMetrics(nil))

	for _, path := range []string{"/api/healthz", "/api/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected %s to return 200, got %d", path, rec.Code)
		}
	}
}
