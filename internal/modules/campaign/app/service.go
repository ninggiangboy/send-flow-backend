package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type Options struct {
	CampaignsRead    ports.CampaignReadRepository
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

type Service struct {
	campaignsRead    ports.CampaignReadRepository
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

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}
	return &Service{
		campaignsRead:    opts.CampaignsRead,
		campaignsWrite:   opts.CampaignsWrite,
		audienceResolver: opts.AudienceResolver,
		contentService:   opts.ContentService,
		senderService:    opts.SenderService,
		outboxWriter:     opts.OutboxWriter,
		txManager:        opts.TxManager,
		accessChecker:    opts.AccessChecker,
		idGen:            opts.IDGen,
		log:              opts.Logger.With("module", "campaign"),
	}
}

type CreateCampaignInput struct {
	WorkspaceID    string
	UserID         string
	Name           string
	AudienceRef    domain.AudienceRef
	TemplateID     string
	SenderDomainID string
	MessageType    domain.MessageType
	Now            time.Time
}

type UpdateCampaignDraftInput struct {
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

type ScheduleCampaignInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	ScheduledAt *time.Time
	Now         time.Time
}

type CampaignResult struct {
	Campaign       domain.Campaign
	CandidateCount int64
}

type CampaignListResult struct {
	Campaigns  []domain.Campaign
	NextCursor string
}

type CandidateListResult struct {
	Candidates []domain.CampaignMessageCandidate
	NextCursor string
}

type ListInput struct {
	WorkspaceID    string
	Status         string
	SenderDomainID string
	TemplateID     string
	From           string
	To             string
	Limit          int
	Cursor         string
}

type CandidateListInput struct {
	WorkspaceID string
	CampaignID  string
	Status      string
	Limit       int
	Cursor      string
}

