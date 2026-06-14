package getcampaign

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
)

type Options struct {
	CampaignsRead ports.CampaignReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
}

type Handler struct {
	campaignsRead ports.CampaignReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		campaignsRead: opts.CampaignsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "get_campaign"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Campaign, int64, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "campaign_id", cmd.CampaignID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "campaign.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, 0, err
		}
		return nil, 0, err
	}

	campaign, err := h.campaignsRead.FindByID(ctx, cmd.WorkspaceID, cmd.CampaignID)
	if err != nil {
		if errors.Is(err, domain.ErrCampaignNotFound) {
			return nil, 0, err
		}
		log.Error("failed to find campaign", "error", err)
		return nil, 0, err
	}

	count, err := h.campaignsRead.CountCandidates(ctx, cmd.WorkspaceID, cmd.CampaignID)
	if err != nil {
		log.Error("failed to count candidates", "error", err)
	}

	return campaign, count, nil
}
