package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type noFlushWriter struct {
	header http.Header
	body   strings.Builder
	code   int
}

func (w *noFlushWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *noFlushWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

func (w *noFlushWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func TestNewSetsHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/events/stream", nil)
	rec := httptest.NewRecorder()

	stream, err := New(rec, req, Options{HeartbeatInterval: -1})
	if err != nil {
		t.Fatalf("expected stream, got error: %v", err)
	}
	defer stream.Close()

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("expected no-cache, got %q", got)
	}
	if got := rec.Header().Get("Connection"); got != "keep-alive" {
		t.Fatalf("expected keep-alive, got %q", got)
	}
}

func TestWriteEventFormatsMultilineData(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/events/stream", nil)
	rec := httptest.NewRecorder()

	stream, err := New(rec, req, Options{HeartbeatInterval: -1})
	if err != nil {
		t.Fatalf("expected stream, got error: %v", err)
	}
	defer stream.Close()

	err = stream.WriteEvent(Event{
		Event: "tick",
		ID:    "1",
		Retry: 3 * time.Second,
		Data:  "line-1\nline-2",
	})
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: tick\n") {
		t.Fatalf("missing event line: %q", body)
	}
	if !strings.Contains(body, "id: 1\n") {
		t.Fatalf("missing id line: %q", body)
	}
	if !strings.Contains(body, "retry: 3000\n") {
		t.Fatalf("missing retry line: %q", body)
	}
	if !strings.Contains(body, "data: line-1\ndata: line-2\n\n") {
		t.Fatalf("missing multiline data framing: %q", body)
	}
}

func TestNewReturnsErrorWhenFlusherMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/events/stream", nil)
	w := &noFlushWriter{}

	_, err := New(w, req, Options{})
	if err == nil {
		t.Fatal("expected error for missing flusher")
	}
	if err != ErrStreamingUnsupported {
		t.Fatalf("expected ErrStreamingUnsupported, got %v", err)
	}
}

func TestHeartbeatWritesComment(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/events/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	stream, err := New(rec, req, Options{HeartbeatInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("expected stream, got error: %v", err)
	}

	time.Sleep(25 * time.Millisecond)
	stream.Close()
	cancel()

	if !strings.Contains(rec.Body.String(), ": keepalive\n\n") {
		t.Fatalf("expected keepalive comment in response body, got %q", rec.Body.String())
	}
}
