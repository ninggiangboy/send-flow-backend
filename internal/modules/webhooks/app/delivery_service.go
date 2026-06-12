package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	webhookscontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type DeliverWebhookInput struct {
	WorkspaceID     string
	WebhookID       string
	DeliveryID      string
	TargetURL       string
	SecretHash      string
	EventPayload    map[string]any
	SourceEventID   string
	SourceEventType string
	AttemptNumber   int64
}

type DeliverWebhookResult struct {
	Success        bool
	StatusCode     int
	Error          string
	DurationMs     int64
	Attempt        domain.WebhookDeliveryAttempt
	DeliveryResult domain.DeliveryResult
}

func (s *Service) DeliverWebhook(ctx context.Context, input DeliverWebhookInput) (*DeliverWebhookResult, error) {
	now := s.clock()
	timestamp := fmt.Sprintf("%d", now.Unix())

	payloadJSON, err := json.Marshal(input.EventPayload)
	if err != nil {
		return nil, err
	}

	signature := SignPayloadRaw(payloadJSON, timestamp, input.SecretHash)

	deliveryReq := ports.DeliveryHTTPRequest{
		URL:             input.TargetURL,
		Body:            payloadJSON,
		SignatureHeader: "Sendflow-Signature",
		SignatureValue:  signature,
		TimestampHeader: "Sendflow-Timestamp",
		TimestampValue:  timestamp,
		EventIDHeader:   "Sendflow-Event-ID",
		EventIDValue:    input.SourceEventID,
	}

	resp, deliverErr := s.deliverer.Deliver(ctx, deliveryReq)

	attemptID, err := s.idGen()
	if err != nil {
		return nil, err
	}

	isSuccess := deliverErr == nil && resp.StatusCode >= 200 && resp.StatusCode < 300

	var attempt domain.WebhookDeliveryAttempt
	var deliveryResult domain.DeliveryResult

	if deliverErr != nil {
		attempt = domain.WebhookDeliveryAttempt{
			ID:             attemptID,
			DeliveryID:     input.DeliveryID,
			AttemptNumber:  input.AttemptNumber,
			Status:         string(domain.DeliveryStatusFailed),
			Error:          domain.SanitizeError(deliverErr.Error()),
			RequestHeaders: map[string]string{},
			AttemptedAt:    now,
		}
		deliveryResult = domain.DeliveryResult{
			Error:          domain.SanitizeError(deliverErr.Error()),
			RequestHeaders: map[string]string{},
		}
	} else {
		status := string(domain.DeliveryStatusFailed)
		if isSuccess {
			status = string(domain.DeliveryStatusSucceeded)
		}
		attempt = domain.WebhookDeliveryAttempt{
			ID:              attemptID,
			DeliveryID:      input.DeliveryID,
			AttemptNumber:   input.AttemptNumber,
			Status:          status,
			StatusCode:      &resp.StatusCode,
			Error:           domain.SanitizeError(resp.Error),
			DurationMs:      resp.DurationMs,
			RequestHeaders:  map[string]string{},
			ResponseHeaders: domain.SanitizeHeaders(resp.Headers),
			AttemptedAt:     now,
		}
		if resp.StatusCode == 0 {
			attempt.StatusCode = nil
		}
		deliveryResult = domain.DeliveryResult{
			StatusCode:      resp.StatusCode,
			DurationMs:      resp.DurationMs,
			Error:           domain.SanitizeError(resp.Error),
			RequestHeaders:  map[string]string{},
			ResponseHeaders: domain.SanitizeHeaders(resp.Headers),
		}
	}

	return &DeliverWebhookResult{
		Success:        isSuccess,
		StatusCode:     resp.StatusCode,
		Error:          attempt.Error,
		DurationMs:     attempt.DurationMs,
		Attempt:        attempt,
		DeliveryResult: deliveryResult,
	}, nil
}

type ListDeliveriesInput struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
	Status      string
	EventType   string
	From        *time.Time
	To          *time.Time
	Limit       int
	Cursor      string
}

type ListDeliveriesResult struct {
	Deliveries []domain.WebhookDelivery
	NextCursor string
}

func (s *Service) ListWebhookDeliveries(ctx context.Context, input ListDeliveriesInput) (*ListDeliveriesResult, error) {
	log := s.log.With("usecase", "list_webhook_deliveries", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.delivery.read"); err != nil {
		return nil, err
	}

	filter := ports.DeliveryFilter{
		WebhookID: input.WebhookID,
		Status:    input.Status,
		EventType: input.EventType,
		From:      input.From,
		To:        input.To,
		Limit:     input.Limit,
		Cursor:    input.Cursor,
	}

	deliveries, cursor, err := s.deliveryRead.ListByWorkspace(ctx, input.WorkspaceID, filter)
	if err != nil {
		log.Error("failed to list deliveries", "error", err)
		return nil, err
	}

	return &ListDeliveriesResult{Deliveries: deliveries, NextCursor: cursor}, nil
}

type GetDeliveryInput struct {
	WorkspaceID string
	UserID      string
	DeliveryID  string
}

