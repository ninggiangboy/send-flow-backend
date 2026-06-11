package httpjson

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteSetsContentType(t *testing.T) {
	rec := httptest.NewRecorder()

	err := Write(rec, http.StatusOK, map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json, got %q", got)
	}
}

func TestWriteSetsStatusCode(t *testing.T) {
	rec := httptest.NewRecorder()

	err := Write(rec, http.StatusCreated, map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestWriteEncodesPayload(t *testing.T) {
	rec := httptest.NewRecorder()

	payload := map[string]string{"hello": "world"}
	err := Write(rec, http.StatusOK, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{"hello":"world"}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestWriteReturnsErrorOnUnencodablePayload(t *testing.T) {
	rec := httptest.NewRecorder()

	err := Write(rec, http.StatusOK, make(chan int))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
