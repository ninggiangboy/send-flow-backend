package ports

import "context"

// QuotaEnforcer checks and consumes API key email quota before accepting
// a transactional send request. Returns nil when within quota, or
// domain.ErrAPIKeyQuotaExceeded when quota would be exceeded.
// On infrastructure error (e.g., Redis down) it returns the error
// for fail-closed behavior.
type QuotaEnforcer interface {
	CheckAndConsume(ctx context.Context, workspaceID, apiKeyID string, recipientCount int) error
}
