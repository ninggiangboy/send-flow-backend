package handlecampaignscheduled

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/queuecampaignmessages"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

type Input struct {
	EventID   string
	EventType string
	Payload   []byte
	Now       time.Time
}

type NonRetryableError = usecase.NonRetryableError

type Handler struct {
	queueCampaignMessages *queuecampaignmessages.Handler
	log                   *slog.Logger
}

func New(queueCampaignMessages *queuecampaignmessages.Handler, logger *slog.Logger) *Handler {
	return &Handler{queueCampaignMessages: queueCampaignMessages, log: logger.With("usecase", "handle_campaign_scheduled")}
}

func (h *Handler) Execute(ctx context.Context, input Input) error {
	if input.EventID == "" {
		return &NonRetryableError{Err: domain.ErrPayloadInvalid}
	}

	payload, err := contracts.ParseCampaignScheduledPayload(input.EventType, input.Payload)
	if err != nil {
		return &NonRetryableError{Err: err}
	}

	scheduledAt, err := time.Parse(time.RFC3339, payload.ScheduledAt)
	if err != nil {
		return &NonRetryableError{Err: domain.ErrCampaignScheduleInvalid}
	}

	result, err := h.queueCampaignMessages.Execute(ctx, queuecampaignmessages.Input{
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
			return &NonRetryableError{Err: err}
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
