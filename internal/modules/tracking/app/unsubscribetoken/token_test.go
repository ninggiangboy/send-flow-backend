package unsubscribetoken

import (
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	signer := NewSigner("test-secret-12345")
	payload := Payload{
		WorkspaceID:              "ws_1",
		MessageID:                "msg_1",
		RecipientEmailNormalized: "user@example.com",
		Scope:                    "workspace",
		ExpiresAt:                time.Now().Add(24 * time.Hour).Unix(),
	}

	token, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	verified, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if verified.WorkspaceID != payload.WorkspaceID {
		t.Errorf("expected workspace_id %q, got %q", payload.WorkspaceID, verified.WorkspaceID)
	}
	if verified.MessageID != payload.MessageID {
		t.Errorf("expected message_id %q, got %q", payload.MessageID, verified.MessageID)
	}
	if verified.RecipientEmailNormalized != payload.RecipientEmailNormalized {
		t.Errorf("expected recipient_email %q, got %q", payload.RecipientEmailNormalized, verified.RecipientEmailNormalized)
	}
}

func TestTamperedTokenFails(t *testing.T) {
	signer := NewSigner("test-secret-12345")
	payload := Payload{
		WorkspaceID:              "ws_1",
		MessageID:                "msg_1",
		RecipientEmailNormalized: "user@example.com",
		Scope:                    "workspace",
	}

	token, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}

	tampered := token + "x"
	if _, err := signer.Verify(tampered); err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for tampered token, got %v", err)
	}
}

func TestExpiredTokenFails(t *testing.T) {
	signer := NewSigner("test-secret-12345")
	payload := Payload{
		WorkspaceID:              "ws_1",
		MessageID:                "msg_1",
		RecipientEmailNormalized: "user@example.com",
		Scope:                    "workspace",
		ExpiresAt:                time.Now().Add(-1 * time.Hour).Unix(),
	}

	token, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := signer.Verify(token); err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestDifferentSecretFails(t *testing.T) {
	signer := NewSigner("test-secret-12345")
	payload := Payload{
		WorkspaceID:              "ws_1",
		MessageID:                "msg_1",
		RecipientEmailNormalized: "user@example.com",
		Scope:                    "workspace",
	}

	token, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}

	otherSigner := NewSigner("different-secret")
	if _, err := otherSigner.Verify(token); err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for different secret, got %v", err)
	}
}

func TestNoExpirySucceeds(t *testing.T) {
	signer := NewSigner("test-secret-12345")
	payload := Payload{
		WorkspaceID:              "ws_1",
		MessageID:                "msg_1",
		RecipientEmailNormalized: "user@example.com",
		Scope:                    "workspace",
	}

	token, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}

	verified, err := signer.Verify(token)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if verified.WorkspaceID != "ws_1" {
		t.Errorf("expected workspace_id ws_1, got %q", verified.WorkspaceID)
	}
}
