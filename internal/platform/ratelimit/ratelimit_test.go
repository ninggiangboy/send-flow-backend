package ratelimit

import (
	"context"
	"testing"
	"time"
)

type mockClient struct {
	result any
	err    error
}

func (m *mockClient) Eval(_ context.Context, _ string, _ []string, _ ...any) (any, error) {
	return m.result, m.err
}

func TestAllowLimitZero(t *testing.T) {
	s := NewRedisService(&mockClient{})
	ok, err := s.Allow(context.Background(), "test", 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true for limit == 0")
	}
}

func TestAllowLimitNegative(t *testing.T) {
	s := NewRedisService(&mockClient{})
	ok, err := s.Allow(context.Background(), "test", -1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true for limit < 0")
	}
}

func TestAllowUnderLimit(t *testing.T) {
	s := NewRedisService(&mockClient{result: int64(5)})
	ok, err := s.Allow(context.Background(), "test", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true when count < limit")
	}
}

func TestAllowAtLimit(t *testing.T) {
	s := NewRedisService(&mockClient{result: int64(10)})
	ok, err := s.Allow(context.Background(), "test", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true when count == limit")
	}
}

func TestAllowOverLimit(t *testing.T) {
	s := NewRedisService(&mockClient{result: int64(11)})
	ok, err := s.Allow(context.Background(), "test", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false when count > limit")
	}
}

func TestAllowError(t *testing.T) {
	s := NewRedisService(&mockClient{err: context.DeadlineExceeded})
	ok, err := s.Allow(context.Background(), "test", 10, time.Minute)
	if err == nil {
		t.Fatal("expected error")
	}
	if ok {
		t.Error("expected false on error")
	}
}
