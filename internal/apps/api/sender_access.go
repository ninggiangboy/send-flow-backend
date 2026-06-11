package api

import (
	"context"
	"errors"

	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	identitydomain "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
)

type workspaceAccessAdapter struct {
	svc *identityapp.Service
}

func newWorkspaceAccessAdapter(svc *identityapp.Service) *workspaceAccessAdapter {
	return &workspaceAccessAdapter{svc: svc}
}

func (a *workspaceAccessAdapter) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	membership, err := a.svc.GetWorkspaceAccess(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, identitydomain.ErrWorkspaceAccessDenied) {
			return &auth.PermissionDeniedError{Permission: permission}
		}
		return err
	}
	if !identitydomain.HasPermission(membership.EffectivePermissions, permission) {
		return &auth.PermissionDeniedError{Permission: permission}
	}
	return nil
}
