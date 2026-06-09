package app

import (
	"context"
	"errors"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

type HandleCampaignScheduledInput struct {
	EventID   string
	EventType string
	Payload   []byte
	Now       time.Time
}

type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return e.Err.Error()
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

func (s *Service) HandleCampaignScheduled(ctx context.Context, input HandleCampaignScheduledInput) error {
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

	result, err := s.QueueCampaignMessages(ctx, QueueCampaignMessagesInput{
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

	s.log.Info("campaign scheduled handled",
		"workspace_id", payload.WorkspaceID,
		"campaign_id", payload.CampaignID,
		"queued_count", result.QueuedCount,
	)
	return nil
}
