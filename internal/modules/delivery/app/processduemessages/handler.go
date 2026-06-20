package processduemessages

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	deliveryredis "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/batching"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type messageStatus int

const (
	messageStatusAccepted messageStatus = iota
	messageStatusFailed
	messageStatusRetryScheduled
)

const (
	DefaultMaxRetries = 5
	baseRetryDelay    = 1 * time.Minute
	maxRetryDelay     = 1 * time.Hour
)

type ProcessDueMessagesAllInput struct {
	MessageType string
	Limit       int
	Now         time.Time
}

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

type Handler struct {
	messagesWrite      ports.MessageWriteRepository
	attemptsWrite      ports.AttemptWriteRepository
	retryStatesWrite   ports.RetryStateWriteRepository
	senderChecker      ports.SenderReadinessChecker
	suppressionChecker ports.SuppressionChecker
	contentRenderer    ports.ContentRenderer
	emailProvider      ports.EmailProvider
	outboxWriter       ports.OutboxWriter
	txManager          ports.UnitOfWork
	txRequestsWrite    ports.TransactionalRequestWriteRepository
	attachmentRepo     ports.AttachmentRepository
	objectStorage      ports.ObjectStorage
	idGen              func() (string, error)
	log                *slog.Logger
	cache              *deliveryredis.Cache
	eventRepo          ports.MessageEventRepository
}

func New(
	messagesWrite ports.MessageWriteRepository,
	attemptsWrite ports.AttemptWriteRepository,
	retryStatesWrite ports.RetryStateWriteRepository,
	senderChecker ports.SenderReadinessChecker,
	suppressionChecker ports.SuppressionChecker,
	contentRenderer ports.ContentRenderer,
	emailProvider ports.EmailProvider,
	outboxWriter ports.OutboxWriter,
	txManager ports.UnitOfWork,
	txRequestsWrite ports.TransactionalRequestWriteRepository,
	idGen func() (string, error),
	logger *slog.Logger,
	cache *deliveryredis.Cache,
	eventRepo ports.MessageEventRepository,
	attachmentRepo ports.AttachmentRepository,
	objectStorage ports.ObjectStorage,
) *Handler {
	return &Handler{
		messagesWrite:      messagesWrite,
		attemptsWrite:      attemptsWrite,
		retryStatesWrite:   retryStatesWrite,
		senderChecker:      senderChecker,
		suppressionChecker: suppressionChecker,
		contentRenderer:    contentRenderer,
		emailProvider:      emailProvider,
		outboxWriter:       outboxWriter,
		txManager:          txManager,
		txRequestsWrite:    txRequestsWrite,
		attachmentRepo:     attachmentRepo,
		objectStorage:      objectStorage,
		idGen:              idGen,
		log:                logger.With("usecase", "process_due_messages"),
		cache:              cache,
		eventRepo:          eventRepo,
	}
}

