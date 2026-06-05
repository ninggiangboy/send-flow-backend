package featureflags

import (
	"context"
	"testing"
)

func TestStaticClient_EnabledUsesTenantOverride(t *testing.T) {
	client := NewStatic(false, map[string]bool{
		"campaigns": false,
	}).WithTenantFlag("tenant_123", "campaigns", true)

	if !client.Enabled(context.Background(), "campaigns", EvaluationContext{TenantID: "tenant_123"}) {
		t.Fatal("expected tenant override to enable flag")
	}
	if client.Enabled(context.Background(), "campaigns", EvaluationContext{TenantID: "tenant_456"}) {
		t.Fatal("expected global flag to remain disabled")
	}
}
