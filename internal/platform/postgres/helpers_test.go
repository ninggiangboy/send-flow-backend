package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type mockTx struct {
	pgx.Tx
}

func TestNullable_EmptyString_ReturnsNil(t *testing.T) {
	got := Nullable("")
	if got != nil {
		t.Fatalf("expected nil, got %v", *got)
	}
}

func TestNullable_NonEmptyString_ReturnsPointer(t *testing.T) {
	v := "hello"
	got := Nullable(v)
	if got == nil {
		t.Fatalf("expected pointer, got nil")
	}
	if *got != v {
		t.Fatalf("expected %q, got %q", v, *got)
	}
}

func TestNullableTime_ZeroTime_ReturnsNil(t *testing.T) {
	got := NullableTime(time.Time{})
	if got != nil {
		t.Fatalf("expected nil, got %v", *got)
	}
}

func TestNullableTime_NonZeroTime_ReturnsPointer(t *testing.T) {
	v := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	got := NullableTime(v)
	if got == nil {
		t.Fatalf("expected pointer, got nil")
	}
	if !got.Equal(v) {
		t.Fatalf("expected %v, got %v", v, got)
	}
}

func TestIsUniqueViolation_PgErrorWithCode23505_ReturnsTrue(t *testing.T) {
	err := &pgconn.PgError{Code: "23505"}
	if !IsUniqueViolation(err) {
		t.Fatalf("expected true for code 23505")
	}
}

func TestIsUniqueViolation_PgErrorWithOtherCode_ReturnsFalse(t *testing.T) {
	err := &pgconn.PgError{Code: "42P01"}
	if IsUniqueViolation(err) {
		t.Fatalf("expected false for code %q", err.Code)
	}
}

func TestIsUniqueViolation_NonPgconnError_ReturnsFalse(t *testing.T) {
	err := errors.New("some other error")
	if IsUniqueViolation(err) {
		t.Fatalf("expected false for non-pgconn error")
	}
}

func TestIsUniqueViolation_NilError_ReturnsFalse(t *testing.T) {
	if IsUniqueViolation(nil) {
		t.Fatalf("expected false for nil error")
	}
}

func TestItoa_Zero_ReturnsZero(t *testing.T) {
	got := Itoa(0)
	if got != "0" {
		t.Fatalf("expected %q, got %q", "0", got)
	}
}

func TestItoa_PositiveInt_ReturnsCorrectString(t *testing.T) {
	got := Itoa(12345)
	if got != "12345" {
		t.Fatalf("expected %q, got %q", "12345", got)
	}
}

func TestItoa_NegativeInt_ReturnsCorrectString(t *testing.T) {
	got := Itoa(-42)
	if got != "-42" {
		t.Fatalf("expected %q, got %q", "-42", got)
	}
}

func TestItoa_LargeInt_ReturnsCorrectString(t *testing.T) {
	got := Itoa(1234567890)
	if got != "1234567890" {
		t.Fatalf("expected %q, got %q", "1234567890", got)
	}
}

func TestTxFromCtx_NoTxInContext_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	tx := TxFromCtx(ctx)
	if tx != nil {
		t.Fatalf("expected nil, got %v", tx)
	}
}

func TestContextWithTx_TxFromCtxRoundTrip(t *testing.T) {
	ctx := context.Background()
	mock := &mockTx{}
	ctx = ContextWithTx(ctx, mock)
	got := TxFromCtx(ctx)
	if got == nil {
		t.Fatalf("expected tx, got nil")
	}
	if got != mock {
		t.Fatalf("expected same tx instance")
	}
}

func TestTxFromCtx_DifferentKeyDoesNotConflict(t *testing.T) {
	type differentKey struct{}
	ctx := context.WithValue(context.Background(), differentKey{}, "some value")
	tx := TxFromCtx(ctx)
	if tx != nil {
		t.Fatalf("expected nil, got %v", tx)
	}
}

func TestGetDB_WithTxInContext_ReturnsTx(t *testing.T) {
	ctx := context.Background()
	mock := &mockTx{}
	ctx = ContextWithTx(ctx, mock)
	got := GetDB(ctx, nil)
	if got == nil {
		t.Fatalf("expected non-nil DBTX, got nil")
	}
	if got != mock {
		t.Fatalf("expected GetDB to return the tx from context")
	}
}
