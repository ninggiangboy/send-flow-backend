package httputil

import "testing"

func TestExtractBearerToken_Valid(t *testing.T) {
	token, ok := ExtractBearerToken("Bearer sk_test_token")
	if !ok {
		t.Fatal("expected successful extraction")
	}
	if token != "sk_test_token" {
		t.Errorf("expected %q, got %q", "sk_test_token", token)
	}
}

func TestExtractBearerToken_Missing(t *testing.T) {
	_, ok := ExtractBearerToken("")
	if ok {
		t.Fatal("expected failure for empty header")
	}
}

func TestExtractBearerToken_NoBearer(t *testing.T) {
	_, ok := ExtractBearerToken("Basic dGVzdDp0ZXN0")
	if ok {
		t.Fatal("expected failure for non-bearer auth")
	}
}

func TestExtractBearerToken_EmptyToken(t *testing.T) {
	_, ok := ExtractBearerToken("Bearer ")
	if ok {
		t.Fatal("expected failure for empty token")
	}
}
