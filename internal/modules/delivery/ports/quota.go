package ports

import "context"

// QuotaEnforcer checks, consumes, and refunds API key email quota.
// CheckAndConsume returns nil when within quota, domain.ErrAPIKeyQuotaExceeded
// when exhausted, or an infrastructure error for fail-closed behavior.
// Refund restores previously consumed units and is best-effort — callers
// should log failures but must not propagate them to the user response.
type QuotaEnforcer interface {
	CheckAndConsume(ctx context.Context, workspaceID, apiKeyID string, recipientCount int) error
	Refund(ctx context.Context, workspaceID, apiKeyID string, recipientCount int) error
}
