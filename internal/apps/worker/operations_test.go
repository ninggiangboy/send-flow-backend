package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRow struct{}

func (r *fakeRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if b, ok := dest[0].(*bool); ok {
			*b = false
		}
	}
	return nil
}

type fakeExecer struct {
	err     error
	queries []string
	args    [][]any
}

func (f *fakeExecer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	f.queries = append(f.queries, query)
	f.args = append(f.args, args)
	return pgconn.CommandTag{}, f.err
}

func (f *fakeExecer) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &fakeRow{}
}

func TestProcessedEventMarkers_MarkProcessed(t *testing.T) {
	execer := &fakeExecer{}
	markers := NewProcessedEventMarkersWithExecer(execer)

	inserted, err := markers.MarkProcessed(context.Background(), "consumer-a", "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inserted {
		t.Fatal("expected marker insert")
	}
}

func TestProcessedEventMarkers_DuplicateReturnsFalse(t *testing.T) {
	execer := &fakeExecer{err: &pgconn.PgError{Code: "23505"}}
	markers := NewProcessedEventMarkersWithExecer(execer)

	inserted, err := markers.MarkProcessed(context.Background(), "consumer-a", "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate marker to return false")
	}
}

func TestProcessedEventMarkers_PropagatesErrors(t *testing.T) {
	want := errors.New("database unavailable")
	execer := &fakeExecer{err: want}
	markers := NewProcessedEventMarkersWithExecer(execer)

	_, err := markers.MarkProcessed(context.Background(), "consumer-a", "00000000-0000-0000-0000-000000000001")
	if !errors.Is(err, want) {
		t.Fatalf("expected propagated error, got %v", err)
	}
}

func TestDeadLetterRepository_Save(t *testing.T) {
	execer := &fakeExecer{}
	repo := NewDeadLetterRepositoryWithExecer(execer)

	err := repo.Save(context.Background(), DeadLetterRecord{
		ID:           "00000000-0000-0000-0000-000000000010",
		Source:       "delivery.consumer",
		EventID:      "00000000-0000-0000-0000-000000000001",
		Payload:      map[string]string{"event_type": "delivery.message.queued.v1"},
		ErrorMessage: "provider unavailable",
		Retryable:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(execer.args) != 1 {
		t.Fatalf("expected one insert, got %d", len(execer.args))
	}
	if retryable, ok := execer.args[0][6].(bool); !ok || !retryable {
		t.Fatalf("expected retryable flag in args, got %#v", execer.args[0])
	}
}

func TestDeadLetterRepository_SaveNonRetryable(t *testing.T) {
	execer := &fakeExecer{}
	repo := NewDeadLetterRepositoryWithExecer(execer)

	err := repo.Save(context.Background(), DeadLetterRecord{
		ID:           "00000000-0000-0000-0000-000000000010",
		Source:       "analytics.consumer",
		Payload:      map[string]string{"bad": "payload"},
		ErrorMessage: "invalid event payload",
		Retryable:    false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retryable, ok := execer.args[0][6].(bool); !ok || retryable {
		t.Fatalf("expected non-retryable flag in args, got %#v", execer.args[0])
	}
}
