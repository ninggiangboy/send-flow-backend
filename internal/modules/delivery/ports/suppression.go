package ports

import "context"

type SuppressionDecision struct {
	Suppressed bool
	Reason     string
	Scope      string
}

type SuppressionChecker interface {
	CheckSuppression(ctx context.Context, workspaceID, emailNormalized, scope string) (*SuppressionDecision, error)
}
