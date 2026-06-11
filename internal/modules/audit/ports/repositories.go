package ports

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audit/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
)

type EntryReadRepository interface {
	List(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, string, error)
}

type EntryWriteRepository interface {
	Append(ctx context.Context, entry domain.AuditEntry) error
}

type PermissionChecker = auth.WorkspaceAccessChecker
