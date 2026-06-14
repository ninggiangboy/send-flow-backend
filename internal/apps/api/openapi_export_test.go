package api

import (
	"strings"
	"testing"
)

func TestGenerateOpenAPIYAMLIncludesFullRouteSurface(t *testing.T) {
	spec, err := GenerateOpenAPIYAML()
	if err != nil {
		t.Fatalf("generate openapi: %v", err)
	}
	body := string(spec)
	for _, expected := range []string{
		"/api/v1/auth/signup:",
		"/api/v1/transactional/send:",
		"/api/v1/webhooks/providers/{provider}:",
		"/o/{tracking_id}:",
		"bearerAuth:",
		"apiKeyAuth:",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected generated OpenAPI to contain %q", expected)
		}
	}
}
