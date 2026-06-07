package ports

import "context"

type WorkspaceAccessChecker interface {
	RequirePermission(ctx context.Context, workspaceID, userID, permission string) error
}
