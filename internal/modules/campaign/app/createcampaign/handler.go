package createcampaign

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
	IDGen          func() (string, error)
	Logger         *slog.Logger
}

type Command struct {
	WorkspaceID    string
	UserID         string
	Name           string
	AudienceRef    domain.AudienceRef
	TemplateID     string
	SenderDomainID string
	MessageType    domain.MessageType
	Now            time.Time
}

type Handler struct {
	campaignsWrite ports.CampaignWriteRepository
	accessChecker  ports.WorkspaceAccessChecker
	idGen          func() (string, error)
	log            *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		campaignsWrite: opts.CampaignsWrite,
		accessChecker:  opts.AccessChecker,
		idGen:          opts.IDGen,
		log:            opts.Logger.With("usecase", "create_campaign"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Campaign, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "campaign.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if err := domain.ValidateCampaignName(cmd.Name); err != nil {
		return nil, err
	}
	if !domain.ValidMessageType(string(cmd.MessageType)) {
		return nil, domain.ErrPayloadInvalid
	}
	if !domain.ValidAudienceType(string(cmd.AudienceRef.Type)) {
		return nil, domain.ErrPayloadInvalid
	}
	if err := domain.ValidateAudienceRef(cmd.AudienceRef); err != nil {
		return nil, err
	}
	if cmd.TemplateID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if cmd.SenderDomainID == "" {
		return nil, domain.ErrPayloadInvalid
	}

	id, err := h.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	campaign := domain.Campaign{
		ID:             id,
		WorkspaceID:    cmd.WorkspaceID,
		Name:           cmd.Name,
		Status:         domain.CampaignStatusDraft,
		AudienceRef:    cmd.AudienceRef,
		TemplateRef:    domain.TemplateRef{TemplateID: cmd.TemplateID},
		SenderDomainID: cmd.SenderDomainID,
		MessageType:    cmd.MessageType,
		CreatedAt:      cmd.Now,
		UpdatedAt:      cmd.Now,
	}

	if err := h.campaignsWrite.Create(ctx, campaign); err != nil {
		log.Error("failed to create campaign", "error", err)
		return nil, err
	}

	log.Info("campaign created", "campaign_id", id, "user_id", cmd.UserID)
	return &campaign, nil
}
