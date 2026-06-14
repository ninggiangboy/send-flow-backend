package listcampaigns

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
	WorkspaceID    string
	UserID         string
	Status         string
	SenderDomainID string
	TemplateID     string
	From           string
	To             string
	Limit          int
	Cursor         string
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
		log:           opts.Logger.With("usecase", "list_campaigns"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.Campaign, string, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "campaign.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, "", err
		}
		return nil, "", err
	}

	limit := cmd.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.CampaignListQuery{
		WorkspaceID:    cmd.WorkspaceID,
		Status:         cmd.Status,
		SenderDomainID: cmd.SenderDomainID,
		TemplateID:     cmd.TemplateID,
		From:           cmd.From,
		To:             cmd.To,
		Limit:          limit,
		Cursor:         cmd.Cursor,
	}

	campaigns, cursor, err := h.campaignsRead.List(ctx, query)
	if err != nil {
		log.Error("failed to list campaigns", "error", err)
		return nil, "", err
	}

	return campaigns, cursor, nil
}
