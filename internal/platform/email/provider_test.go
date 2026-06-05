package email

import (
	"context"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

func TestNewSender_SMTP(t *testing.T) {
	sender, err := NewSender(context.Background(), config.Config{
		EmailProvider: "smtp",
		SMTP: config.SMTPConfig{
			Host: "localhost",
			Port: 1025,
			From: "noreply@sendflow.local",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := sender.(*SMTPSender); !ok {
		t.Fatalf("expected SMTPSender, got %T", sender)
	}
}

func TestNewSender_Unsupported(t *testing.T) {
	_, err := NewSender(context.Background(), config.Config{EmailProvider: "unknown"})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}
