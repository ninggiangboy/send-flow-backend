package api

import (
	"context"
	"errors"

	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	identitydomain "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
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
			return senderdomain.ErrManageDenied
		}
		return err
	}
	if !identitydomain.HasPermission(membership.EffectivePermissions, permission) {
		return senderdomain.ErrManageDenied
	}
	return nil
}
