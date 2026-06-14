package getoutboxsummary

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

type stubOutboxRepo struct {
	getSummaryFn func(ctx context.Context, workspaceID string, filter domain.OutboxFilter) (domain.OutboxSummary, error)
}

func (s *stubOutboxRepo) GetSummary(ctx context.Context, workspaceID string, filter domain.OutboxFilter) (domain.OutboxSummary, error) {
	return s.getSummaryFn(ctx, workspaceID, filter)
}
func (s *stubOutboxRepo) FindByID(ctx context.Context, workspaceID, outboxID string) (*domain.OutboxRecord, error) {
	return nil, errors.New("unexpected")
}
func (s *stubOutboxRepo) List(ctx context.Context, workspaceID string, filter domain.OutboxFilter) ([]domain.OutboxRecord, string, error) {
	return nil, "", errors.New("unexpected")
}
func (s *stubOutboxRepo) CreateReplayOutboxEvent(ctx context.Context, event domain.OutboxRecord) error {
	return errors.New("unexpected")
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

func newTestHandler(t *testing.T, repo *stubOutboxRepo, checker *stubAccessChecker) *Handler {
	t.Helper()
	if repo == nil {
		repo = &stubOutboxRepo{}
	}
	if checker == nil {
		checker = &stubAccessChecker{requirePermissionFn: okAccess}
	}
	return New(Options{
		OutboxRepo:    repo,
		AccessChecker: checker,
		Logger:        slog.New(slog.DiscardHandler),
	})
}

func TestGetOutboxSummary_Success(t *testing.T) {
	workspaceID := "ws-1"
	repo := &stubOutboxRepo{
		getSummaryFn: func(_ context.Context, wid string, _ domain.OutboxFilter) (domain.OutboxSummary, error) {
			if wid != workspaceID {
				t.Fatalf("expected workspace %s, got %s", workspaceID, wid)
			}
			return domain.OutboxSummary{
				TotalCount:   5,
				OldestAgeSec: 120,
				ByEventType:  map[string]int{"msg.sent.v1": 3, "msg.delivered.v1": 2},
			}, nil
		},
	}
	h := newTestHandler(t, repo, &stubAccessChecker{requirePermissionFn: okAccess})

	summary, err := h.Execute(context.Background(), Input{
		WorkspaceID: workspaceID,
		UserID:      "user-1",
		Filter:      domain.OutboxFilter{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalCount != 5 {
		t.Fatalf("expected TotalCount=5, got %d", summary.TotalCount)
	}
	if summary.ByEventType["msg.sent.v1"] != 3 {
		t.Fatalf("expected msg.sent.v1 count=3, got %d", summary.ByEventType["msg.sent.v1"])
	}
}

func TestGetOutboxSummary_PermissionDenied(t *testing.T) {
	h := newTestHandler(t, nil, &stubAccessChecker{requirePermissionFn: denyAccess})
	_, err := h.Execute(context.Background(), Input{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
}

func TestGetOutboxSummary_FilterValidationError(t *testing.T) {
	h := newTestHandler(t, nil, nil)
	_, err := h.Execute(context.Background(), Input{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		Filter:      domain.OutboxFilter{Limit: 999},
	})
	if err == nil {
		t.Fatal("expected filter validation error, got nil")
	}
}

func TestGetOutboxSummary_MissingWorkspace(t *testing.T) {
	h := newTestHandler(t, nil, nil)
	_, err := h.Execute(context.Background(), Input{
		WorkspaceID: "",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected workspace required error, got nil")
	}
}
