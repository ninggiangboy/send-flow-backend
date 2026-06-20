package outbox

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

type outboxRepoStub struct {
	getSummaryFn func(context.Context, string, domain.OutboxFilter) (domain.OutboxSummary, error)
}

func (s *outboxRepoStub) GetSummary(ctx context.Context, workspaceID string, filter domain.OutboxFilter) (domain.OutboxSummary, error) {
	return s.getSummaryFn(ctx, workspaceID, filter)
}
func (outboxRepoStub) FindByID(context.Context, string, string) (*domain.OutboxRecord, error) {
	return nil, errors.New("unexpected")
}
func (outboxRepoStub) List(context.Context, string, domain.OutboxFilter) ([]domain.OutboxRecord, string, error) {
	return nil, "", errors.New("unexpected")
}
func (outboxRepoStub) CreateReplayOutboxEvent(context.Context, domain.OutboxRecord) error {
	return errors.New("unexpected")
}

type outboxAccessCheckerStub struct {
	requirePermissionFn func(context.Context, string, string, string) error
}

func (s *outboxAccessCheckerStub) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return s.requirePermissionFn(ctx, workspaceID, userID, permission)
}

func TestQueryServiceGetSummarySuccess(t *testing.T) {
	svc := NewQueryService(Options{
		OutboxRepo: &outboxRepoStub{getSummaryFn: func(_ context.Context, wid string, _ domain.OutboxFilter) (domain.OutboxSummary, error) {
			return domain.OutboxSummary{TotalCount: 5, ByEventType: map[string]int{"msg.sent.v1": 3}}, nil
		}},
		AccessChecker: &outboxAccessCheckerStub{requirePermissionFn: func(context.Context, string, string, string) error { return nil }},
		Logger:        slog.New(slog.DiscardHandler),
	})

	summary, err := svc.GetSummary(context.Background(), SummaryInput{WorkspaceID: "ws_1", UserID: "user_1", Filter: domain.OutboxFilter{Limit: 10}})
	if err != nil || summary.TotalCount != 5 || summary.ByEventType["msg.sent.v1"] != 3 {
		t.Fatalf("unexpected result: %+v err=%v", summary, err)
	}
}
