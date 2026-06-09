package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

type OutboxRepository interface {
	GetSummary(ctx context.Context, workspaceID string, filter domain.OutboxFilter) (domain.OutboxSummary, error)
	FindByID(ctx context.Context, workspaceID, outboxID string) (*domain.OutboxRecord, error)
	List(ctx context.Context, workspaceID string, filter domain.OutboxFilter) ([]domain.OutboxRecord, string, error)
	CreateReplayOutboxEvent(ctx context.Context, event domain.OutboxRecord) error
}

type DeadLetterRepository interface {
	FindByID(ctx context.Context, workspaceID, id string) (*domain.DeadLetterRecord, error)
	List(ctx context.Context, workspaceID string, filter domain.DeadLetterFilter) ([]domain.DeadLetterRecord, string, error)
}

type ReplayJobRepository interface {
	Create(ctx context.Context, job domain.ReplayJob) error
	FindByID(ctx context.Context, workspaceID, jobID string) (*domain.ReplayJob, error)
	List(ctx context.Context, workspaceID string, filter domain.ReplayJobFilter) ([]domain.ReplayJob, string, error)
	MarkRunning(ctx context.Context, workspaceID, jobID string, startedAt time.Time) error
	MarkCompleted(ctx context.Context, workspaceID, jobID string, result map[string]any, completedAt time.Time) error
	MarkFailed(ctx context.Context, workspaceID, jobID, errorMessage string, completedAt time.Time) error
}

type WorkspaceAccessChecker interface {
	RequirePermission(ctx context.Context, workspaceID, userID, permission string) error
}

type TransactionManager interface {
	RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type OutboxWriter interface {
	Write(ctx context.Context, eventType, aggregateID, workspaceID string, payload any) error
}