func (h *Handler) ProcessDueMessagesAllWorkspaces(ctx context.Context, input ProcessDueMessagesAllInput) (int, error) {
	log := h.log.With("usecase", "process_due_messages_all")

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

	workspaces, err := h.messagesWrite.ListDistinctWorkspacesWithDue(ctx, input.MessageType, input.Now)
	if err != nil {
		log.Error("failed to list distinct workspaces with due messages", "error", err)
		return 0, err
	}

	for _, ws := range workspaces {
		if _, err := h.ProcessDueMessages(ctx, ProcessDueMessagesInput{
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

func (h *Handler) ProcessDueMessages(ctx context.Context, input ProcessDueMessagesInput) (*ProcessDueMessagesResult, error) {
	log := h.log.With("usecase", "process_due_messages", "workspace_id", input.WorkspaceID)

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

	messages, err := h.messagesWrite.ClaimDueMessages(ctx, ports.DueMessageQuery{
		Now:         input.Now,
		Limit:       limit,
		MessageType: input.MessageType,
		WorkspaceID: input.WorkspaceID,
	}, input.Now)
	if err != nil {
		log.Error("failed to claim due messages", "error", err)
		return nil, err
	}

	result := &ProcessDueMessagesResult{
		SelectedCount: len(messages),
	}

	idx := 0
	pipeline, err := batching.NewPipeline[domain.Message, messageStatus](batching.Config[domain.Message, messageStatus]{
		Options: batching.Options{
			BufferedItemsSize:    limit,
			WriteBatchSize:       limit,
			ProcessorConcurrency: 1,
			MaxInflight:          limit,
		},
		Reader: batching.ItemReaderFunc[domain.Message](func(ctx context.Context, _ int, _ int) (domain.Message, bool, error) {
			if idx >= len(messages) {
				return domain.Message{}, false, nil
			}
			msg := messages[idx]
			idx++
			return msg, true, nil
		}),
		Processor: batching.ItemProcessorFunc[domain.Message, messageStatus](func(ctx context.Context, msg domain.Message) (messageStatus, bool, error) {
			status, err := h.processMessage(ctx, msg, input.Now)
			if err != nil {
				log.Error("failed to process message",
					"message_id", msg.ID,
					"error", err,
				)
				return 0, false, nil
			}
			return status, true, nil
		}),
		Writer: batching.ItemWriterFunc[messageStatus](func(ctx context.Context, statuses []messageStatus) error {
			for _, status := range statuses {
				switch status {
				case messageStatusAccepted:
					result.AcceptedCount++
				case messageStatusFailed:
					result.FailedCount++
				case messageStatusRetryScheduled:
					result.RetryScheduledCount++
				}
			}
			return nil
		}),
	})
	if err != nil {
		return nil, err
	}
	if err := pipeline.Run(ctx); err != nil {
		return nil, err
	}

	log.Info("due messages processed",
		"selected_count", result.SelectedCount,
		"accepted_count", result.AcceptedCount,
		"failed_count", result.FailedCount,
		"retry_scheduled_count", result.RetryScheduledCount,
	)

	return result, nil
}

func (h *Handler) processMessage(ctx context.Context, msg domain.Message, now time.Time) (messageStatus, error) {
	log := h.log.With("usecase", "process_message",
		"workspace_id", msg.WorkspaceID,
		"message_id", msg.ID,
		"message_type", msg.MessageType,
	)

	now = now.UTC()

	h.writeEvent(ctx, msg, domain.MessageEventProcessingStarted, domain.MessageStatusProcessing, "", "", now)

	if h.cache != nil {
		token, acquired, err := h.cache.AcquireMessageLock(ctx, msg.ID, 30*time.Second)
		if err != nil {
			log.Error("failed to acquire message lock", "error", err)
			return 0, err
		}
		if !acquired {
			log.Warn("message locked by another worker, skipping")
			return messageStatusFailed, nil
		}
		defer func() {
			if _, releaseErr := h.cache.ReleaseMessageLock(ctx, msg.ID, token); releaseErr != nil {
				log.Error("failed to release message lock", "error", releaseErr)
			}
		}()
	}

	readiness, err := h.senderChecker.GetSenderReadiness(ctx, msg.WorkspaceID, msg.SenderDomainID)
	if err != nil {
		log.Error("failed to check sender readiness", "sender_domain_id", msg.SenderDomainID, "error", err)
		h.failMessage(ctx, msg, now, "sender_not_ready", "sender readiness check failed")
		return messageStatusFailed, nil
	}
	if !readiness.Ready {
		log.Warn("sender not ready", "sender_domain_id", msg.SenderDomainID)
		h.failMessage(ctx, msg, now, "sender_not_ready", "sender domain not ready")
		return messageStatusFailed, nil
	}

	decision, err := h.suppressionChecker.CheckSuppression(ctx, msg.WorkspaceID, msg.RecipientEmailNormalized, "workspace")
	if err != nil {
		log.Error("failed to check suppression", "error", err)
		h.failMessage(ctx, msg, now, "suppression_check_failed", "suppression check failed")
		return messageStatusFailed, nil
	}
	if decision.Suppressed {
		log.Warn("recipient suppressed", "reason", decision.Reason)
		h.failMessage(ctx, msg, now, "recipient_suppressed", decision.Reason)
		return messageStatusFailed, nil
	}

	var subject, htmlBody, textBody string

	if msg.TemplateID != "" {
		// Template mode: render from template
		renderData := buildRenderData(msg.RecipientSnapshot)
		rendered, err := h.contentRenderer.RenderForMessage(ctx, msg.WorkspaceID, msg.TemplateID, msg.TemplateVersionID, renderData)
		if err != nil {
			log.Warn("template render failed", "template_id", msg.TemplateID, "template_version_id", msg.TemplateVersionID, "error", err)
			h.failMessage(ctx, msg, now, "template_render_failed", "template render failed")
			return messageStatusFailed, nil
		}
		subject = rendered.Subject
		htmlBody = rendered.HTMLBody
		textBody = rendered.TextBody
	} else {
		// Raw mode: use stored body directly
		subject = msg.Subject
		htmlBody = msg.HTMLBody
		textBody = msg.TextBody
	}

	attemptNo, err := h.attemptsWrite.NextAttemptNumber(ctx, msg.WorkspaceID, msg.ID)
	if err != nil {
		log.Error("failed to get next attempt number", "error", err)
		h.failMessage(ctx, msg, now, "infrastructure_error", "failed to get attempt number")
		return messageStatusFailed, nil
	}

	attemptID, err := h.idGen()
	if err != nil {
		log.Error("failed to generate attempt ID", "error", err)
		h.failMessage(ctx, msg, now, "infrastructure_error", "failed to generate attempt ID")
		return messageStatusFailed, nil
	}

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

	if err := h.attemptsWrite.Create(ctx, attempt); err != nil {
		log.Error("failed to create delivery attempt", "error", err)
		h.failMessage(ctx, msg, now, "infrastructure_error", "failed to create attempt")
		return messageStatusFailed, nil
	}

	attachments, attErr := h.loadAttachments(ctx, msg)
	if attErr != nil {
		log.Error("failed to load attachments", "error", attErr)
		h.failMessage(ctx, msg, now, "attachment_load_failed", attErr.Error())
		return messageStatusFailed, nil
	}

	var replyToList []string
	if msg.ReplyTo != "" {
		replyToList = []string{msg.ReplyTo}
	}

	providerResult, providerErr := h.emailProvider.SendEmail(ctx, ports.ProviderSendRequest{
		To:          []string{msg.RecipientSnapshot.Email},
		Subject:     subject,
		HTMLBody:    htmlBody,
		TextBody:    textBody,
		SenderName:  msg.SenderName,
		ReplyTo:     replyToList,
		Headers:     msg.Headers,
		Attachments: attachments,
	})

	if providerErr != nil {
		log.Error("provider send failed", "error", providerErr)

		if errors.Is(providerErr, domain.ErrProviderPermanentFailure) {
			h.failMessageWithAttempt(ctx, msg, attempt, now, "provider_permanent_failure", providerErr.Error())
			return messageStatusFailed, nil
		}

		h.handleTemporaryFailure(ctx, msg, attempt, now, providerErr)
		return messageStatusRetryScheduled, nil
	}

	if providerResult.ProviderMessageID == "" {
		log.Error("provider returned empty provider_message_id")
		h.failMessageWithAttempt(ctx, msg, attempt, now, "provider_invalid_response", "empty provider_message_id")
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

	if err := h.persistAcceptedState(ctx, msg, attempt, providerResult, now); err != nil {
		log.Error("failed to persist accepted state after retries, moving to dlq", "error", err)

		current, readErr := h.messagesWrite.FindByID(context.Background(), msg.WorkspaceID, msg.ID)
		if readErr == nil && current.Status == domain.MessageStatusAccepted {
			log.Warn("message was already committed as accepted on a prior attempt")
			return messageStatusAccepted, nil
		}

		msg.Status = domain.MessageStatusDLQ
		msg.LastErrorClass = "accepted_state_persistence_failed"
		msg.LastErrorMessage = err.Error()
		msg.UpdatedAt = now
		h.writeEvent(context.Background(), msg, domain.MessageEventFailed, domain.MessageStatusFailed, "accepted_state_persistence_failed", err.Error(), now)

		if dlqErr := h.messagesWrite.Update(context.Background(), msg); dlqErr != nil {
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

func (h *Handler) writeEvent(ctx context.Context, msg domain.Message, eventType, status, reasonCode, reasonMessage string, now time.Time) {
	if h.eventRepo == nil {
		return
	}
	eventID, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate event id", "error", err)
		return
	}
	evt := domain.MessageEvent{
		ID:                     eventID,
		WorkspaceID:            msg.WorkspaceID,
		MessageID:              msg.ID,
		TransactionalRequestID: msg.TransactionalRequestID,
		EventType:              eventType,
		Status:                 status,
		ReasonCode:             reasonCode,
		ReasonMessage:          reasonMessage,
		OccurredAt:             now,
		CreatedAt:              now,
	}
	if err := h.eventRepo.Create(ctx, evt); err != nil {
		h.log.Error("failed to write message event",
			"message_id", msg.ID,
			"event_type", eventType,
			"error", err,
		)
	}
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

func (h *Handler) failMessage(ctx context.Context, msg domain.Message, now time.Time, errorClass, errorMessage string) {
	msg.FailedAt = &now
	msg.LastErrorClass = errorClass
	msg.LastErrorMessage = errorMessage
	msg.UpdatedAt = now

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.messagesWrite.MarkFailed(txCtx, msg); err != nil {
			return err
		}
		if msg.TransactionalRequestID != "" {
			if err := h.txRequestsWrite.UpdateAggregates(txCtx, msg.WorkspaceID, msg.TransactionalRequestID, true, false, now); err != nil {
				return err
			}
		}
		h.writeEvent(txCtx, msg, domain.MessageEventFailed, domain.MessageStatusFailed, errorClass, errorMessage, now)
		return nil
	}); err != nil {
		h.log.Error("failed to mark message failed",
			"message_id", msg.ID,
			"error_class", errorClass,
			"error", err,
		)
	}
}

func (h *Handler) failMessageWithAttempt(ctx context.Context, msg domain.Message, attempt domain.DeliveryAttempt, now time.Time, errorClass, errorMessage string) {
	attempt.Status = domain.AttemptStatusPermanentFailed
	attempt.ErrorClass = errorClass
	attempt.ErrorMessage = errorMessage
	attempt.FinishedAt = &now

	msg.FailedAt = &now
	msg.LastErrorClass = errorClass
	msg.LastErrorMessage = errorMessage
	msg.UpdatedAt = now

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.attemptsWrite.Update(txCtx, attempt); err != nil {
			return err
		}
		if err := h.messagesWrite.MarkFailed(txCtx, msg); err != nil {
			return err
		}
		if msg.TransactionalRequestID != "" {
			if err := h.txRequestsWrite.UpdateAggregates(txCtx, msg.WorkspaceID, msg.TransactionalRequestID, true, false, now); err != nil {
				return err
			}
		}
		h.writeEvent(txCtx, msg, domain.MessageEventFailed, domain.MessageStatusFailed, errorClass, errorMessage, now)
		return nil
	}); err != nil {
		h.log.Error("failed to mark message failed with attempt",
			"message_id", msg.ID,
			"error_class", errorClass,
			"error", err,
		)
	}
}

func (h *Handler) handleTemporaryFailure(ctx context.Context, msg domain.Message, attempt domain.DeliveryAttempt, now time.Time, providerErr error) {
	log := h.log.With("message_id", msg.ID)

	attempt.Status = domain.AttemptStatusTemporaryFailed
	attempt.ErrorClass = "provider_temporary_failure"
	attempt.ErrorMessage = providerErr.Error()
	attempt.FinishedAt = &now

	retryState, err := h.retryStatesWrite.FindByMessage(ctx, msg.WorkspaceID, msg.ID)
	if err != nil {
		log.Error("failed to find retry state", "error", err)
		h.failMessageWithAttempt(ctx, msg, attempt, now, "retry_state_error", "failed to find retry state")
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
		h.failMessageWithAttempt(ctx, msg, attempt, now, "retry_exhausted", "max retries reached")
		return
	}

	nextDelay := computeRetryDelay(currentRetryCount)
	nextAttemptAt := now.Add(nextDelay)

	msg.Status = domain.MessageStatusQueued
	msg.ScheduledAt = &nextAttemptAt
	msg.UpdatedAt = now

	newRetry := retryState == nil
	if newRetry {
		retryStateID, err := h.idGen()
		if err != nil {
			log.Error("failed to generate retry state ID", "error", err)
			return
		}
		retryState = &domain.RetryState{
			ID:               retryStateID,
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

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.messagesWrite.Update(txCtx, msg); err != nil {
			return err
		}
		if err := h.attemptsWrite.Update(txCtx, attempt); err != nil {
			return err
		}
		if newRetry {
			if err := h.retryStatesWrite.Create(txCtx, *retryState); err != nil {
				return err
			}
		} else {
			if err := h.retryStatesWrite.Update(txCtx, *retryState); err != nil {
				return err
			}
		}
		h.writeEvent(txCtx, msg, domain.MessageEventRetryScheduled, domain.MessageStatusQueued, "provider_temporary_failure", providerErr.Error(), now)
		if err := h.saveRetryScheduledEvent(txCtx, msg, retryState, now); err != nil {
			return err
		}
		return nil
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

func (h *Handler) persistAcceptedState(ctx context.Context, msg domain.Message, attempt domain.DeliveryAttempt, providerResult *ports.ProviderSendResult, now time.Time) error {
	eventID, err := h.idGen()
	if err != nil {
		return fmt.Errorf("generate event id: %w", err)
	}
	var lastErr error
	for i := 0; i < 3; i++ {
		txCtx := ctx
		var cancel context.CancelFunc
		if i > 0 {
			txCtx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		}

		err := h.txManager.WithinTx(txCtx, func(txCtx context.Context) error {
			if err := h.attemptsWrite.Update(txCtx, attempt); err != nil {
				return err
			}
			if err := h.messagesWrite.MarkAccepted(txCtx, msg); err != nil {
				return err
			}
			return h.saveAcceptedEvent(txCtx, msg, providerResult, now, eventID)
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

func (h *Handler) saveAcceptedEvent(ctx context.Context, msg domain.Message, providerResult *ports.ProviderSendResult, now time.Time, eventID string) error {
	h.writeEvent(ctx, msg, domain.MessageEventProviderAccepted, domain.MessageStatusAccepted, "", "", now)

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

	return h.outboxWriter.Save(ctx, ports.OutboxEvent{
		ID:            envelope.EventID,
		AggregateType: "message",
		AggregateID:   msg.ID,
		EventType:     contracts.EventDeliveryMessageAcceptedV1,
		Payload:       payloadBytes,
		WorkspaceID:   msg.WorkspaceID,
		OccurredAt:    now,
	})
}

func (h *Handler) saveRetryScheduledEvent(ctx context.Context, msg domain.Message, retryState *domain.RetryState, now time.Time) error {
	eventID, err := h.idGen()
	if err != nil {
		return err
	}

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

	return h.outboxWriter.Save(ctx, ports.OutboxEvent{
		ID:            envelope.EventID,
		AggregateType: "message",
		AggregateID:   msg.ID,
		EventType:     contracts.EventDeliveryMessageRetryScheduledV1,
		Payload:       payloadBytes,
		WorkspaceID:   msg.WorkspaceID,
		OccurredAt:    now,
	})

}

func (h *Handler) loadAttachments(ctx context.Context, msg domain.Message) ([]ports.AttachmentPart, error) {
	if msg.TransactionalRequestID == "" || h.attachmentRepo == nil || h.objectStorage == nil {
		return nil, nil
	}

	manifests, err := h.attachmentRepo.ListByRequest(ctx, msg.WorkspaceID, msg.TransactionalRequestID)
	if err != nil {
		return nil, fmt.Errorf("list attachment manifests: %w", err)
	}
	if len(manifests) == 0 {
		return nil, nil
	}

	parts := make([]ports.AttachmentPart, 0, len(manifests))
	for _, m := range manifests {
		rc, err := h.objectStorage.GetObject(ctx, m.StorageKey)
		if err != nil {
			return nil, fmt.Errorf("get attachment %s (key=%s): %w", m.OriginalFilename, m.StorageKey, err)
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("read attachment %s: %w", m.OriginalFilename, err)
		}

		disp := m.Disposition
		if disp == "" {
			disp = domain.AttachmentDispositionAttachment
		}

		parts = append(parts, ports.AttachmentPart{
			Filename:    m.OriginalFilename,
			ContentType: m.ContentType,
			ContentID:   m.ContentID,
			Disposition: disp,
			Data:        bytes.NewReader(data),
		})
	}

	return parts, nil
}
