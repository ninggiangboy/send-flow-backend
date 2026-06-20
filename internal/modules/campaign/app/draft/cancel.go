package draft

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
)

type CancelOptions struct {
	CampaignsWrite ports.CampaignWriteRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type CancelInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	Now         time.Time
}

type CancelHandler struct {
	campaignsWrite ports.CampaignWriteRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

func NewCancelHandler(opts CancelOptions) *CancelHandler {
	return &CancelHandler{
		campaignsWrite: opts.CampaignsWrite,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "cancel_campaign"),
	}
}

func (h *CancelHandler) Execute(ctx context.Context, cmd CancelInput) (*domain.Campaign, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "campaign_id", cmd.CampaignID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "campaign.send"); err != nil {
		if errors.Is(err, domain.ErrSendDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrSendDenied
		}
		return nil, err
	}

	campaign, err := h.campaignsWrite.FindByID(ctx, cmd.WorkspaceID, cmd.CampaignID)
	if err != nil {
		if errors.Is(err, domain.ErrCampaignNotFound) {
			return nil, err
		}
		log.Error("failed to find campaign", "error", err)
		return nil, err
	}

	if !campaign.CanCancel() {
		return nil, domain.ErrInvalidStateTransition
	}

	campaign.Status = domain.CampaignStatusCancelled
	campaign.CancelledAt = &cmd.Now
	campaign.UpdatedAt = cmd.Now

	if err := h.campaignsWrite.Update(ctx, *campaign); err != nil {
		log.Error("failed to cancel campaign", "error", err)
		return nil, err
	}

	log.Info("campaign cancelled")
	return campaign, nil
}
