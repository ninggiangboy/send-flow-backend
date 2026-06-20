package schedulecampaign

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type Options struct {
	CampaignsWrite   ports.CampaignWriteRepository
	AudienceResolver ports.AudienceResolver
	ContentService   ports.ContentService
	SenderService    ports.SenderService
	OutboxWriter     ports.OutboxWriter
	TxManager        ports.UnitOfWork
	AccessChecker    ports.WorkspaceAccessChecker
	IDGen            func() (string, error)
	Logger           *slog.Logger
}

type Command struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	ScheduledAt *time.Time
	Now         time.Time
}

type Result struct {
	Campaign   *domain.Campaign
	Candidates []domain.CampaignMessageCandidate
}

type Handler struct {
	campaignsWrite   ports.CampaignWriteRepository
	audienceResolver ports.AudienceResolver
	contentService   ports.ContentService
	senderService    ports.SenderService
	outboxWriter     ports.OutboxWriter
	txManager        ports.UnitOfWork
	accessChecker    ports.WorkspaceAccessChecker
	idGen            func() (string, error)
	log              *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		campaignsWrite:   opts.CampaignsWrite,
		audienceResolver: opts.AudienceResolver,
		contentService:   opts.ContentService,
		senderService:    opts.SenderService,
		outboxWriter:     opts.OutboxWriter,
		txManager:        opts.TxManager,
		accessChecker:    opts.AccessChecker,
		idGen:            opts.IDGen,
		log:              opts.Logger.With("usecase", "schedule_campaign"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*Result, error) {
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

	if !campaign.CanSchedule() {
		return nil, domain.ErrInvalidStateTransition
	}

	readiness, err := h.senderService.GetSenderReadiness(ctx, cmd.WorkspaceID, campaign.SenderDomainID)
	if err != nil {
		log.Error("failed to check sender readiness", "error", err)
		return nil, err
	}
	if readiness == nil || !readiness.Ready {
		return nil, domain.ErrSenderNotVerified
	}

	publishedVersion, err := h.contentService.GetPublishedTemplateVersion(ctx, cmd.WorkspaceID, campaign.TemplateRef.TemplateID)
	if err != nil {
		log.Error("failed to get published template version", "error", err)
		return nil, domain.ErrTemplatePublishRequired
	}
	if publishedVersion == nil {
		return nil, domain.ErrTemplatePublishRequired
	}

	if err := h.contentService.ValidateTemplateRenderable(ctx, cmd.WorkspaceID, campaign.TemplateRef.TemplateID); err != nil {
		log.Error("template not renderable", "error", err)
		return nil, domain.ErrTemplatePublishRequired
	}

	audienceRef := ports.AudienceSelectionRef{
		ContactIDs: campaign.AudienceRef.ContactIDs,
	}
	switch campaign.AudienceRef.Type {
	case domain.AudienceTypeSegment:
		audienceRef.SegmentID = campaign.AudienceRef.ID
	case domain.AudienceTypeList:
		audienceRef.ListID = campaign.AudienceRef.ID
	}

	recipients, err := h.audienceResolver.ResolveAudienceRecipients(ctx, cmd.WorkspaceID, cmd.UserID, audienceRef)
	if err != nil {
		log.Error("failed to resolve audience recipients", "error", err)
		return nil, err
	}
	if len(recipients) == 0 {
		return nil, domain.ErrAudienceNotReady
	}

	scheduledAt := cmd.Now
	if cmd.ScheduledAt != nil {
		scheduledAt = *cmd.ScheduledAt
	}

	campaign.Status = domain.CampaignStatusScheduled
	campaign.TemplateRef.TemplateVersionID = publishedVersion.ID
	campaign.PlannedRecipients = int64(len(recipients))
	campaign.ScheduledAt = &scheduledAt
	campaign.UpdatedAt = cmd.Now

	candidates := make([]domain.CampaignMessageCandidate, 0, len(recipients))
	for _, r := range recipients {
		cid, err := h.idGen()
		if err != nil {
			log.Error("failed to generate candidate id", "error", err)
			return nil, err
		}
		candidates = append(candidates, domain.CampaignMessageCandidate{
			ID:              cid,
			WorkspaceID:     cmd.WorkspaceID,
			CampaignID:      campaign.ID,
			ContactID:       r.ContactID,
			EmailNormalized: r.EmailNormalized,
			RecipientSnapshot: domain.RecipientSnapshot{
				ContactID:       r.ContactID,
				Email:           r.Email,
				EmailNormalized: r.EmailNormalized,
				FirstName:       r.FirstName,
				LastName:        r.LastName,
				Tags:            r.Tags,
				Attributes:      r.Attributes,
			},
			Status:    domain.CandidateStatusPlanned,
			CreatedAt: cmd.Now,
			UpdatedAt: cmd.Now,
		})
	}

	audienceRefJSON, err := json.Marshal(campaign.AudienceRef)
	if err != nil {
		log.Error("failed to marshal audience ref", "error", err)
		return nil, err
	}

	eventID, err := h.idGen()
	if err != nil {
		log.Error("failed to generate event id", "error", err)
		return nil, err
	}
	outboxPayload := contracts.CampaignScheduledPayload{
		CampaignID:        campaign.ID,
		WorkspaceID:       cmd.WorkspaceID,
		AudienceRef:       audienceRefJSON,
		TemplateID:        campaign.TemplateRef.TemplateID,
		TemplateVersionID: publishedVersion.ID,
		SenderDomainID:    campaign.SenderDomainID,
		MessageType:       string(campaign.MessageType),
		ScheduledAt:       scheduledAt.Format(time.RFC3339),
		PlannedRecipients: int64(len(recipients)),
	}
	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventCampaignScheduledV1,
		EventVersion:  1,
		AggregateType: "campaign",
		AggregateID:   campaign.ID,
		WorkspaceID:   cmd.WorkspaceID,
		OccurredAt:    cmd.Now,
	}, outboxPayload)
	if err != nil {
		log.Error("failed to create outbox envelope", "error", err)
		return nil, err
	}
	payloadBytes, err := events.Marshal(envelope)
	if err != nil {
		log.Error("failed to marshal envelope", "error", err)
		return nil, err
	}

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.campaignsWrite.Update(txCtx, *campaign); err != nil {
			return err
		}
		if err := h.campaignsWrite.ReplaceCandidates(txCtx, cmd.WorkspaceID, campaign.ID, candidates); err != nil {
			return err
		}
		if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
			ID:            eventID,
			AggregateType: "campaign",
			AggregateID:   campaign.ID,
			EventType:     contracts.EventCampaignScheduledV1,
			Payload:       payloadBytes,
			WorkspaceID:   cmd.WorkspaceID,
			OccurredAt:    cmd.Now,
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Error("failed to execute schedule transaction", "error", err)
		return nil, err
	}

	log.Info("campaign scheduled", "planned_recipients", len(recipients))
	return &Result{Campaign: campaign, Candidates: candidates}, nil
}
