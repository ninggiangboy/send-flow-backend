package token

import (
	"testing"
	"time"
)

func TestJWTManagerIssueAndParseAccess(t *testing.T) {
	m := NewJWTManager("send-flow", "access-secret", "refresh-secret", time.Minute, time.Hour)
	tokens, _, _, err := m.Issue("user_1", "sess_1", time.Now().UTC())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := m.ParseAccess(tokens.AccessToken)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.Subject != "user_1" {
		t.Fatalf("expected subject user_1, got %s", claims.Subject)
	}
}
