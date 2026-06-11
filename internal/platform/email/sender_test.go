package email

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

func TestMessageValidate_EmptyRecipients(t *testing.T) {
	msg := Message{Subject: "test", Text: "body"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for empty recipients")
	}
	if !strings.Contains(err.Error(), "recipient") {
		t.Fatalf("expected recipient error, got: %v", err)
	}
}

func TestMessageValidate_EmptySubject(t *testing.T) {
	msg := Message{To: []string{"a@b.com"}, Text: "body"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for empty subject")
	}
	if !strings.Contains(err.Error(), "subject") {
		t.Fatalf("expected subject error, got: %v", err)
	}
}

func TestMessageValidate_EmptyBody(t *testing.T) {
	msg := Message{To: []string{"a@b.com"}, Subject: "test"}
	err := msg.Validate()
	if err == nil {
		t.Fatal("expected error for empty body")
	}
	if !strings.Contains(err.Error(), "body") {
		t.Fatalf("expected body error, got: %v", err)
	}
}

func TestMessageValidate_TextOnly(t *testing.T) {
	msg := Message{To: []string{"a@b.com"}, Subject: "test", Text: "hello"}
	if err := msg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMessageValidate_HTMLOnly(t *testing.T) {
	msg := Message{To: []string{"a@b.com"}, Subject: "test", HTML: "<p>hello</p>"}
	if err := msg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMessageValidate_Both(t *testing.T) {
	msg := Message{To: []string{"a@b.com"}, Subject: "test", Text: "hello", HTML: "<p>hello</p>"}
	if err := msg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildMIMEMessage_TextOnly(t *testing.T) {
	msg := Message{To: []string{"a@b.com"}, Subject: "test", Text: "hello"}
	data, err := buildMIMEMessage("from@a.com", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(data, []byte("Content-Type: text/plain")) {
		t.Fatal("expected text/plain content type")
	}
	if bytes.Contains(data, []byte("Content-Type: text/html")) {
		t.Fatal("unexpected text/html content type")
	}
	if bytes.Contains(data, []byte("boundary")) {
		t.Fatal("unexpected boundary for text-only message")
	}
}

func TestBuildMIMEMessage_HTMLOnly(t *testing.T) {
	msg := Message{To: []string{"a@b.com"}, Subject: "test", HTML: "<p>hello</p>"}
	data, err := buildMIMEMessage("from@a.com", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(data, []byte("Content-Type: text/html")) {
		t.Fatal("expected text/html content type")
	}
	if bytes.Contains(data, []byte("Content-Type: text/plain")) {
		t.Fatal("unexpected text/plain content type")
	}
	if bytes.Contains(data, []byte("boundary")) {
		t.Fatal("unexpected boundary for html-only message")
	}
}

func TestBuildMIMEMessage_Multipart(t *testing.T) {
	msg := Message{To: []string{"a@b.com"}, Subject: "test", Text: "hello", HTML: "<p>hello</p>"}
	data, err := buildMIMEMessage("from@a.com", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(data, []byte("Content-Type: text/plain")) {
		t.Fatal("expected text/plain content type")
	}
	if !bytes.Contains(data, []byte("Content-Type: text/html")) {
		t.Fatal("expected text/html content type")
	}
	if !bytes.Contains(data, []byte("boundary")) {
		t.Fatal("expected boundary for multipart message")
	}
}

func TestBuildMIMEMessage_Headers(t *testing.T) {
	msg := Message{To: []string{"a@b.com"}, Subject: "test", Text: "hello"}
	data, err := buildMIMEMessage("from@a.com", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(data, []byte("From: from@a.com")) {
		t.Fatal("expected From header")
	}
	if !bytes.Contains(data, []byte("To: a@b.com")) {
		t.Fatal("expected To header")
	}
	if !bytes.Contains(data, []byte("Subject: test")) {
		t.Fatal("expected Subject header")
	}
	if !bytes.Contains(data, []byte("MIME-Version: 1.0")) {
		t.Fatal("expected MIME-Version header")
	}
}

func TestNewSMTPSender_WithAuth(t *testing.T) {
	cfg := config.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		From:     "noreply@example.com",
		Username: "user",
		Password: "pass",
	}
	s := NewSMTPSender(cfg)
	if s.auth == nil {
		t.Fatal("expected non-nil auth")
	}
	typeName := fmt.Sprintf("%T", s.auth)
	if !strings.Contains(typeName, "plainAuth") {
		t.Fatalf("expected plainAuth type, got %T", s.auth)
	}
}

func TestNewSMTPSender_NoAuth(t *testing.T) {
	cfg := config.SMTPConfig{
		Host: "smtp.example.com",
		Port: 25,
		From: "noreply@example.com",
	}
	s := NewSMTPSender(cfg)
	if s.auth != nil {
		t.Fatal("expected nil auth")
	}
}
