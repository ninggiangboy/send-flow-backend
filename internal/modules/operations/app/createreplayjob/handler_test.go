package createreplayjob

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

type stubDeadLetterRepo struct {
	findByIDFn func(ctx context.Context, workspaceID, id string) (*domain.DeadLetterRecord, error)
}

func (s *stubDeadLetterRepo) FindByID(ctx context.Context, workspaceID, id string) (*domain.DeadLetterRecord, error) {
	return s.findByIDFn(ctx, workspaceID, id)
}
func (s *stubDeadLetterRepo) List(ctx context.Context, workspaceID string, filter domain.DeadLetterFilter) ([]domain.DeadLetterRecord, string, error) {
	return nil, "", errors.New("unexpected")
}

type stubOutboxRepo struct {
	findByIDFn func(ctx context.Context, workspaceID, outboxID string) (*domain.OutboxRecord, error)
}

func (s *stubOutboxRepo) GetSummary(ctx context.Context, workspaceID string, filter domain.OutboxFilter) (domain.OutboxSummary, error) {
	return domain.OutboxSummary{}, errors.New("unexpected")
}
func (s *stubOutboxRepo) FindByID(ctx context.Context, workspaceID, outboxID string) (*domain.OutboxRecord, error) {
	return s.findByIDFn(ctx, workspaceID, outboxID)
}
func (s *stubOutboxRepo) List(ctx context.Context, workspaceID string, filter domain.OutboxFilter) ([]domain.OutboxRecord, string, error) {
	return nil, "", errors.New("unexpected")
}
func (s *stubOutboxRepo) CreateReplayOutboxEvent(ctx context.Context, event domain.OutboxRecord) error {
	return errors.New("unexpected")
}

type stubReplayJobRepo struct {
	createFn func(ctx context.Context, job domain.ReplayJob) error
}

func (s *stubReplayJobRepo) Create(ctx context.Context, job domain.ReplayJob) error {
	return s.createFn(ctx, job)
}
func (s *stubReplayJobRepo) FindByID(ctx context.Context, workspaceID, jobID string) (*domain.ReplayJob, error) {
	return nil, errors.New("unexpected")
}
func (s *stubReplayJobRepo) List(ctx context.Context, workspaceID string, filter domain.ReplayJobFilter) ([]domain.ReplayJob, string, error) {
	return nil, "", errors.New("unexpected")
}
func (s *stubReplayJobRepo) MarkRunning(ctx context.Context, workspaceID, jobID string, startedAt time.Time) error {
	return errors.New("unexpected")
}
func (s *stubReplayJobRepo) MarkCompleted(ctx context.Context, workspaceID, jobID string, result map[string]any, completedAt time.Time) error {
	return errors.New("unexpected")
}
func (s *stubReplayJobRepo) MarkFailed(ctx context.Context, workspaceID, jobID, errorMessage string, completedAt time.Time) error {
	return errors.New("unexpected")
}

type stubOutboxWriter struct {
	writeFn func(ctx context.Context, eventType, aggregateID, workspaceID string, payload any) error
}

func (s *stubOutboxWriter) Write(ctx context.Context, eventType, aggregateID, workspaceID string, payload any) error {
	return s.writeFn(ctx, eventType, aggregateID, workspaceID, payload)
}

type stubAccessChecker struct {
	requirePermissionFn func(ctx context.Context, workspaceID, userID, permission string) error
}

func (s *stubAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return s.requirePermissionFn(ctx, workspaceID, userID, permission)
}

var errPermissionDenied = errors.New("permission denied")

func okAccess(_ context.Context, _, _, _ string) error   { return nil }
func denyAccess(_ context.Context, _, _, _ string) error { return errPermissionDenied }

func newTestHandler(t *testing.T, deps *Options) *Handler {
	t.Helper()
	opts := *deps
	if opts.Clock == nil {
		opts.Clock = func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "test-id-1", nil }
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	return New(opts)
}

func TestCreateReplayJob_Success(t *testing.T) {
	workspaceID := "ws-1"
	var writtenJob domain.ReplayJob

	deadLetterRepo := &stubDeadLetterRepo{
		findByIDFn: func(_ context.Context, wid, id string) (*domain.DeadLetterRecord, error) {
			return &domain.DeadLetterRecord{ID: id, WorkspaceID: wid}, nil
		},
	}
	replayJobRepo := &stubReplayJobRepo{
		createFn: func(_ context.Context, job domain.ReplayJob) error {
			writtenJob = job
			return nil
		},
	}
	outboxWriter := &stubOutboxWriter{
		writeFn: func(_ context.Context, _, _, _ string, _ any) error {
			return nil
		},
	}

	h := newTestHandler(t, &Options{
		DeadLetterRepo: deadLetterRepo,
		OutboxRepo:     &stubOutboxRepo{},
		ReplayJobRepo:  replayJobRepo,
		OutboxWriter:   outboxWriter,
		AccessChecker:  &stubAccessChecker{requirePermissionFn: okAccess},
	})

	job, err := h.Execute(context.Background(), Input{
		WorkspaceID: workspaceID,
		UserID:      "user-1",
		TargetType:  domain.ReplayTargetDeadLetter,
		TargetID:    "dlq-1",
		Source:      "manual",
		Reason:      "testing replay",
		Filter:      json.RawMessage(`{"event_type":"msg.sent.v1"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job == nil {
		t.Fatal("expected non-nil result")
	}
	if job.TargetType != domain.ReplayTargetDeadLetter {
		t.Fatalf("expected target type dead_letter_record, got %s", job.TargetType)
	}
	if job.Status != domain.ReplayJobQueued {
		t.Fatalf("expected status queued, got %s", job.Status)
	}
	if writtenJob.ID != job.ID {
		t.Fatal("written job ID does not match result ID")
	}
}

func TestCreateReplayJob_PermissionDenied(t *testing.T) {
	h := newTestHandler(t, &Options{
		AccessChecker: &stubAccessChecker{requirePermissionFn: denyAccess},
	})
	_, err := h.Execute(context.Background(), Input{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestCreateReplayJob_InvalidTargetType(t *testing.T) {
	h := newTestHandler(t, &Options{
		AccessChecker: &stubAccessChecker{requirePermissionFn: okAccess},
	})
	_, err := h.Execute(context.Background(), Input{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		TargetType:  "invalid_target",
		TargetID:    "some-id",
		Source:      "manual",
		Reason:      "testing",
	})
	if err == nil {
		t.Fatal("expected invalid target type error")
	}
}