func (s *Service) CreateCampaign(ctx context.Context, input CreateCampaignInput) (*CampaignResult, error) {
	log := s.log.With("usecase", "create_campaign", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "campaign.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if err := domain.ValidateCampaignName(input.Name); err != nil {
		return nil, err
	}
	if !domain.ValidMessageType(string(input.MessageType)) {
		return nil, domain.ErrPayloadInvalid
	}
	if !domain.ValidAudienceType(string(input.AudienceRef.Type)) {
		return nil, domain.ErrPayloadInvalid
	}
	if err := domain.ValidateAudienceRef(input.AudienceRef); err != nil {
		return nil, err
	}
	if input.TemplateID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if input.SenderDomainID == "" {
		return nil, domain.ErrPayloadInvalid
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	campaign := domain.Campaign{
		ID:             id,
		WorkspaceID:    input.WorkspaceID,
		Name:           input.Name,
		Status:         domain.CampaignStatusDraft,
		AudienceRef:    input.AudienceRef,
		TemplateRef:    domain.TemplateRef{TemplateID: input.TemplateID},
		SenderDomainID: input.SenderDomainID,
		MessageType:    input.MessageType,
		CreatedAt:      input.Now,
		UpdatedAt:      input.Now,
	}

	if err := s.campaignsWrite.Create(ctx, campaign); err != nil {
		log.Error("failed to create campaign", "error", err)
		return nil, err
	}

	log.Info("campaign created", "campaign_id", id, "user_id", input.UserID)
	return &CampaignResult{Campaign: campaign}, nil
}

func (s *Service) ListCampaigns(ctx context.Context, input ListInput, userID string) (*CampaignListResult, error) {
	log := s.log.With("usecase", "list_campaigns", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, userID, "campaign.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.CampaignListQuery{
		WorkspaceID:    input.WorkspaceID,
		Status:         input.Status,
		SenderDomainID: input.SenderDomainID,
		TemplateID:     input.TemplateID,
		From:           input.From,
		To:             input.To,
		Limit:          limit,
		Cursor:         input.Cursor,
	}

	campaigns, cursor, err := s.campaignsRead.List(ctx, query)
	if err != nil {
		log.Error("failed to list campaigns", "error", err)
		return nil, err
	}

	return &CampaignListResult{Campaigns: campaigns, NextCursor: cursor}, nil
}

func (s *Service) GetCampaign(ctx context.Context, workspaceID, campaignID, userID string) (*CampaignResult, error) {
	log := s.log.With("usecase", "get_campaign", "workspace_id", workspaceID, "campaign_id", campaignID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "campaign.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	campaign, err := s.campaignsRead.FindByID(ctx, workspaceID, campaignID)
	if err != nil {
		if errors.Is(err, domain.ErrCampaignNotFound) {
			return nil, err
		}
		log.Error("failed to find campaign", "error", err)
		return nil, err
	}

	count, err := s.campaignsRead.CountCandidates(ctx, workspaceID, campaignID)
	if err != nil {
		log.Error("failed to count candidates", "error", err)
	}

	return &CampaignResult{Campaign: *campaign, CandidateCount: count}, nil
}

func (s *Service) UpdateCampaignDraft(ctx context.Context, input UpdateCampaignDraftInput) (*CampaignResult, error) {
	log := s.log.With("usecase", "update_campaign_draft", "workspace_id", input.WorkspaceID, "campaign_id", input.CampaignID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "campaign.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	campaign, err := s.campaignsRead.FindByID(ctx, input.WorkspaceID, input.CampaignID)
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

	if input.Name != nil {
		if err := domain.ValidateCampaignName(*input.Name); err != nil {
			return nil, err
		}
		campaign.Name = *input.Name
	}
	if input.AudienceRef != nil {
		if err := domain.ValidateAudienceRef(*input.AudienceRef); err != nil {
			return nil, err
		}
		campaign.AudienceRef = *input.AudienceRef
	}
	if input.TemplateID != nil {
		if *input.TemplateID == "" {
			return nil, domain.ErrPayloadInvalid
		}
		campaign.TemplateRef = domain.TemplateRef{TemplateID: *input.TemplateID}
	}
	if input.SenderDomainID != nil {
		if *input.SenderDomainID == "" {
			return nil, domain.ErrPayloadInvalid
		}
		campaign.SenderDomainID = *input.SenderDomainID
	}
	if input.MessageType != nil {
		if !domain.ValidMessageType(string(*input.MessageType)) {
			return nil, domain.ErrPayloadInvalid
		}
		campaign.MessageType = *input.MessageType
	}
	campaign.UpdatedAt = input.Now

	if err := s.campaignsWrite.Update(ctx, *campaign); err != nil {
		log.Error("failed to update campaign", "error", err)
		return nil, err
	}

	log.Info("campaign draft updated")
	return &CampaignResult{Campaign: *campaign}, nil
}

func (s *Service) ScheduleCampaign(ctx context.Context, input ScheduleCampaignInput) (*CampaignResult, error) {
	log := s.log.With("usecase", "schedule_campaign", "workspace_id", input.WorkspaceID, "campaign_id", input.CampaignID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "campaign.send"); err != nil {
		if errors.Is(err, domain.ErrSendDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrSendDenied
		}
		return nil, err
	}

	campaign, err := s.campaignsRead.FindByID(ctx, input.WorkspaceID, input.CampaignID)
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

	readiness, err := s.senderService.GetSenderReadiness(ctx, input.WorkspaceID, campaign.SenderDomainID)
	if err != nil {
		log.Error("failed to check sender readiness", "error", err)
		return nil, err
	}
	if readiness == nil || !readiness.Ready {
		return nil, domain.ErrSenderNotVerified
	}

	publishedVersion, err := s.contentService.GetPublishedTemplateVersion(ctx, input.WorkspaceID, campaign.TemplateRef.TemplateID)
	if err != nil {
		log.Error("failed to get published template version", "error", err)
		return nil, domain.ErrTemplatePublishRequired
	}
	if publishedVersion == nil {
		return nil, domain.ErrTemplatePublishRequired
	}

	if err := s.contentService.ValidateTemplateRenderable(ctx, input.WorkspaceID, campaign.TemplateRef.TemplateID); err != nil {
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

	recipients, err := s.audienceResolver.ResolveAudienceRecipients(ctx, input.WorkspaceID, input.UserID, audienceRef)
	if err != nil {
		log.Error("failed to resolve audience recipients", "error", err)
		return nil, err
	}
	if len(recipients) == 0 {
		return nil, domain.ErrAudienceNotReady
	}

	scheduledAt := input.Now
	if input.ScheduledAt != nil {
		scheduledAt = *input.ScheduledAt
	}

	campaign.Status = domain.CampaignStatusScheduled
	campaign.TemplateRef.TemplateVersionID = publishedVersion.ID
	campaign.PlannedRecipients = int64(len(recipients))
	campaign.ScheduledAt = &scheduledAt
	campaign.UpdatedAt = input.Now

	candidates := make([]domain.CampaignMessageCandidate, 0, len(recipients))
	for _, r := range recipients {
		cid, err := s.idGen()
		if err != nil {
			log.Error("failed to generate candidate id", "error", err)
			return nil, err
		}
		candidates = append(candidates, domain.CampaignMessageCandidate{
			ID:              cid,
			WorkspaceID:     input.WorkspaceID,
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
			CreatedAt: input.Now,
			UpdatedAt: input.Now,
		})
	}

	audienceRefJSON, err := json.Marshal(campaign.AudienceRef)
	if err != nil {
		log.Error("failed to marshal audience ref", "error", err)
		return nil, err
	}

	eventID, err := s.idGen()
	if err != nil {
		log.Error("failed to generate event id", "error", err)
		return nil, err
	}
	outboxPayload := contracts.CampaignScheduledPayload{
		CampaignID:        campaign.ID,
		WorkspaceID:       input.WorkspaceID,
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
		WorkspaceID:   input.WorkspaceID,
		OccurredAt:    input.Now,
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

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.campaignsWrite.Update(txCtx, *campaign); err != nil {
			return err
		}
		if err := s.campaignsWrite.ReplaceCandidates(txCtx, input.WorkspaceID, campaign.ID, candidates); err != nil {
			return err
		}
		if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
			ID:            eventID,
			AggregateType: "campaign",
			AggregateID:   campaign.ID,
			EventType:     contracts.EventCampaignScheduledV1,
			Payload:       payloadBytes,
			WorkspaceID:   input.WorkspaceID,
			OccurredAt:    input.Now,
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Error("failed to execute schedule transaction", "error", err)
		return nil, err
	}

	log.Info("campaign scheduled", "planned_recipients", len(recipients))
	return &CampaignResult{Campaign: *campaign, CandidateCount: int64(len(candidates))}, nil
}

func (s *Service) CancelCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	log := s.log.With("usecase", "cancel_campaign", "workspace_id", workspaceID, "campaign_id", campaignID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "campaign.send"); err != nil {
		if errors.Is(err, domain.ErrSendDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrSendDenied
		}
		return nil, err
	}

	campaign, err := s.campaignsRead.FindByID(ctx, workspaceID, campaignID)
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
	campaign.CancelledAt = &now
	campaign.UpdatedAt = now

	if err := s.campaignsWrite.Update(ctx, *campaign); err != nil {
		log.Error("failed to cancel campaign", "error", err)
		return nil, err
	}

	log.Info("campaign cancelled")
	return &CampaignResult{Campaign: *campaign}, nil
}

func (s *Service) PauseCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	log := s.log.With("usecase", "pause_campaign", "workspace_id", workspaceID, "campaign_id", campaignID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "campaign.send"); err != nil {
		if errors.Is(err, domain.ErrSendDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrSendDenied
		}
		return nil, err
	}

	campaign, err := s.campaignsRead.FindByID(ctx, workspaceID, campaignID)
	if err != nil {
		if errors.Is(err, domain.ErrCampaignNotFound) {
			return nil, err
		}
		log.Error("failed to find campaign", "error", err)
		return nil, err
	}

	if !campaign.CanPause() {
		return nil, domain.ErrInvalidStateTransition
	}

	campaign.Status = domain.CampaignStatusPaused
	campaign.PausedAt = &now
	campaign.UpdatedAt = now

	if err := s.campaignsWrite.Update(ctx, *campaign); err != nil {
		log.Error("failed to pause campaign", "error", err)
		return nil, err
	}

	log.Info("campaign paused")
	return &CampaignResult{Campaign: *campaign}, nil
}

func (s *Service) ResumeCampaign(ctx context.Context, workspaceID, campaignID, userID string, now time.Time) (*CampaignResult, error) {
	log := s.log.With("usecase", "resume_campaign", "workspace_id", workspaceID, "campaign_id", campaignID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "campaign.send"); err != nil {
		if errors.Is(err, domain.ErrSendDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrSendDenied
		}
		return nil, err
	}

	campaign, err := s.campaignsRead.FindByID(ctx, workspaceID, campaignID)
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
	campaign.UpdatedAt = now

	if err := s.campaignsWrite.Update(ctx, *campaign); err != nil {
		log.Error("failed to resume campaign", "error", err)
		return nil, err
	}

	log.Info("campaign resumed")
	return &CampaignResult{Campaign: *campaign}, nil
}

func (s *Service) ListCampaignCandidates(ctx context.Context, input CandidateListInput, userID string) (*CandidateListResult, error) {
	log := s.log.With("usecase", "list_campaign_candidates", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, userID, "campaign.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if _, err := s.campaignsRead.FindByID(ctx, input.WorkspaceID, input.CampaignID); err != nil {
		if errors.Is(err, domain.ErrCampaignNotFound) {
			return nil, err
		}
		log.Error("failed to verify campaign", "error", err)
		return nil, err
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.CandidateListQuery{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		Status:      input.Status,
		Limit:       limit,
		Cursor:      input.Cursor,
	}

	candidates, cursor, err := s.campaignsRead.ListCandidates(ctx, query)
	if err != nil {
		log.Error("failed to list candidates", "error", err)
		return nil, err
	}

	return &CandidateListResult{Candidates: candidates, NextCursor: cursor}, nil
}
