package listcampaigncandidates

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
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
	Status      string
	Limit       int
	Cursor      string
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
		log:           opts.Logger.With("usecase", "list_campaign_candidates"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.CampaignMessageCandidate, string, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "campaign.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, "", err
		}
		return nil, "", err
	}

	if _, err := h.campaignsRead.FindByID(ctx, cmd.WorkspaceID, cmd.CampaignID); err != nil {
		if errors.Is(err, domain.ErrCampaignNotFound) {
			return nil, "", err
		}
		log.Error("failed to verify campaign", "error", err)
		return nil, "", err
	}

	limit := cmd.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.CandidateListQuery{
		WorkspaceID: cmd.WorkspaceID,
		CampaignID:  cmd.CampaignID,
		Status:      cmd.Status,
		Limit:       limit,
		Cursor:      cmd.Cursor,
	}

	candidates, cursor, err := h.campaignsRead.ListCandidates(ctx, query)
	if err != nil {
		log.Error("failed to list candidates", "error", err)
		return nil, "", err
	}

	return candidates, cursor, nil
}
