package api

import (
	"context"
	"errors"

	accessdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	audiencedomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	campaigndomain "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	contentdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	deliverydomain "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
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
		case permission == "campaign.read":
			return campaigndomain.ErrReadDenied
		case permission == "campaign.write":
			return campaigndomain.ErrWriteDenied
		case permission == "campaign.send":
			return campaigndomain.ErrSendDenied
		case permission == "delivery.read":
			return deliverydomain.ErrReadDenied
		case permission == "api_key.manage":
			return accessdomain.ErrAPIKeyManageDenied
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
	case permission == "campaign.read":
		return campaigndomain.ErrReadDenied
	case permission == "campaign.write":
		return campaigndomain.ErrWriteDenied
	case permission == "campaign.send":
		return campaigndomain.ErrSendDenied
	case permission == "delivery.read":
		return deliverydomain.ErrReadDenied
	case permission == "api_key.manage":
		return accessdomain.ErrAPIKeyManageDenied
	default:
		return senderdomain.ErrManageDenied
	}
}
