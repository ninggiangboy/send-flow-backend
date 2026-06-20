package ports

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type WorkspaceSettingsReadRepository interface {
	GetByWorkspace(ctx context.Context, workspaceID string) (*domain.WorkspaceSettings, error)
}

type WorkspaceSettingsWriteRepository interface {
	WorkspaceSettingsReadRepository
	CreateDefault(ctx context.Context, settings domain.WorkspaceSettings) error
	Upsert(ctx context.Context, settings domain.WorkspaceSettings, expectedVersion *int64) error
}
