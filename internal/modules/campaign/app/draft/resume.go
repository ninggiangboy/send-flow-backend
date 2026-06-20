package draft

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
)

type ResumeOptions struct {
	CampaignsWrite ports.CampaignWriteRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type ResumeInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	Now         time.Time
}

type ResumeHandler struct {
	campaignsWrite ports.CampaignWriteRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

func NewResumeHandler(opts ResumeOptions) *ResumeHandler {
	return &ResumeHandler{
		campaignsWrite: opts.CampaignsWrite,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "resume_campaign"),
	}
}

func (h *ResumeHandler) Execute(ctx context.Context, cmd ResumeInput) (*domain.Campaign, error) {
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

	if !campaign.CanResume() {
		return nil, domain.ErrInvalidStateTransition
	}

	campaign.Status = domain.CampaignStatusScheduled
	campaign.UpdatedAt = cmd.Now

	if err := h.campaignsWrite.Update(ctx, *campaign); err != nil {
		log.Error("failed to resume campaign", "error", err)
		return nil, err
	}

	log.Info("campaign resumed")
	return campaign, nil
}
