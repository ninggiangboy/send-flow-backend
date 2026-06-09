package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	audienceapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app"
	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
)

func setupFullAPIRouter(t *testing.T) http.Handler {
	t.Helper()
	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:       "sendflow",
		PostgresCheck: func(ctx context.Context) error { return nil },
		RedisCheck:    func(ctx context.Context) error { return nil },
	})
	audienceSvc := audienceapp.NewService(audienceapp.Options{
		Logger: slog.Default(),
	})
	contentSvc := contentapp.NewService(contentapp.Options{
		Logger: slog.Default(),
	})
	suppressionSvc := suppressionapp.NewService(suppressionapp.Options{
		Logger: slog.Default(),
	})
	return newRouter(healthSvc, nil, nil, audienceSvc, contentSvc, suppressionSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "http://localhost:3000", observability.NewHTTPMetrics(nil), slog.Default())
}

func TestOpenAPIDocumentsContactRoutes(t *testing.T) {
	router := setupFullAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var spec struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}

	expectedPaths := []string{
		"/api/v1/workspaces/{workspace_id}/contacts",
		"/api/v1/workspaces/{workspace_id}/lists",
		"/api/v1/workspaces/{workspace_id}/lists/{list_id}/contacts",
		"/api/v1/workspaces/{workspace_id}/segments",
		"/api/v1/workspaces/{workspace_id}/audience/imports",
		"/api/v1/workspaces/{workspace_id}/audience/exports",
	}

	for _, path := range expectedPaths {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("expected path %q in OpenAPI spec", path)
		}
	}
}

func TestOpenAPIDocumentsTemplateRoutes(t *testing.T) {
	router := setupFullAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var spec struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}

	expectedPaths := []string{
		"/api/v1/workspaces/{workspace_id}/templates",
		"/api/v1/workspaces/{workspace_id}/templates/{template_id}",
		"/api/v1/workspaces/{workspace_id}/templates/{template_id}/publish",
		"/api/v1/workspaces/{workspace_id}/templates/{template_id}/versions",
		"/api/v1/workspaces/{workspace_id}/templates/{template_id}/preview",
		"/api/v1/workspaces/{workspace_id}/render",
	}

	for _, path := range expectedPaths {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("expected path %q in OpenAPI spec", path)
		}
	}
}

func TestOpenAPIDocumentsSuppressionRoutes(t *testing.T) {
	router := setupFullAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var spec struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}

	if _, ok := spec.Paths["/api/v1/workspaces/{workspace_id}/suppression"]; !ok {
		t.Errorf("expected path %q in OpenAPI spec", "/api/v1/workspaces/{workspace_id}/suppression")
	}
}

func TestOpenAPIProtectedRoutesHaveBearerAuth(t *testing.T) {
	router := setupFullAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var spec struct {
		Paths map[string]struct {
			Get    *humaOp `json:"get"`
			Post   *humaOp `json:"post"`
			Put    *humaOp `json:"put"`
			Patch  *humaOp `json:"patch"`
			Delete *humaOp `json:"delete"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}

	routes := []string{
		"/api/v1/workspaces/{workspace_id}/contacts",
		"/api/v1/workspaces/{workspace_id}/templates",
		"/api/v1/workspaces/{workspace_id}/suppression",
	}

	for _, route := range routes {
		item := spec.Paths[route]
		var ops []*humaOp
		if item.Get != nil {
			ops = append(ops, item.Get)
		}
		if item.Post != nil {
			ops = append(ops, item.Post)
		}
		if item.Put != nil {
			ops = append(ops, item.Put)
		}
		if item.Patch != nil {
			ops = append(ops, item.Patch)
		}
		if item.Delete != nil {
			ops = append(ops, item.Delete)
		}
		for _, op := range ops {
			if len(op.Security) == 0 {
				t.Errorf("expected route %q to have bearer auth", route)
				break
			}
		}
	}
}

type humaOp struct {
	Security []map[string][]string `json:"security"`
}

func TestOpenAPIDocumentsAudienceErrorCodes(t *testing.T) {
	router := setupFullAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()

	expectedCodes := []string{
		"audience.read_denied",
		"audience.write_denied",
		"audience.contact_not_found",
		"audience.contact_email_conflict",
		"audience.contact_payload_invalid",
	}

	for _, code := range expectedCodes {
		if !strings.Contains(body, code) {
			t.Errorf("expected error code %q in OpenAPI spec", code)
		}
	}
}

func TestOpenAPIDocumentsTemplateErrorCodes(t *testing.T) {
	router := setupFullAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()

	expectedCodes := []string{
		"template.read_denied",
		"template.write_denied",
		"template.not_found",
		"template.source_invalid",
		"template.render_payload_invalid",
	}

	for _, code := range expectedCodes {
		if !strings.Contains(body, code) {
			t.Errorf("expected error code %q in OpenAPI spec", code)
		}
	}
}

func TestOpenAPIDocumentsSuppressionErrorCodes(t *testing.T) {
	router := setupFullAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()

	expectedCodes := []string{
		"suppression.read_denied",
		"suppression.manage_denied",
		"suppression.entry_not_found",
		"suppression.unsuppress_conflict",
		"suppression.scope_invalid",
	}

	for _, code := range expectedCodes {
		if !strings.Contains(body, code) {
			t.Errorf("expected error code %q in OpenAPI spec", code)
		}
	}
}
