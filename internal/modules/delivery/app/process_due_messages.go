package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type messageStatus int

const (
	messageStatusAccepted messageStatus = iota
	messageStatusFailed
	messageStatusRetryScheduled
)

type ProcessDueMessagesAllInput struct {
	MessageType string
	Limit       int
	Now         time.Time
}

func (s *Service) ProcessDueMessagesAllWorkspaces(ctx context.Context, input ProcessDueMessagesAllInput) (int, error) {
	log := s.log.With("usecase", "process_due_messages_all")

	if input.Now.IsZero() {
		return 0, domain.ErrPayloadInvalid
	}
	if !domain.ValidMessageType(input.MessageType) {
		return 0, domain.ErrPayloadInvalid
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	workspaces, err := s.messagesRead.ListDistinctWorkspacesWithDue(ctx, input.MessageType, input.Now)
	if err != nil {
		log.Error("failed to list distinct workspaces with due messages", "error", err)
		return 0, err
	}

	for _, ws := range workspaces {
		if _, err := s.ProcessDueMessages(ctx, ProcessDueMessagesInput{
			WorkspaceID: ws,
			MessageType: input.MessageType,
			Limit:       limit,
			Now:         input.Now,
		}); err != nil {
			log.Error("failed to process workspace", "workspace_id", ws, "error", err)
		}
	}

	return len(workspaces), nil
}

const (
	DefaultMaxRetries = 5
	baseRetryDelay    = 1 * time.Minute
	maxRetryDelay     = 1 * time.Hour
)

type ProcessDueMessagesInput struct {
	WorkspaceID string
	MessageType string
	Limit       int
	Now         time.Time
}

type ProcessDueMessagesResult struct {
	SelectedCount       int
	AcceptedCount       int
	FailedCount         int
	RetryScheduledCount int
}

func (s *Service) ProcessDueMessages(ctx context.Context, input ProcessDueMessagesInput) (*ProcessDueMessagesResult, error) {
	log := s.log.With("usecase", "process_due_messages", "workspace_id", input.WorkspaceID)

	if input.WorkspaceID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if input.Now.IsZero() {
		return nil, domain.ErrPayloadInvalid
	}
	if !domain.ValidMessageType(input.MessageType) {
		return nil, domain.ErrPayloadInvalid
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	messages, err := s.messagesRead.ListDueQueued(ctx, ports.DueMessageQuery{
		Now:         input.Now,
		Limit:       limit,
		MessageType: input.MessageType,
		WorkspaceID: input.WorkspaceID,
	})
	if err != nil {
		log.Error("failed to list due queued messages", "error", err)
		return nil, err
	}

	result := &ProcessDueMessagesResult{
		SelectedCount: len(messages),
	}

	for _, msg := range messages {
		status, err := s.ProcessMessage(ctx, msg, input.Now)
		if err != nil {
			log.Error("failed to process message",
				"message_id", msg.ID,
				"error", err,
			)
			continue
		}
		switch status {
		case messageStatusAccepted:
			result.AcceptedCount++
		case messageStatusFailed:
			result.FailedCount++
		case messageStatusRetryScheduled:
			result.RetryScheduledCount++
		}
	}

	log.Info("due messages processed",
		"selected_count", result.SelectedCount,
		"accepted_count", result.AcceptedCount,
		"failed_count", result.FailedCount,
		"retry_scheduled_count", result.RetryScheduledCount,
	)

	return result, nil
}

func (s *Service) ProcessMessage(ctx context.Context, msg domain.Message, now time.Time) (messageStatus, error) {
	log := s.log.With("usecase", "process_message",
		"workspace_id", msg.WorkspaceID,
		"message_id", msg.ID,
		"message_type", msg.MessageType,
	)

	now = now.UTC()

	err := s.messagesWrite.MarkProcessing(ctx, msg.WorkspaceID, msg.ID, now)
	if err != nil {
		if errors.Is(err, domain.ErrMessageNotFound) {
			log.Debug("message already claimed by another worker")
			return 0, nil
		}
		log.Error("failed to mark message processing", "error", err)
		return 0, err
	}

	readiness, err := s.senderChecker.GetSenderReadiness(ctx, msg.WorkspaceID, msg.SenderDomainID)
	if err != nil {
		log.Error("failed to check sender readiness", "sender_domain_id", msg.SenderDomainID, "error", err)
		s.failMessage(ctx, msg, now, "sender_not_ready", "sender readiness check failed")
		return messageStatusFailed, nil
	}
	if !readiness.Ready {
		log.Warn("sender not ready", "sender_domain_id", msg.SenderDomainID)
		s.failMessage(ctx, msg, now, "sender_not_ready", "sender domain not ready")
		return messageStatusFailed, nil
	}

	decision, err := s.suppressionChecker.CheckSuppression(ctx, msg.WorkspaceID, msg.RecipientEmailNormalized, "workspace")
	if err != nil {
		log.Error("failed to check suppression", "error", err)
		s.failMessage(ctx, msg, now, "suppression_check_failed", "suppression check failed")
		return messageStatusFailed, nil
	}
	if decision.Suppressed {
		log.Warn("recipient suppressed", "reason", decision.Reason)
		s.failMessage(ctx, msg, now, "recipient_suppressed", decision.Reason)
		return messageStatusFailed, nil
	}

	renderData := buildRenderData(msg.RecipientSnapshot)

	rendered, err := s.contentRenderer.RenderForMessage(ctx, msg.WorkspaceID, msg.TemplateID, msg.TemplateVersionID, renderData)
	if err != nil {
		log.Warn("template render failed", "template_id", msg.TemplateID, "template_version_id", msg.TemplateVersionID, "error", err)
		s.failMessage(ctx, msg, now, "template_render_failed", "template render failed")
		return messageStatusFailed, nil
	}

	attemptNo, err := s.attemptsRead.NextAttemptNumber(ctx, msg.WorkspaceID, msg.ID)
	if err != nil {
		log.Error("failed to get next attempt number", "error", err)
		s.failMessage(ctx, msg, now, "infrastructure_error", "failed to get attempt number")
		return messageStatusFailed, nil
	}

	attemptID := mustNewID(s.idGen)

	requestSnapshot := buildRequestSnapshot(msg, attemptNo)

	attempt := domain.DeliveryAttempt{
		ID:              attemptID,
		WorkspaceID:     msg.WorkspaceID,
		MessageID:       msg.ID,
		AttemptNo:       attemptNo,
		Provider:        "",
		Status:          domain.AttemptStatusStarted,
		RequestSnapshot: requestSnapshot,
		StartedAt:       now,
	}

	if err := s.attemptsWrite.Create(ctx, attempt); err != nil {
		log.Error("failed to create delivery attempt", "error", err)
		s.failMessage(ctx, msg, now, "infrastructure_error", "failed to create attempt")
		return messageStatusFailed, nil
	}

	providerResult, providerErr := s.emailProvider.SendEmail(ctx, ports.ProviderSendRequest{
		To:       msg.RecipientSnapshot.Email,
		Subject:  rendered.Subject,
		HTMLBody: rendered.HTMLBody,
		TextBody: rendered.TextBody,
	})

	if providerErr != nil {
		log.Error("provider send failed", "error", providerErr)

		if errors.Is(providerErr, domain.ErrProviderPermanentFailure) {
			s.failMessageWithAttempt(ctx, msg, attempt, now, "provider_permanent_failure", providerErr.Error())
			return messageStatusFailed, nil
		}

		s.handleTemporaryFailure(ctx, msg, attempt, now, providerErr)
		return messageStatusRetryScheduled, nil
	}

	if providerResult.ProviderMessageID == "" {
		log.Error("provider returned empty provider_message_id")
		s.failMessageWithAttempt(ctx, msg, attempt, now, "provider_invalid_response", "empty provider_message_id")
		return messageStatusFailed, nil
	}

	responseSnapshot, _ := json.Marshal(map[string]string{
		"provider":            providerResult.Provider,
		"provider_message_id": providerResult.ProviderMessageID,
	})

	attempt.Status = domain.AttemptStatusAccepted
	attempt.Provider = providerResult.Provider
	attempt.ResponseSnapshot = responseSnapshot
	attempt.FinishedAt = &now

	msg.UpdatedAt = now
	msg.Provider = providerResult.Provider
	msg.ProviderMessageID = providerResult.ProviderMessageID
	msg.AcceptedAt = &now

	if err := s.persistAcceptedState(ctx, msg, attempt, providerResult, now); err != nil {
		log.Error("failed to persist accepted state after retries, moving to dlq", "error", err)

		current, readErr := s.messagesRead.FindByID(context.Background(), msg.WorkspaceID, msg.ID)
		if readErr == nil && current.Status == domain.MessageStatusAccepted {
			log.Warn("message was already committed as accepted on a prior attempt")
			return messageStatusAccepted, nil
		}

		msg.Status = domain.MessageStatusDLQ
		msg.LastErrorClass = "accepted_state_persistence_failed"
		msg.LastErrorMessage = err.Error()
		msg.UpdatedAt = now

		if dlqErr := s.messagesWrite.Update(context.Background(), msg); dlqErr != nil {
			log.Error("failed to move message to dlq after accepted save failure", "dlq_error", dlqErr)
		}
		return messageStatusFailed, nil
	}

	log.Info("message accepted by provider",
		"provider", providerResult.Provider,
		"provider_message_id", providerResult.ProviderMessageID,
	)
	return messageStatusAccepted, nil
}

func buildRenderData(snapshot domain.RecipientSnapshot) map[string]any {
	data := map[string]any{
		"first_name": snapshot.FirstName,
		"last_name":  snapshot.LastName,
		"email":      snapshot.Email,
	}
	if len(snapshot.Tags) > 0 {
		data["tags"] = snapshot.Tags
	}
	if snapshot.Attributes != nil {
		data["attributes"] = snapshot.Attributes
	}
	if snapshot.TemplateData != nil {
		for k, v := range snapshot.TemplateData {
			data[k] = v
		}
	}
	return data
}

func buildRequestSnapshot(msg domain.Message, attemptNo int) json.RawMessage {
	snapshot := map[string]any{
		"attempt_no":       attemptNo,
		"message_id":       msg.ID,
		"template_id":      msg.TemplateID,
		"sender_domain_id": msg.SenderDomainID,
	}
	raw, _ := json.Marshal(snapshot)
	return raw
}

func (s *Service) failMessage(ctx context.Context, msg domain.Message, now time.Time, errorClass, errorMessage string) {
	msg.FailedAt = &now
	msg.LastErrorClass = errorClass
	msg.LastErrorMessage = errorMessage
	msg.UpdatedAt = now

	if err := s.txManager.RunInTransaction(ctx, func(txCtx context.Context) error {
		return s.messagesWrite.MarkFailed(txCtx, msg)
	}); err != nil {
		s.log.Error("failed to mark message failed",
			"message_id", msg.ID,
			"error_class", errorClass,
			"error", err,
		)
	}
}

func (s *Service) failMessageWithAttempt(ctx context.Context, msg domain.Message, attempt domain.DeliveryAttempt, now time.Time, errorClass, errorMessage string) {
	attempt.Status = domain.AttemptStatusPermanentFailed
	attempt.ErrorClass = errorClass
	attempt.ErrorMessage = errorMessage
	attempt.FinishedAt = &now

	msg.FailedAt = &now
	msg.LastErrorClass = errorClass
	msg.LastErrorMessage = errorMessage
	msg.UpdatedAt = now

	if err := s.txManager.RunInTransaction(ctx, func(txCtx context.Context) error {
		if err := s.attemptsWrite.Update(txCtx, attempt); err != nil {
			return err
		}
		return s.messagesWrite.MarkFailed(txCtx, msg)
	}); err != nil {
		s.log.Error("failed to mark message failed with attempt",
			"message_id", msg.ID,
			"error_class", errorClass,
			"error", err,
		)
	}
}

func (s *Service) handleTemporaryFailure(ctx context.Context, msg domain.Message, attempt domain.DeliveryAttempt, now time.Time, providerErr error) {
	log := s.log.With("message_id", msg.ID)

	attempt.Status = domain.AttemptStatusTemporaryFailed
	attempt.ErrorClass = "provider_temporary_failure"
	attempt.ErrorMessage = providerErr.Error()
	attempt.FinishedAt = &now

	retryState, err := s.retryStatesRead.FindByMessage(ctx, msg.WorkspaceID, msg.ID)
	if err != nil {
		log.Error("failed to find retry state", "error", err)
		s.failMessageWithAttempt(ctx, msg, attempt, now, "retry_state_error", "failed to find retry state")
		return
	}

	currentRetryCount := 0
	if retryState != nil {
		currentRetryCount = retryState.RetryCount
	}

	if currentRetryCount >= DefaultMaxRetries {
		log.Warn("retry exhausted, marking failed", "retry_count", currentRetryCount)
		if retryState != nil {
			retryState.Status = domain.RetryStatusExhausted
			retryState.UpdatedAt = now
		}
		s.failMessageWithAttempt(ctx, msg, attempt, now, "retry_exhausted", "max retries reached")
		return
	}

	nextDelay := computeRetryDelay(currentRetryCount)
	nextAttemptAt := now.Add(nextDelay)

	msg.Status = domain.MessageStatusQueued
	msg.ScheduledAt = &nextAttemptAt
	msg.UpdatedAt = now

	newRetry := retryState == nil
	if newRetry {
		retryState = &domain.RetryState{
			ID:               mustNewID(s.idGen),
			WorkspaceID:      msg.WorkspaceID,
			MessageID:        msg.ID,
			RetryCount:       1,
			MaxRetries:       DefaultMaxRetries,
			NextAttemptAt:    &nextAttemptAt,
			LastErrorClass:   "provider_temporary_failure",
			LastErrorMessage: providerErr.Error(),
			Status:           domain.RetryStatusScheduled,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	} else {
		retryState.RetryCount = currentRetryCount + 1
		retryState.NextAttemptAt = &nextAttemptAt
		retryState.LastErrorClass = "provider_temporary_failure"
		retryState.LastErrorMessage = providerErr.Error()
		retryState.Status = domain.RetryStatusScheduled
		retryState.UpdatedAt = now
	}

	if err := s.txManager.RunInTransaction(ctx, func(txCtx context.Context) error {
		if err := s.messagesWrite.Update(txCtx, msg); err != nil {
			return err
		}
		if err := s.attemptsWrite.Update(txCtx, attempt); err != nil {
			return err
		}
		if newRetry {
			if err := s.retryStatesWrite.Create(txCtx, *retryState); err != nil {
				return err
			}
		} else {
			if err := s.retryStatesWrite.Update(txCtx, *retryState); err != nil {
				return err
			}
		}
		return s.saveRetryScheduledEvent(txCtx, msg, retryState, now)
	}); err != nil {
		log.Error("failed to save retry state", "error", err)
		return
	}

	log.Info("retry scheduled",
		"retry_count", retryState.RetryCount,
		"next_attempt_at", nextAttemptAt,
	)
}

func computeRetryDelay(retryCount int) time.Duration {
	delay := float64(baseRetryDelay) * math.Pow(2, float64(retryCount))
	if delay > float64(maxRetryDelay) {
		delay = float64(maxRetryDelay)
	}
	jitter := rand.Float64() * delay * 0.1
	return time.Duration(delay + jitter)
}

func (s *Service) persistAcceptedState(ctx context.Context, msg domain.Message, attempt domain.DeliveryAttempt, providerResult *ports.ProviderSendResult, now time.Time) error {
	eventID := mustNewID(s.idGen)
	var lastErr error
	for i := 0; i < 3; i++ {
		txCtx := ctx
		var cancel context.CancelFunc
		if i > 0 {
			txCtx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		}

		err := s.txManager.RunInTransaction(txCtx, func(txCtx context.Context) error {
			if err := s.attemptsWrite.Update(txCtx, attempt); err != nil {
				return err
			}
			if err := s.messagesWrite.MarkAccepted(txCtx, msg); err != nil {
				return err
			}
			return s.saveAcceptedEvent(txCtx, msg, providerResult, now, eventID)
		})
		if cancel != nil {
			cancel()
		}
		if err == nil {
			return nil
		}
		lastErr = err

		if i < 2 {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
		}
	}
	return fmt.Errorf("persist accepted state after 3 attempts: %w", lastErr)
}

func (s *Service) saveAcceptedEvent(ctx context.Context, msg domain.Message, providerResult *ports.ProviderSendResult, now time.Time, eventID string) error {
	payload := contracts.MessageAcceptedPayload{
		MessageID:              msg.ID,
		WorkspaceID:            msg.WorkspaceID,
		CampaignID:             msg.CampaignID,
		CampaignCandidateID:    msg.CampaignCandidateID,
		TransactionalRequestID: msg.TransactionalRequestID,
		TemplateID:             msg.TemplateID,
		TemplateVersionID:      msg.TemplateVersionID,
		SenderDomainID:         msg.SenderDomainID,
		MessageType:            msg.MessageType,
		SourceType:             msg.SourceType,
		Provider:               providerResult.Provider,
		ProviderMessageID:      providerResult.ProviderMessageID,
		AcceptedAt:             now.Format(time.RFC3339),
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventDeliveryMessageAcceptedV1,
		EventVersion:  1,
		AggregateType: "message",
		AggregateID:   msg.ID,
		WorkspaceID:   msg.WorkspaceID,
		OccurredAt:    now,
	}, payload)
	if err != nil {
		return err
	}

	payloadBytes, err := events.Marshal(envelope)
	if err != nil {
		return err
	}

	return s.outboxWriter.Save(ctx, ports.OutboxEvent{
		ID:            envelope.EventID,
		AggregateType: "message",
		AggregateID:   msg.ID,
		EventType:     contracts.EventDeliveryMessageAcceptedV1,
		Payload:       payloadBytes,
		WorkspaceID:   msg.WorkspaceID,
		OccurredAt:    now,
	})
}

func (s *Service) saveRetryScheduledEvent(ctx context.Context, msg domain.Message, retryState *domain.RetryState, now time.Time) error {
	eventID := mustNewID(s.idGen)

	payload := map[string]any{
		"message_id":      msg.ID,
		"workspace_id":    msg.WorkspaceID,
		"retry_count":     retryState.RetryCount,
		"max_retries":     retryState.MaxRetries,
		"next_attempt_at": retryState.NextAttemptAt.Format(time.RFC3339),
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventDeliveryMessageRetryScheduledV1,
		EventVersion:  1,
		AggregateType: "message",
		AggregateID:   msg.ID,
		WorkspaceID:   msg.WorkspaceID,
		OccurredAt:    now,
	}, payload)
	if err != nil {
		return err
	}

	payloadBytes, err := events.Marshal(envelope)
	if err != nil {
		return err
	}

	return s.outboxWriter.Save(ctx, ports.OutboxEvent{
		ID:            envelope.EventID,
		AggregateType: "message",
		AggregateID:   msg.ID,
		EventType:     contracts.EventDeliveryMessageRetryScheduledV1,
		Payload:       payloadBytes,
		WorkspaceID:   msg.WorkspaceID,
		OccurredAt:    now,
	})
}
