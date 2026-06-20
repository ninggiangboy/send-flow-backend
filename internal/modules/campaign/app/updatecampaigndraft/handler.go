package updatecampaigndraft

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
)

type Options struct {
	CampaignsWrite ports.CampaignWriteRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type Command struct {
	WorkspaceID    string
	CampaignID     string
	UserID         string
	Name           *string
	AudienceRef    *domain.AudienceRef
	TemplateID     *string
	SenderDomainID *string
	MessageType    *domain.MessageType
	Now            time.Time
}

type Handler struct {
	campaignsWrite ports.CampaignWriteRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		campaignsWrite: opts.CampaignsWrite,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "update_campaign_draft"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Campaign, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "campaign_id", cmd.CampaignID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "campaign.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
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

	if !campaign.CanUpdateDraft() {
		return nil, domain.ErrInvalidStateTransition
	}

	if cmd.Name != nil {
		if err := domain.ValidateCampaignName(*cmd.Name); err != nil {
			return nil, err
		}
		campaign.Name = *cmd.Name
	}
	if cmd.AudienceRef != nil {
		if err := domain.ValidateAudienceRef(*cmd.AudienceRef); err != nil {
			return nil, err
		}
		campaign.AudienceRef = *cmd.AudienceRef
	}
	if cmd.TemplateID != nil {
		if *cmd.TemplateID == "" {
			return nil, domain.ErrPayloadInvalid
		}
		campaign.TemplateRef = domain.TemplateRef{TemplateID: *cmd.TemplateID}
	}
	if cmd.SenderDomainID != nil {
		if *cmd.SenderDomainID == "" {
			return nil, domain.ErrPayloadInvalid
		}
		campaign.SenderDomainID = *cmd.SenderDomainID
	}
	if cmd.MessageType != nil {
		if !domain.ValidMessageType(string(*cmd.MessageType)) {
			return nil, domain.ErrPayloadInvalid
		}
		campaign.MessageType = *cmd.MessageType
	}
	campaign.UpdatedAt = cmd.Now

	if err := h.campaignsWrite.Update(ctx, *campaign); err != nil {
		log.Error("failed to update campaign", "error", err)
		return nil, err
	}

	log.Info("campaign draft updated")
	return campaign, nil
}
