package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
)

type APIKeyListQuery struct {
	WorkspaceID string
	Status      string
	Limit       int
	Cursor      string
}

type APIKeyRepository interface {
	ListByWorkspace(ctx context.Context, query APIKeyListQuery) ([]domain.APIKey, string, error)
	FindByID(ctx context.Context, workspaceID, keyID string) (*domain.APIKey, error)
	FindByPrefix(ctx context.Context, keyPrefix string) (*domain.APIKey, error)
	Create(ctx context.Context, key domain.APIKey) error
	Update(ctx context.Context, key domain.APIKey) error
	TouchLastUsed(ctx context.Context, workspaceID, keyID string, usedAt time.Time) error
}

type WorkspaceAccessChecker = auth.WorkspaceAccessChecker

type IDGenerator func() (string, error)

type SecretGenerator interface {
	Generate() (plaintext, keyPrefix string, err error)
}

type SecretHasher interface {
	Hash(secret string) (string, error)
	Verify(hash, secret string) bool
}
