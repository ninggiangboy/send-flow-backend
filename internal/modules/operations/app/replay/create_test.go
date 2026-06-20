package replay

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

type replayDeadLetterRepoStub struct {
	findByIDFn func(context.Context, string, string) (*domain.DeadLetterRecord, error)
}

func (s *replayDeadLetterRepoStub) FindByID(ctx context.Context, workspaceID, id string) (*domain.DeadLetterRecord, error) {
	return s.findByIDFn(ctx, workspaceID, id)
}
func (s *replayDeadLetterRepoStub) List(context.Context, string, domain.DeadLetterFilter) ([]domain.DeadLetterRecord, string, error) {
	return nil, "", errors.New("unexpected")
}

type replayOutboxRepoStub struct{}

func (replayOutboxRepoStub) GetSummary(context.Context, string, domain.OutboxFilter) (domain.OutboxSummary, error) {
	return domain.OutboxSummary{}, errors.New("unexpected")
}
func (replayOutboxRepoStub) FindByID(context.Context, string, string) (*domain.OutboxRecord, error) {
	return nil, errors.New("unexpected")
}
func (replayOutboxRepoStub) List(context.Context, string, domain.OutboxFilter) ([]domain.OutboxRecord, string, error) {
	return nil, "", errors.New("unexpected")
}
func (replayOutboxRepoStub) CreateReplayOutboxEvent(context.Context, domain.OutboxRecord) error {
	return errors.New("unexpected")
}

type replayJobRepoStub struct {
	createFn func(context.Context, domain.ReplayJob) error
}

func (s *replayJobRepoStub) Create(ctx context.Context, job domain.ReplayJob) error {
	return s.createFn(ctx, job)
}
func (replayJobRepoStub) FindByID(context.Context, string, string) (*domain.ReplayJob, error) {
	return nil, errors.New("unexpected")
}
func (replayJobRepoStub) List(context.Context, string, domain.ReplayJobFilter) ([]domain.ReplayJob, string, error) {
	return nil, "", errors.New("unexpected")
}
func (replayJobRepoStub) MarkRunning(context.Context, string, string, time.Time) error {
	return errors.New("unexpected")
}
func (replayJobRepoStub) MarkCompleted(context.Context, string, string, map[string]any, time.Time) error {
	return errors.New("unexpected")
}
func (replayJobRepoStub) MarkFailed(context.Context, string, string, string, time.Time) error {
	return errors.New("unexpected")
}

type replayOutboxWriterStub struct {
	writeFn func(context.Context, string, string, string, any) error
}

func (s *replayOutboxWriterStub) Write(ctx context.Context, eventType, aggregateID, workspaceID string, payload any) error {
	return s.writeFn(ctx, eventType, aggregateID, workspaceID, payload)
}

type replayAccessCheckerStub struct {
	requirePermissionFn func(context.Context, string, string, string) error
}

func (s *replayAccessCheckerStub) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return s.requirePermissionFn(ctx, workspaceID, userID, permission)
}

func TestCreateHandlerExecuteSuccess(t *testing.T) {
	var writtenJob domain.ReplayJob
	h := NewCreateHandler(CreateOptions{
		DeadLetterRepo: &replayDeadLetterRepoStub{findByIDFn: func(_ context.Context, wid, id string) (*domain.DeadLetterRecord, error) {
			return &domain.DeadLetterRecord{ID: id, WorkspaceID: wid}, nil
		}},
		OutboxRepo:    replayOutboxRepoStub{},
		ReplayJobRepo: &replayJobRepoStub{createFn: func(_ context.Context, job domain.ReplayJob) error { writtenJob = job; return nil }},
		OutboxWriter:  &replayOutboxWriterStub{writeFn: func(context.Context, string, string, string, any) error { return nil }},
		AccessChecker: &replayAccessCheckerStub{requirePermissionFn: func(context.Context, string, string, string) error { return nil }},
		IDGen:         func() (string, error) { return "job_1", nil },
		Clock:         func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
		Logger:        slog.New(slog.DiscardHandler),
	})

	job, err := h.Execute(context.Background(), CreateInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		TargetType:  domain.ReplayTargetDeadLetter,
		TargetID:    "dlq_1",
		Source:      "manual",
		Reason:      "testing replay",
		Filter:      json.RawMessage(`{"event_type":"msg.sent.v1"}`),
	})
	if err != nil || job == nil || writtenJob.ID != job.ID {
		t.Fatalf("unexpected result: job=%+v written=%+v err=%v", job, writtenJob, err)
	}
}