func (s *Service) GetWebhookDelivery(ctx context.Context, input GetDeliveryInput) (*domain.WebhookDelivery, error) {
	log := s.log.With("usecase", "get_webhook_delivery", "workspace_id", input.WorkspaceID, "delivery_id", input.DeliveryID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.delivery.read"); err != nil {
		return nil, err
	}

	delivery, err := s.deliveryRead.FindByID(ctx, input.WorkspaceID, input.DeliveryID)
	if err != nil {
		if errors.Is(err, domain.ErrDeliveryNotFound) {
			return nil, err
		}
		log.Error("failed to find delivery", "error", err)
		return nil, err
	}

	attempts, err := s.attemptRead.ListByDelivery(ctx, input.DeliveryID)
	if err != nil {
		log.Error("failed to list delivery attempts", "error", err)
	}
	delivery.Attempts = attempts

	return delivery, nil
}

type RetryDeliveryInput struct {
	WorkspaceID string
	UserID      string
	DeliveryID  string
}

func (s *Service) RetryWebhookDelivery(ctx context.Context, input RetryDeliveryInput) error {
	log := s.log.With("usecase", "retry_webhook_delivery", "workspace_id", input.WorkspaceID, "delivery_id", input.DeliveryID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.delivery.retry"); err != nil {
		return err
	}

	delivery, err := s.deliveryRead.FindByID(ctx, input.WorkspaceID, input.DeliveryID)
	if err != nil {
		if errors.Is(err, domain.ErrDeliveryNotFound) {
			return err
		}
		log.Error("failed to find delivery for retry", "error", err)
		return err
	}

	if !domain.CanDeliveryBeRetried(delivery.Status) {
		return domain.ErrRetryConflict
	}

	cfg, err := s.configRead.FindByID(ctx, input.WorkspaceID, delivery.WebhookID)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return domain.ErrConfigNotFound
		}
		log.Error("failed to find webhook config for retry", "error", err)
		return err
	}

	if cfg.Status != domain.ConfigStatusActive {
		return domain.ErrConfigDisabled
	}

	now := s.clock()
	nextAttempt := delivery.AttemptCount + 1

	if err := s.deliveryWrite.MarkDelivering(ctx, input.DeliveryID, now); err != nil {
		if errors.Is(err, domain.ErrDeliveryNotFound) {
			return err
		}
		log.Error("failed to mark delivery as delivering for retry", "error", err)
		return err
	}

	result, err := s.DeliverWebhook(ctx, DeliverWebhookInput{
		WorkspaceID:     input.WorkspaceID,
		WebhookID:       cfg.ID,
		DeliveryID:      input.DeliveryID,
		TargetURL:       cfg.TargetURL,
		SecretHash:      cfg.SecretHash,
		EventPayload:    delivery.EventPayloadJSON,
		SourceEventID:   delivery.SourceEventID,
		SourceEventType: delivery.SourceEventType,
		AttemptNumber:   nextAttempt,
	})
	if err != nil {
		log.Error("retry delivery failed", "error", err)
		return err
	}

	usesOutbox := s.outboxWriter != nil
	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if result.Success {
			if err := s.deliveryWrite.MarkSucceeded(txCtx, input.DeliveryID, result.DeliveryResult); err != nil {
				return err
			}
			if err := s.attemptWrite.Create(txCtx, result.Attempt); err != nil {
				return err
			}
			if usesOutbox {
				eventID, err := s.idGen()
				if err != nil {
					return err
				}
				payload, _ := json.Marshal(webhookscontracts.DeliverySucceededPayload{
					DeliveryID:      input.DeliveryID,
					WorkspaceID:     input.WorkspaceID,
					WebhookID:       cfg.ID,
					SourceEventID:   delivery.SourceEventID,
					SourceEventType: delivery.SourceEventType,
					StatusCode:      result.StatusCode,
					DurationMs:      result.DurationMs,
				})
				return s.outboxWriter.Save(txCtx, ports.OutboxEvent{
					ID:            eventID,
					AggregateType: "webhook_delivery",
					AggregateID:   input.DeliveryID,
					EventType:     webhookscontracts.EventDeliverySucceededV1,
					Payload:       payload,
					WorkspaceID:   input.WorkspaceID,
					OccurredAt:    now,
				})
			}
			return nil
		}

		if err := s.deliveryWrite.MarkFailed(txCtx, input.DeliveryID, result.DeliveryResult); err != nil {
			return err
		}
		if err := s.attemptWrite.Create(txCtx, result.Attempt); err != nil {
			return err
		}
		if usesOutbox {
			eventID, err := s.idGen()
			if err != nil {
				return err
			}
			failedPayload := webhookscontracts.DeliveryFailedPayload{
				DeliveryID:      input.DeliveryID,
				WorkspaceID:     input.WorkspaceID,
				WebhookID:       cfg.ID,
				SourceEventID:   delivery.SourceEventID,
				SourceEventType: delivery.SourceEventType,
				Error:           result.Error,
				DurationMs:      result.DurationMs,
			}
			if result.Attempt.StatusCode != nil {
				failedPayload.StatusCode = result.Attempt.StatusCode
			}
			payload, _ := json.Marshal(failedPayload)
			return s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_delivery",
				AggregateID:   input.DeliveryID,
				EventType:     webhookscontracts.EventDeliveryFailedV1,
				Payload:       payload,
				WorkspaceID:   input.WorkspaceID,
				OccurredAt:    now,
			})
		}
		return nil
	})
}

func SignPayloadRaw(payload []byte, timestamp, signingKey string) string {
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(timestamp))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
