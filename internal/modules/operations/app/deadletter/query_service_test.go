package deadletter

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

type deadLetterRepoStub struct {
	findByIDFn func(context.Context, string, string) (*domain.DeadLetterRecord, error)
}

func (s *deadLetterRepoStub) FindByID(ctx context.Context, workspaceID, id string) (*domain.DeadLetterRecord, error) {
	return s.findByIDFn(ctx, workspaceID, id)
}
func (s *deadLetterRepoStub) List(context.Context, string, domain.DeadLetterFilter) ([]domain.DeadLetterRecord, string, error) {
	return nil, "", errors.New("unexpected")
}

type deadLetterAccessCheckerStub struct {
	requirePermissionFn func(context.Context, string, string, string) error
}

func (s *deadLetterAccessCheckerStub) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return s.requirePermissionFn(ctx, workspaceID, userID, permission)
}

func TestQueryServiceGetRecordSuccess(t *testing.T) {
	svc := NewQueryService(Options{
		DeadLetterRepo: &deadLetterRepoStub{findByIDFn: func(context.Context, string, string) (*domain.DeadLetterRecord, error) {
			return &domain.DeadLetterRecord{ID: "dlq_1"}, nil
		}},
		AccessChecker: &deadLetterAccessCheckerStub{requirePermissionFn: func(context.Context, string, string, string) error { return nil }},
		Logger:        slog.New(slog.DiscardHandler),
	})

	record, err := svc.GetRecord(context.Background(), GetInput{WorkspaceID: "ws_1", UserID: "user_1", RecordID: "dlq_1"})
	if err != nil || record.ID != "dlq_1" {
		t.Fatalf("unexpected result: %+v err=%v", record, err)
	}
}
