package api

import (
	"context"
	"errors"

	audiencedomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	contentdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	identitydomain "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
	suppressiondomain "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
)

type moduleErrFunc func() error

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
			return mapPermissionErr(permission, true)
		}
		return err
	}
	if !identitydomain.HasPermission(membership.EffectivePermissions, permission) {
		return mapPermissionErr(permission, false)
	}
	return nil
}

func mapPermissionErr(permission string, workspaceDenied bool) error {
	if workspaceDenied {
		switch {
		case permission == "audience.read":
			return audiencedomain.ErrReadDenied
		case permission == "audience.write":
			return audiencedomain.ErrWriteDenied
		case permission == "audience.import":
			return audiencedomain.ErrImportDenied
		case permission == "audience.export":
			return audiencedomain.ErrExportDenied
		case permission == "template.read":
			return contentdomain.ErrReadDenied
		case permission == "template.write":
			return contentdomain.ErrWriteDenied
		case permission == "template.render":
			return contentdomain.ErrRenderDenied
		case permission == "suppression.read":
			return suppressiondomain.ErrReadDenied
		case permission == "suppression.manage":
			return suppressiondomain.ErrManageDenied
		default:
			return senderdomain.ErrManageDenied
		}
	}
	switch {
	case permission == "audience.read":
		return audiencedomain.ErrReadDenied
	case permission == "audience.write":
		return audiencedomain.ErrWriteDenied
	case permission == "audience.import":
		return audiencedomain.ErrImportDenied
	case permission == "audience.export":
		return audiencedomain.ErrExportDenied
	case permission == "template.read":
		return contentdomain.ErrReadDenied
	case permission == "template.write":
		return contentdomain.ErrWriteDenied
	case permission == "template.render":
		return contentdomain.ErrRenderDenied
	case permission == "suppression.read":
		return suppressiondomain.ErrReadDenied
	case permission == "suppression.manage":
		return suppressiondomain.ErrManageDenied
	default:
		return senderdomain.ErrManageDenied
	}
}
