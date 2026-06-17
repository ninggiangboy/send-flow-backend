package redis

import (
	"errors"
	"testing"
)

func TestIsCacheMiss(t *testing.T) {
	if !IsCacheMiss(ErrCacheMiss) {
		t.Fatal("expected true for ErrCacheMiss")
	}
	if IsCacheMiss(errors.New("other error")) {
		t.Fatal("expected false for other error")
	}
	if IsCacheMiss(nil) {
		t.Fatal("expected false for nil")
	}
}

func TestIsDecodeError(t *testing.T) {
	de := ErrDecodeError
	if !IsDecodeError(de) {
		t.Fatal("expected true for ErrDecodeError")
	}
	if IsDecodeError(errors.New("other error")) {
		t.Fatal("expected false for other error")
	}
	if IsDecodeError(nil) {
		t.Fatal("expected false for nil")
	}
}

func TestErrDecodeErrorWrapping(t *testing.T) {
	wrapped := errors.New("redis: decode error: some cause")
	if errors.Is(wrapped, ErrDecodeError) {
		t.Fatal("wrapped error without %w should not match")
	}
	proper := errors.New("inner")
	joined := errors.Join(ErrDecodeError, proper)
	if !errors.Is(joined, ErrDecodeError) {
		t.Fatal("expected errors.Is to match ErrDecodeError")
	}
}
