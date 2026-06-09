package domain

import (
	"testing"
)

func TestRedactPayloadRedactsSensitiveKeys(t *testing.T) {
	payload := map[string]any{
		"email":    "user@example.com",
		"password": "supersecret",
		"token":    "my-token",
		"api_key":  "sk-12345",
	}
	result := RedactPayload(payload)

	if result["email"] != "user@example.com" {
		t.Errorf("expected email to be unchanged, got %v", result["email"])
	}
	if result["password"] != redactionPlaceholder {
		t.Errorf("expected password to be redacted, got %v", result["password"])
	}
	if result["token"] != redactionPlaceholder {
		t.Errorf("expected token to be redacted, got %v", result["token"])
	}
	if result["api_key"] != redactionPlaceholder {
		t.Errorf("expected api_key to be redacted, got %v", result["api_key"])
	}
}

func TestRedactPayloadNestedRedaction(t *testing.T) {
	payload := map[string]any{
		"name": "test",
		"nested": map[string]any{
			"secret":       "my-secret",
			"access_token": "eyJhbGci",
			"public_field": "visible",
			"deeply_nested": map[string]any{
				"private_key": "-----BEGIN PRIVATE KEY-----",
			},
		},
	}
	result := RedactPayload(payload)

	nested, ok := result["nested"].(map[string]any)
	if !ok {
		t.Fatal("expected nested to be a map")
	}
	if nested["secret"] != redactionPlaceholder {
		t.Errorf("expected nested secret to be redacted, got %v", nested["secret"])
	}
	if nested["access_token"] != redactionPlaceholder {
		t.Errorf("expected nested access_token to be redacted, got %v", nested["access_token"])
	}
	if nested["public_field"] != "visible" {
		t.Errorf("expected nested public_field to be unchanged, got %v", nested["public_field"])
	}
	deep, _ := nested["deeply_nested"].(map[string]any)
	if deep["private_key"] != redactionPlaceholder {
		t.Errorf("expected deeply nested private_key to be redacted, got %v", deep["private_key"])
	}
}

func TestRedactPayloadTruncatesLongStrings(t *testing.T) {
	longStr := ""
	for i := 0; i < 600; i++ {
		longStr += "a"
	}
	payload := map[string]any{
		"long_field": longStr,
	}
	result := RedactPayload(payload)

	val, ok := result["long_field"].(string)
	if !ok {
		t.Fatal("expected long_field to be a string")
	}
	if len(val) > maxStringLength+3 {
		t.Errorf("expected truncated string, got length %d", len(val))
	}
}

func TestRedactPayloadTruncatesLargeArrays(t *testing.T) {
	arr := make([]any, maxArrayItems+10)
	for i := range arr {
		arr[i] = i
	}
	payload := map[string]any{
		"items": arr,
	}
	result := RedactPayload(payload)

	items, ok := result["items"].([]any)
	if !ok {
		t.Fatal("expected items to be an array")
	}
	if len(items) > maxArrayItems {
		t.Errorf("expected truncated array, got length %d", len(items))
	}
}

func TestRedactPayloadTruncatesLargeMaps(t *testing.T) {
	largeMap := make(map[string]any)
	for i := 0; i < maxMapKeys+10; i++ {
		largeMap[string(rune('a'+i%26))+string(rune('0'+i/10))] = i
	}
	payload := map[string]any{
		"large": largeMap,
	}
	result := RedactPayload(payload)

	rl, ok := result["large"].(map[string]any)
	if !ok {
		t.Fatal("expected large to be a map")
	}
	if len(rl) > maxMapKeys+1 {
		t.Errorf("expected truncated map, got length %d", len(rl))
	}
}

func TestRedactPayloadNil(t *testing.T) {
	result := RedactPayload(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}
