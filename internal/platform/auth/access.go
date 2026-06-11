package auth

import (
	"context"
	"errors"
)

var ErrPermissionDenied = errors.New("permission denied")

type PermissionDeniedError struct {
	Permission string
}

func (e *PermissionDeniedError) Error() string { return "permission denied: " + e.Permission }
func (e *PermissionDeniedError) Unwrap() error { return ErrPermissionDenied }

type WorkspaceAccessChecker interface {
	RequirePermission(ctx context.Context, workspaceID, userID, permission string) error
}
