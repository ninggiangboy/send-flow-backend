package domain

import (
	"context"
	"net"
	"testing"
)

func TestValidateWebhookConfig_Valid(t *testing.T) {
	cfg := WebhookConfig{
		WorkspaceID:   "ws-1",
		Name:          "My Webhook",
		TargetURL:     "https://example.com/hooks",
		Subscriptions: []string{"delivery.message.delivered.v1"},
	}
	if err := ValidateWebhookConfig(cfg); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateWebhookConfig_MissingWorkspaceID(t *testing.T) {
	cfg := WebhookConfig{
		Name:          "My Webhook",
		TargetURL:     "https://example.com/hooks",
		Subscriptions: []string{"delivery.message.delivered.v1"},
	}
	if err := ValidateWebhookConfig(cfg); err != ErrConfigInvalid {
		t.Errorf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestValidateWebhookConfig_EmptyName(t *testing.T) {
	cfg := WebhookConfig{
		WorkspaceID:   "ws-1",
		TargetURL:     "https://example.com/hooks",
		Subscriptions: []string{"delivery.message.delivered.v1"},
	}
	if err := ValidateWebhookConfig(cfg); err != ErrConfigInvalid {
		t.Errorf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestValidateTargetURL_Invalid(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://example.com/hooks", true},
		{"http://10.0.0.1/hooks", false},
		{"http://10.0.0.1:8080/hooks", false},
		{"http://192.168.1.1/hooks", false},
		{"http://192.168.1.1:8080/hooks", false},
		{"http://127.0.0.1/hooks", false},
		{"http://127.0.0.1:8080/hooks", false},
		{"http://localhost/hooks", false},
		{"http://localhost:8080/hooks", false},
		{"http://[::1]/hooks", false},
		{"http://[::1]:8080/hooks", false},
		{"http://0.0.0.0/hooks", false},
		{"http://172.16.0.1/hooks", false},
		{"http://169.254.1.1/hooks", false},
		{"ftp://example.com/hooks", false},
		{"", false},
		{"not-a-url", false},
		{"https://user:pass@example.com/hooks", false},
	}
	for _, tt := range tests {
		err := ValidateTargetURL(tt.url)
		if tt.valid && err != nil {
			t.Errorf("expected %q to be valid, got %v", tt.url, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("expected %q to be invalid, but it passed", tt.url)
		}
	}
}

func TestValidateSubscriptions_Valid(t *testing.T) {
	subs := []string{"delivery.message.queued.v1", "tracking.email_opened.v1"}
	if err := ValidateSubscriptions(subs); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateSubscriptions_Empty(t *testing.T) {
	if err := ValidateSubscriptions(nil); err != ErrSubscriptionInvalid {
		t.Errorf("expected ErrSubscriptionInvalid, got %v", err)
	}
}

func TestValidateSubscriptions_Invalid(t *testing.T) {
	subs := []string{"unknown.event.v1"}
	if err := ValidateSubscriptions(subs); err != ErrSubscriptionInvalid {
		t.Errorf("expected ErrSubscriptionInvalid, got %v", err)
	}
}

func TestCanDeliveryBeRetried(t *testing.T) {
	if !CanDeliveryBeRetried(DeliveryStatusFailed) {
		t.Error("expected failed delivery to be retryable")
	}
	if CanDeliveryBeRetried(DeliveryStatusSucceeded) {
		t.Error("expected succeeded delivery to not be retryable")
	}
	if CanDeliveryBeRetried(DeliveryStatusPending) {
		t.Error("expected pending delivery to not be retryable")
	}
	if CanDeliveryBeRetried(DeliveryStatusDelivering) {
		t.Error("expected delivering delivery to not be retryable")
	}
}

func TestIsTerminalDeliveryStatus(t *testing.T) {
	if !IsTerminalDeliveryStatus(DeliveryStatusSucceeded) {
		t.Error("expected succeeded to be terminal")
	}
	if !IsTerminalDeliveryStatus(DeliveryStatusFailed) {
		t.Error("expected failed to be terminal")
	}
	if IsTerminalDeliveryStatus(DeliveryStatusPending) {
		t.Error("expected pending to not be terminal")
	}
}

func TestSanitizeError_Short(t *testing.T) {
	short := "short error"
	if got := SanitizeError(short); got != short {
		t.Errorf("expected %q, got %q", short, got)
	}
}

func TestSanitizeError_Long(t *testing.T) {
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}
	s := string(long)
	got := SanitizeError(s)
	if len(got) > 500 {
		t.Errorf("expected max 500 chars, got %d", len(got))
	}
}

func TestSanitizeHeaders(t *testing.T) {
	headers := map[string]string{
		"content-type":  "application/json",
		"authorization": "Bearer secret-token",
		"cookie":        "session=abc123",
		"x-custom":      "custom-value",
	}
	sanitized := SanitizeHeaders(headers)
	if _, ok := sanitized["authorization"]; ok {
		t.Error("authorization header should be removed")
	}
	if _, ok := sanitized["cookie"]; ok {
		t.Error("cookie header should be removed")
	}
	if sanitized["content-type"] != "application/json" {
		t.Error("content-type header should be preserved")
	}
	if sanitized["x-custom"] != "custom-value" {
		t.Error("x-custom header should be preserved")
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"127.0.0.0", true},
		{"169.254.1.1", true},
		{"0.0.0.0", true},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"8.8.8.8", false},
		{"93.184.216.34", false},
		{"2001:4860:4860::8888", false},
		{"", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		got := IsPrivateIP(ip)
		if got != tt.private {
			t.Errorf("IsPrivateIP(%q) = %v, want %v", tt.ip, got, tt.private)
		}
	}
}

func TestValidateTargetURL_ResolvedPrivateIP(t *testing.T) {
	original := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}}, nil
	}
	defer func() { lookupIPAddr = original }()
	err := ValidateTargetURL("https://public-dns-name.example.com/hooks")
	if err != ErrTargetURLInvalid {
		t.Errorf("expected ErrTargetURLInvalid for host resolving to private IP, got %v", err)
	}
}

func TestValidateTargetURL_ResolvedPublicIP(t *testing.T) {
	original := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	defer func() { lookupIPAddr = original }()
	err := ValidateTargetURL("https://public-dns-name.example.com/hooks")
	if err != nil {
		t.Errorf("expected no error for host resolving to public IP, got %v", err)
	}
}

func TestValidateTargetURL_ResolutionFailure(t *testing.T) {
	original := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return nil, &net.DNSError{IsNotFound: true}
	}
	defer func() { lookupIPAddr = original }()
	err := ValidateTargetURL("https://unresolvable.example.com/hooks")
	if err != nil {
		t.Errorf("expected no error when DNS resolution fails, got %v", err)
	}
}

func TestValidateTargetURL_PublicIPLiteralPasses(t *testing.T) {
	err := ValidateTargetURL("https://8.8.8.8/hooks")
	if err != nil {
		t.Errorf("expected no error for public literal IP, got %v", err)
	}
}

func TestValidateTargetURL_PrivateIPLiteralBlocked(t *testing.T) {
	err := ValidateTargetURL("https://192.168.1.1/hooks")
	if err != ErrTargetURLInvalid {
		t.Errorf("expected ErrTargetURLInvalid for private literal IP, got %v", err)
	}
}

func TestValidateTargetURL_IPLiteralSkipsLookup(t *testing.T) {
	called := false
	original := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		called = true
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	defer func() { lookupIPAddr = original }()
	_ = ValidateTargetURL("https://8.8.8.8/hooks")
	if called {
		t.Error("expected DNS lookup to be skipped for IP literal")
	}
}
