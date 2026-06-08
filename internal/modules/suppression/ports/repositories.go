package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
)

type SuppressionListQuery struct {
	WorkspaceID string
	Email       string
	Scope       string
	Reason      string
	From        *time.Time
	To          *time.Time
	Cursor      string
	Limit       int
}

type SuppressionCheckQuery struct {
	WorkspaceID     string
	EmailNormalized string
	Scopes          []string
	Reasons         []string
}

type SuppressionReadRepository interface {
	FindByID(ctx context.Context, workspaceID, entryID string) (*domain.SuppressionEntry, error)
	List(ctx context.Context, query SuppressionListQuery) ([]domain.SuppressionEntry, string, error)
	FindActiveByEmail(ctx context.Context, query SuppressionCheckQuery) (*domain.SuppressionEntry, error)
}

type SuppressionWriteRepository interface {
	Create(ctx context.Context, entry domain.SuppressionEntry) error
	Remove(ctx context.Context, workspaceID, entryID string, removedAt time.Time) error
}
