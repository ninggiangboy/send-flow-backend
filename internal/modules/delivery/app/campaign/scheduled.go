package campaign

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

type CampaignScheduledInput struct {
	EventID   string
	EventType string
	Payload   []byte
	Now       time.Time
}

type CampaignScheduledHandler struct {
	queueCampaignMessages *QueueCampaignMessagesHandler
	log                   *slog.Logger
}

func NewCampaignScheduledHandler(queueCampaignMessages *QueueCampaignMessagesHandler, logger *slog.Logger) *CampaignScheduledHandler {
	return &CampaignScheduledHandler{queueCampaignMessages: queueCampaignMessages, log: logger.With("usecase", "handle_campaign_scheduled")}
}

func (h *CampaignScheduledHandler) Execute(ctx context.Context, input CampaignScheduledInput) error {
	if input.EventID == "" {
		return &shared.NonRetryableError{Err: domain.ErrPayloadInvalid}
	}

	payload, err := contracts.ParseCampaignScheduledPayload(input.EventType, input.Payload)
	if err != nil {
		return &shared.NonRetryableError{Err: err}
	}

	scheduledAt, err := time.Parse(time.RFC3339, payload.ScheduledAt)
	if err != nil {
		return &shared.NonRetryableError{Err: domain.ErrCampaignScheduleInvalid}
	}

	result, err := h.queueCampaignMessages.Execute(ctx, QueueCampaignMessagesInput{
		WorkspaceID:       payload.WorkspaceID,
		CampaignID:        payload.CampaignID,
		TemplateID:        payload.TemplateID,
		TemplateVersionID: payload.TemplateVersionID,
		SenderDomainID:    payload.SenderDomainID,
		MessageType:       payload.MessageType,
		ScheduledAt:       scheduledAt,
		Now:               input.Now,
	})
	if err != nil {
		if errors.Is(err, domain.ErrCampaignCandidatesNotFound) {
			return err
		}
		if errors.Is(err, domain.ErrPayloadInvalid) {
			return &shared.NonRetryableError{Err: err}
		}
		return err
	}

	h.log.Info("campaign scheduled handled",
		"workspace_id", payload.WorkspaceID,
		"campaign_id", payload.CampaignID,
		"queued_count", result.QueuedCount,
	)
	return nil
}
