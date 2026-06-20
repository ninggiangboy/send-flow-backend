package send

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	deliveryredis "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
)

const (
	maxIdempotencyKeyLength = 255
	maxRecipientNameLength  = 200
	maxMetadataSize         = 16 * 1024
	maxTemplateDataSize     = 64 * 1024
	maxTagsCount            = 20
	maxTagLength            = 64
	maxRecipients           = 50
	maxBodyLength           = 512 * 1024 // 512KB
)

type AttachmentStream struct {
	Filename    string
	ContentType string
	Data        io.ReadSeeker
	Size        int64
	SHA256      string
	Disposition string
	ContentID   string
}

type Input struct {
	WorkspaceID       string
	APIKeyID          string
	IdempotencyKey    string
	Mode              string
	SenderDomainID    string
	SenderName        string
	Subject           string
	TemplateID        string
	TemplateVersionID string
	TemplateData      map[string]any
	TextBody          string
	HTMLBody          string
	ReplyTo           string
	To                []domain.RecipientTarget
	CC                []domain.RecipientTarget
	BCC               []domain.RecipientTarget
	Metadata          map[string]any
	Tags              []string
	Headers           map[string]string
	Attachments       []AttachmentStream
	Now               time.Time
}

type Result struct {
	RequestID  string
	MessageIDs []string
	Status     string
	AcceptedAt time.Time
}

type Handler struct {
	txRequestsWrite    ports.TransactionalRequestWriteRepository
	messagesWrite      ports.MessageWriteRepository
	senderChecker      ports.SenderReadinessChecker
	contentRenderer    ports.ContentRenderer
	suppressionChecker ports.SuppressionChecker
	attachmentRepo     ports.AttachmentRepository
	eventRepo          ports.MessageEventRepository
	objectStorage      ports.ObjectStorage
	outboxWriter       ports.OutboxWriter
	txManager          ports.UnitOfWork
	idGen              func() (string, error)
	log                *slog.Logger
	cache              *deliveryredis.Cache
	attachmentMetrics  *observability.AttachmentMetrics
	quotaEnforcer      ports.QuotaEnforcer
}

func New(
	txRequestsWrite ports.TransactionalRequestWriteRepository,
	messagesWrite ports.MessageWriteRepository,
	senderChecker ports.SenderReadinessChecker,
	contentRenderer ports.ContentRenderer,
	suppressionChecker ports.SuppressionChecker,
	attachmentRepo ports.AttachmentRepository,
	eventRepo ports.MessageEventRepository,
	objectStorage ports.ObjectStorage,
	outboxWriter ports.OutboxWriter,
	txManager ports.UnitOfWork,
	idGen func() (string, error),
	logger *slog.Logger,
	cache *deliveryredis.Cache,
	attachmentMetrics *observability.AttachmentMetrics,
	quotaEnforcer ports.QuotaEnforcer,
) *Handler {
	return &Handler{
		txRequestsWrite:    txRequestsWrite,
		messagesWrite:      messagesWrite,
		senderChecker:      senderChecker,
		contentRenderer:    contentRenderer,
		suppressionChecker: suppressionChecker,
		attachmentRepo:     attachmentRepo,
		eventRepo:          eventRepo,
		objectStorage:      objectStorage,
		outboxWriter:       outboxWriter,
		txManager:          txManager,
		idGen:              idGen,
		log:                logger.With("usecase", "accept_transactional_send"),
		cache:              cache,
		attachmentMetrics:  attachmentMetrics,
		quotaEnforcer:      quotaEnforcer,
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) (*Result, error) {
	log := h.log.With("workspace_id", input.WorkspaceID, "mode", input.Mode)

	if err := h.validateSendInput(&input); err != nil {
		log.Warn("transactional send validation failed", "error", err)
		return nil, err
	}

	targets, err := h.explodeRecipients(input.To, input.CC, input.BCC)
	if err != nil {
		log.Warn("recipient explosion failed", "error", err)
		return nil, err
	}

	// Build canonical request for idempotency
	canonical := h.buildCanonical(input, targets)
	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		return nil, domain.ErrRequestBodyInvalid
	}

	requestHash := computeRequestHash(canonical, input.Attachments)
	inputKey := strings.TrimSpace(input.IdempotencyKey)

	// Handle idempotency
	if inputKey != "" {
		result, err := h.handleIdempotency(ctx, input.WorkspaceID, inputKey, requestHash)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	// Check sender readiness
	if err := h.checkSenderReadiness(ctx, input.WorkspaceID, input.SenderDomainID); err != nil {
		log.Warn("sender readiness check failed", "sender_domain_id", input.SenderDomainID, "error", err)
		return nil, err
	}

	// Template mode: render template to validate and get subject
	var renderedSubject string
	if input.Mode == domain.MessageModeTemplate {
		rendered, err := h.contentRenderer.RenderForMessage(ctx, input.WorkspaceID, input.TemplateID, input.TemplateVersionID, input.TemplateData)
		if err != nil {
			log.Warn("template render failed", "error", err)
			if errors.Is(err, domain.ErrTemplateNotFound) || errors.Is(err, domain.ErrTemplateRenderPayloadInvalid) {
				return nil, err
			}
			return nil, domain.ErrTemporarilyUnavailable
		}
		renderedSubject = rendered.Subject
	} else {
		renderedSubject = input.Subject
	}

	// Suppression check all recipients
	allEmails := make([]string, 0, len(targets))
	for _, t := range targets {
		allEmails = append(allEmails, normalizeEmail(t.recipient.Email))
	}
	for _, email := range allEmails {
		if err := h.checkSuppression(ctx, input.WorkspaceID, email); err != nil {
			log.Warn("suppression check failed", "recipient", email, "error", err)
			return nil, err
		}
	}

	// Enforce API key quota after all business-rule rejections so that sender
	// failures, invalid templates, and suppressed recipients don't burn quota.
	// Replays were already returned above, so this only fires for new requests.
	quotaConsumed := false
	if h.quotaEnforcer != nil {
		if err := h.quotaEnforcer.CheckAndConsume(ctx, input.WorkspaceID, input.APIKeyID, len(targets)); err != nil {
			log.Warn("api key quota exceeded",
				"api_key_id", input.APIKeyID,
				"recipient_units", len(targets),
				"error", err,
			)
			return nil, err
		}
		quotaConsumed = true
	}

	// refundQuota returns consumed units when the request is not durably
	// accepted (attachment failure, DB failure, or concurrent dup). It is
	// best-effort: a Redis failure only produces a warning.
	refundQuota := func() {
		if !quotaConsumed {
			return
		}
		quotaConsumed = false // guard against double-call
		if refundErr := h.quotaEnforcer.Refund(ctx, input.WorkspaceID, input.APIKeyID, len(targets)); refundErr != nil {
			log.Warn("failed to refund api key quota after request failure",
				"api_key_id", input.APIKeyID,
				"recipient_units", len(targets),
				"error", refundErr,
			)
		}
	}

	var requestID string
	if len(input.Attachments) > 0 {
		requestID, err = h.idGen()
		if err != nil {
			log.Error("failed to generate request ID for attachments", "error", err)
			refundQuota()
			return nil, domain.ErrTemporarilyUnavailable
		}
	}

	// Store attachments to object storage (before DB TX) using the final request ID.
	var attachmentManifests []domain.AttachmentManifest
	if len(input.Attachments) > 0 && h.objectStorage != nil {
		manifests, err := h.storeAttachments(ctx, input.WorkspaceID, requestID, input.Attachments)
		if err != nil {
			log.Error("attachment storage failed", "error", err)
			refundQuota()
			return nil, err
		}
		attachmentManifests = manifests
	}

	// Within DB transaction: create request, messages, events, outbox events
	errUniqueViolation := errors.New("unique violation recovery needed")
	var result *Result

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if requestID == "" {
			requestID, err = h.idGen()
			if err != nil {
				log.Error("failed to generate request ID", "error", err)
				return err
			}
		}

		var idempotencyKey *string
		if inputKey != "" {
			idempotencyKey = &inputKey
		}

		subject := renderedSubject
		if subject == "" {
			subject = input.Subject
		}

		txReq := domain.TransactionalSendRequest{
			ID:              requestID,
			WorkspaceID:     input.WorkspaceID,
			IdempotencyKey:  idempotencyKey,
			Status:          domain.TxRequestStatusAccepted,
			RequestPayload:  canonicalJSON,
			Mode:            input.Mode,
			Subject:         subject,
			SenderName:      input.SenderName,
			SourceAPIKeyID:  input.APIKeyID,
			RequestHash:     requestHash,
			TotalRecipients: len(targets),
			CreatedAt:       input.Now,
			UpdatedAt:       input.Now,
		}

		if err := h.txRequestsWrite.Create(txCtx, txReq); err != nil {
			if errors.Is(err, domain.ErrIdempotencyKeyConflict) {
				return errUniqueViolation
			}
			return err
		}

		// Store attachment manifests (re-key with actual request ID)
		if len(attachmentManifests) > 0 {
			for i := range attachmentManifests {
				attachmentManifests[i].TransactionalRequestID = requestID
			}
			if err := h.attachmentRepo.CreateMany(txCtx, attachmentManifests); err != nil {
				return err
			}
		}

		// Create messages per recipient
		var messageIDs []string
		now := input.Now

		for _, t := range targets {
			messageID, err := h.idGen()
			if err != nil {
				log.Error("failed to generate message ID", "error", err)
				return err
			}
			messageIDs = append(messageIDs, messageID)

			recipientEmail := normalizeEmail(t.recipient.Email)
			recipientEmail = strings.ToLower(strings.TrimSpace(t.recipient.Email))

			recipientSnapshot := domain.RecipientSnapshot{
				Email:           t.recipient.Email,
				EmailNormalized: recipientEmail,
				FirstName:       strings.TrimSpace(t.recipient.Name),
				Tags:            input.Tags,
				Attributes:      input.Metadata,
				TemplateData:    input.TemplateData,
			}

			msg := domain.Message{
				ID:                       messageID,
				WorkspaceID:              input.WorkspaceID,
				TransactionalRequestID:   requestID,
				RecipientEmailNormalized: recipientEmail,
				RecipientSnapshot:        recipientSnapshot,
				TemplateID:               input.TemplateID,
				TemplateVersionID:        input.TemplateVersionID,
				SenderDomainID:           input.SenderDomainID,
				Subject:                  subject,
				SenderName:               input.SenderName,
				TextBody:                 input.TextBody,
				HTMLBody:                 input.HTMLBody,
				ReplyTo:                  input.ReplyTo,
				Headers:                  input.Headers,
				RecipientRole:            t.role,
				SourceAPIKeyID:           input.APIKeyID,
				MessageType:              domain.MessageTypeTransactional,
				SourceType:               domain.MessageSourceTransactional,
				Status:                   domain.MessageStatusQueued,
				QueuedAt:                 &now,
				CreatedAt:                now,
				UpdatedAt:                now,
			}

			insertedIDs, err := h.messagesWrite.CreateMany(txCtx, []domain.Message{msg})
			if err != nil {
				return err
			}
			if len(insertedIDs) == 0 {
				return domain.ErrTemporarilyUnavailable
			}

			// Create message event
			eventID, err := h.idGen()
			if err != nil {
				log.Error("failed to generate event ID", "error", err)
				return err
			}

			event := domain.MessageEvent{
				ID:                     eventID,
				WorkspaceID:            input.WorkspaceID,
				MessageID:              messageID,
				TransactionalRequestID: requestID,
				EventType:              domain.MessageEventQueued,
				Status:                 domain.MessageStatusQueued,
				OccurredAt:             now,
				CreatedAt:              now,
			}

			if err := h.eventRepo.Create(txCtx, event); err != nil {
				return err
			}

			// Emit message.queued outbox event
			queuedEventID, err := h.idGen()
			if err != nil {
				log.Error("failed to generate queued event ID", "error", err)
				return err
			}

			payload := contracts.MessageQueuedPayload{
				MessageID:              messageID,
				WorkspaceID:            input.WorkspaceID,
				TransactionalRequestID: requestID,
				TemplateID:             input.TemplateID,
				TemplateVersionID:      input.TemplateVersionID,
				SenderDomainID:         input.SenderDomainID,
				MessageType:            domain.MessageTypeTransactional,
				SourceType:             domain.MessageSourceTransactional,
			}
			envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
				EventID:       queuedEventID,
				EventType:     contracts.EventDeliveryMessageQueuedV1,
				EventVersion:  1,
				AggregateType: "message",
				AggregateID:   messageID,
				WorkspaceID:   input.WorkspaceID,
				OccurredAt:    now,
			}, payload)
			if err != nil {
				return err
			}
			payloadBytes, err := events.Marshal(envelope)
			if err != nil {
				return err
			}
			if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            envelope.EventID,
				AggregateType: "message",
				AggregateID:   messageID,
				EventType:     contracts.EventDeliveryMessageQueuedV1,
				Payload:       payloadBytes,
				WorkspaceID:   input.WorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				return err
			}
		}

		// Emit transactional_send.accepted event
		acceptedEventID, err := h.idGen()
		if err != nil {
			log.Error("failed to generate accepted event ID", "error", err)
			return err
		}

		acceptedPayload := contracts.MessageQueuedPayload{
			MessageID:              messageIDs[0],
			WorkspaceID:            input.WorkspaceID,
			TransactionalRequestID: requestID,
			TemplateID:             input.TemplateID,
			TemplateVersionID:      input.TemplateVersionID,
			SenderDomainID:         input.SenderDomainID,
			MessageType:            domain.MessageTypeTransactional,
			SourceType:             domain.MessageSourceTransactional,
		}
		acceptedEnvelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
			EventID:       acceptedEventID,
			EventType:     contracts.EventDeliveryTransactionalSendAcceptedV1,
			EventVersion:  1,
			AggregateType: "transactional_send_request",
			AggregateID:   requestID,
			WorkspaceID:   input.WorkspaceID,
			OccurredAt:    now,
		}, acceptedPayload)
		if err != nil {
			return err
		}
		acceptedPayloadBytes, err := events.Marshal(acceptedEnvelope)
		if err != nil {
			return err
		}
		if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
			ID:            acceptedEnvelope.EventID,
			AggregateType: "transactional_send_request",
			AggregateID:   requestID,
			EventType:     contracts.EventDeliveryTransactionalSendAcceptedV1,
			Payload:       acceptedPayloadBytes,
			WorkspaceID:   input.WorkspaceID,
			OccurredAt:    now,
		}); err != nil {
			return err
		}

		result = &Result{
			RequestID:  requestID,
			MessageIDs: messageIDs,
			Status:     domain.TxRequestStatusAccepted,
			AcceptedAt: now,
		}
		return nil
	}); err != nil {
		if len(attachmentManifests) > 0 {
			h.cleanupUploadedAttachments(ctx, attachmentManifests)
		}
		// All paths here abandon this request: refund quota regardless of
		// whether we return an error or the existing result of a concurrent dup.
		refundQuota()
		if errors.Is(err, errUniqueViolation) {
			return h.recoverFromUniqueViolation(ctx, input.WorkspaceID, inputKey, requestHash)
		}
		if errors.Is(err, domain.ErrIdempotencyKeyConflict) {
			return nil, err
		}
		log.Error("transactional send transaction failed", "error", err)
		return nil, err
	}

	// Cache idempotency entry
	if h.cache != nil && result != nil && inputKey != "" {
		_ = h.cache.SetIdempotency(ctx, input.WorkspaceID, inputKey, &deliveryredis.IdempotencyEntry{
			PayloadHash: requestHash,
			RequestID:   result.RequestID,
			MessageID:   result.MessageIDs[0],
			Status:      result.Status,
			AcceptedAt:  result.AcceptedAt.Format(time.RFC3339),
		})
	}

	log.Info("transactional send accepted",
		"request_id", result.RequestID,
		"message_count", len(result.MessageIDs),
	)

	return result, nil
}

type recipientTarget struct {
	recipient domain.RecipientTarget
	role      string
}

func (h *Handler) explodeRecipients(to, cc, bcc []domain.RecipientTarget) ([]recipientTarget, error) {
	seen := make(map[string]string)
	var targets []recipientTarget

	appendIfUnique := func(rt domain.RecipientTarget, role string) error {
		email := strings.ToLower(strings.TrimSpace(rt.Email))
		if existing, ok := seen[email]; ok {
			return fmt.Errorf("%w: email %q appears in both %s and %s",
				domain.ErrDuplicateRecipient, rt.Email, existing, role)
		}
		seen[email] = role
		targets = append(targets, recipientTarget{recipient: rt, role: role})
		return nil
	}

	for _, t := range to {
		if err := appendIfUnique(t, domain.RecipientRoleTo); err != nil {
			return nil, err
		}
	}
	for _, t := range cc {
		if err := appendIfUnique(t, domain.RecipientRoleCC); err != nil {
			return nil, err
		}
	}
	for _, t := range bcc {
		if err := appendIfUnique(t, domain.RecipientRoleBCC); err != nil {
			return nil, err
		}
	}

	return targets, nil
}

// buildCanonical builds a JSON-serializable representation of the request for
// idempotency comparison and persistence in request_payload.
func (h *Handler) buildCanonical(input Input, targets []recipientTarget) domain.CanonicalSendRequest {
	canonical := domain.CanonicalSendRequest{
		Mode:              input.Mode,
		SenderDomainID:    strings.TrimSpace(input.SenderDomainID),
		SenderName:        input.SenderName,
		Subject:           input.Subject,
		TemplateID:        strings.TrimSpace(input.TemplateID),
		TemplateVersionID: strings.TrimSpace(input.TemplateVersionID),
		TemplateData:      input.TemplateData,
		TextBody:          input.TextBody,
		HTMLBody:          input.HTMLBody,
		ReplyTo:           input.ReplyTo,
		Metadata:          input.Metadata,
		Tags:              cleanTags(input.Tags),
		Headers:           input.Headers,
	}

	for _, t := range input.To {
		canonical.To = append(canonical.To, domain.RecipientTarget{
			Email: strings.TrimSpace(t.Email),
			Name:  strings.TrimSpace(t.Name),
		})
	}
	for _, t := range input.CC {
		canonical.CC = append(canonical.CC, domain.RecipientTarget{
			Email: strings.TrimSpace(t.Email),
			Name:  strings.TrimSpace(t.Name),
		})
	}
	for _, t := range input.BCC {
		canonical.BCC = append(canonical.BCC, domain.RecipientTarget{
			Email: strings.TrimSpace(t.Email),
			Name:  strings.TrimSpace(t.Name),
		})
	}

	for _, att := range input.Attachments {
		canonical.Attachments = append(canonical.Attachments, domain.FilePart{
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Size:        att.Size,
			SHA256:      att.SHA256,
			Disposition: att.Disposition,
			ContentID:   att.ContentID,
		})
	}

	return canonical
}

func computeRequestHash(canonical domain.CanonicalSendRequest, attachments []AttachmentStream) string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	// Encode canonical (which includes attachment metadata but not data bytes)
	_ = enc.Encode(canonical)
	// Include attachment SHA256 digests in order (belt and suspenders)
	for _, att := range attachments {
		h.Write([]byte(att.SHA256))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (h *Handler) storeAttachments(ctx context.Context, workspaceID, requestID string, streams []AttachmentStream) ([]domain.AttachmentManifest, error) {
	manifests := make([]domain.AttachmentManifest, 0, len(streams))
	for _, stream := range streams {
		storageKey := fmt.Sprintf("attachments/%s/%s/%s_%s", workspaceID, requestID, stream.SHA256[:12], stream.Filename)

		if stream.Data != nil {
			if _, err := stream.Data.Seek(0, io.SeekStart); err != nil {
				return nil, domain.ErrAttachmentStorageFailed
			}
		}

		if h.attachmentMetrics != nil {
			h.attachmentMetrics.ObserveAttachmentCount(len(streams))
		}
		uploadStart := time.Now()

		if err := h.objectStorage.PutObject(ctx, storageKey, stream.Data, stream.ContentType); err != nil {
			if h.attachmentMetrics != nil {
				h.attachmentMetrics.RecordUploadFailure()
			}
			return nil, fmt.Errorf("%w: %w", domain.ErrAttachmentStorageFailed, err)
		}

		if h.attachmentMetrics != nil {
			h.attachmentMetrics.RecordUploadSuccess()
			h.attachmentMetrics.AddBytesUploaded(stream.Size)
			h.attachmentMetrics.ObserveUploadDuration(time.Since(uploadStart))
		}

		attID, err := h.idGen()
		if err != nil {
			return nil, domain.ErrTemporarilyUnavailable
		}

		disp := stream.Disposition
		if disp == "" {
			disp = domain.AttachmentDispositionAttachment
		}

		manifest := domain.AttachmentManifest{
			ID:               attID,
			WorkspaceID:      workspaceID,
			StorageKey:       storageKey,
			OriginalFilename: stream.Filename,
			ContentType:      stream.ContentType,
			ByteSize:         stream.Size,
			SHA256Digest:     stream.SHA256,
			Disposition:      disp,
			ContentID:        stream.ContentID,
			CreatedAt:        time.Now().UTC(),
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

func (h *Handler) cleanupUploadedAttachments(ctx context.Context, manifests []domain.AttachmentManifest) {
	if h.objectStorage == nil {
		return
	}
	for _, manifest := range manifests {
		if manifest.StorageKey == "" {
			continue
		}
		if err := h.objectStorage.DeleteObject(ctx, manifest.StorageKey); err != nil {
			h.log.Warn("failed to cleanup uploaded attachment", "storage_key", manifest.StorageKey, "error", err)
		}
	}
}

func (h *Handler) handleIdempotency(ctx context.Context, workspaceID, idempotencyKey, requestHash string) (*Result, error) {
	if h.cache != nil {
		if entry, err := h.cache.GetIdempotency(ctx, workspaceID, idempotencyKey); err == nil {
			if entry.PayloadHash == requestHash {
				return h.loadExistingRequestResult(ctx, workspaceID, idempotencyKey, requestHash, parseTimeOrZero(entry.AcceptedAt))
			}
			return nil, domain.ErrIdempotencyKeyConflict
		}
	}

	existingReq, err := h.txRequestsWrite.FindByIdempotencyKey(ctx, workspaceID, idempotencyKey)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionalRequestNotFound) {
			return nil, nil
		}
		return nil, domain.ErrTemporarilyUnavailable
	}

	// Compare request hash
	if existingReq.RequestHash != requestHash {
		return nil, domain.ErrIdempotencyKeyConflict
	}

	messageIDs, err := h.listRequestMessageIDs(ctx, workspaceID, existingReq.ID)
	if err != nil {
		h.log.Error("failed to list messages for idempotent retry",
			"request_id", existingReq.ID,
			"workspace_id", workspaceID,
			"error", err,
		)
		return nil, domain.ErrTemporarilyUnavailable
	}

	if h.cache != nil {
		_ = h.cache.SetIdempotency(ctx, workspaceID, idempotencyKey, &deliveryredis.IdempotencyEntry{
			PayloadHash: requestHash,
			RequestID:   existingReq.ID,
			MessageID:   firstMessageID(messageIDs),
			Status:      existingReq.Status,
			AcceptedAt:  existingReq.CreatedAt.Format(time.RFC3339),
		})
	}

	return &Result{
		RequestID:  existingReq.ID,
		MessageIDs: messageIDs,
		Status:     existingReq.Status,
		AcceptedAt: existingReq.CreatedAt,
	}, nil
}

func (h *Handler) recoverFromUniqueViolation(ctx context.Context, workspaceID, idempotencyKey, requestHash string) (*Result, error) {
	return h.loadExistingRequestResult(ctx, workspaceID, idempotencyKey, requestHash, time.Time{})
}

func (h *Handler) loadExistingRequestResult(ctx context.Context, workspaceID, idempotencyKey, requestHash string, acceptedAtFallback time.Time) (*Result, error) {
	existingReq, lookupErr := h.txRequestsWrite.FindByIdempotencyKey(ctx, workspaceID, idempotencyKey)
	if lookupErr != nil {
		h.log.Error("failed to lookup conflicting request on unique violation recovery", "idempotency_key", idempotencyKey, "error", lookupErr)
		return nil, domain.ErrTemporarilyUnavailable
	}

	if existingReq.RequestHash != requestHash {
		return nil, domain.ErrIdempotencyKeyConflict
	}

	messageIDs, lookupErr := h.listRequestMessageIDs(ctx, workspaceID, existingReq.ID)
	if lookupErr != nil {
		h.log.Error("failed to list messages on idempotency recovery", "request_id", existingReq.ID, "error", lookupErr)
		return nil, domain.ErrTemporarilyUnavailable
	}

	acceptedAt := existingReq.CreatedAt
	if acceptedAt.IsZero() {
		acceptedAt = acceptedAtFallback
	}

	return &Result{
		RequestID:  existingReq.ID,
		MessageIDs: messageIDs,
		Status:     existingReq.Status,
		AcceptedAt: acceptedAt,
	}, nil
}

func (h *Handler) listRequestMessageIDs(ctx context.Context, workspaceID, requestID string) ([]string, error) {
	messages, _, err := h.messagesWrite.List(ctx, ports.MessageListQuery{
		WorkspaceID:            workspaceID,
		TransactionalRequestID: requestID,
		Limit:                  maxRecipients,
	})
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, domain.ErrMessageNotFound
	}

	messageIDs := make([]string, 0, len(messages))
	for _, msg := range messages {
		messageIDs = append(messageIDs, msg.ID)
	}
	return messageIDs, nil
}

func firstMessageID(messageIDs []string) string {
	if len(messageIDs) == 0 {
		return ""
	}
	return messageIDs[0]
}

func (h *Handler) validateSendInput(input *Input) error {
	if input.WorkspaceID == "" {
		return domain.ErrRequestBodyInvalid
	}

	// Mode validation
	if input.Mode == "" {
		input.Mode = domain.MessageModeTemplate
	}
	if input.Mode != domain.MessageModeTemplate && input.Mode != domain.MessageModeRaw {
		return domain.ErrModeInvalid
	}

	input.SenderDomainID = strings.TrimSpace(input.SenderDomainID)
	if input.SenderDomainID == "" {
		return domain.ErrRequestBodyInvalid
	}

	// Template mode validations
	if input.Mode == domain.MessageModeTemplate {
		input.TemplateID = strings.TrimSpace(input.TemplateID)
		if input.TemplateID == "" {
			return domain.ErrRequestBodyInvalid
		}
		input.TemplateVersionID = strings.TrimSpace(input.TemplateVersionID)
		if len(input.Attachments) > 0 {
			return domain.ErrAttachmentNotSupported
		}
	}

	// Raw mode validations
	if input.Mode == domain.MessageModeRaw {
		if input.Subject == "" {
			return domain.ErrSubjectRequired
		}
		if input.TextBody == "" && input.HTMLBody == "" {
			return domain.ErrRawBodyRequired
		}
		if len(input.HTMLBody) > maxBodyLength || len(input.TextBody) > maxBodyLength {
			return domain.ErrRequestBodyInvalid
		}

		// If object storage is nil and there are attachments, reject
		if len(input.Attachments) > 0 && h.objectStorage == nil {
			return domain.ErrObjectStorageDisabled
		}
	}

	// Recipient validation
	totalRecipients := len(input.To) + len(input.CC) + len(input.BCC)
	if totalRecipients == 0 {
		return domain.ErrRecipientInvalid
	}
	if totalRecipients > maxRecipients {
		return domain.ErrRecipientInvalid
	}

	for _, list := range [][]domain.RecipientTarget{input.To, input.CC, input.BCC} {
		for _, r := range list {
			if _, err := mail.ParseAddress(strings.TrimSpace(r.Email)); err != nil {
				return domain.ErrRecipientInvalid
			}
		}
	}

	// Tags validation
	if len(input.Tags) > maxTagsCount {
		input.Tags = input.Tags[:maxTagsCount]
	}
	for i, tag := range input.Tags {
		if len(tag) > maxTagLength {
			input.Tags[i] = tag[:maxTagLength]
		}
	}

	// Metadata size validation
	if input.Metadata != nil {
		data, err := json.Marshal(input.Metadata)
		if err != nil {
			return domain.ErrRequestBodyInvalid
		}
		if len(data) > maxMetadataSize {
			return domain.ErrRequestBodyInvalid
		}
	}

	// Template data size validation
	if input.TemplateData != nil {
		data, err := json.Marshal(input.TemplateData)
		if err != nil {
			return domain.ErrRequestBodyInvalid
		}
		if len(data) > maxTemplateDataSize {
			return domain.ErrRequestBodyInvalid
		}
	}

	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}

	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if len(input.IdempotencyKey) > maxIdempotencyKeyLength {
		return domain.ErrRequestBodyInvalid
	}

	return nil
}

func (h *Handler) checkSenderReadiness(ctx context.Context, workspaceID, senderDomainID string) error {
	readiness, err := h.senderChecker.GetSenderReadiness(ctx, workspaceID, senderDomainID)
	if err != nil {
		if errors.Is(err, domain.ErrSenderDomainNotFound) {
			return err
		}
		return domain.ErrTemporarilyUnavailable
	}
	if readiness == nil || !readiness.Ready {
		return domain.ErrSenderDomainNotVerified
	}
	return nil
}

func (h *Handler) checkSuppression(ctx context.Context, workspaceID, emailNormalized string) error {
	decision, err := h.suppressionChecker.CheckSuppression(ctx, workspaceID, emailNormalized, "workspace")
	if err != nil {
		return domain.ErrTemporarilyUnavailable
	}
	if decision != nil && decision.Suppressed {
		return domain.ErrSuppressedRecipient
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func cleanTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if len(t) > maxTagLength {
			t = t[:maxTagLength]
		}
		cleaned = append(cleaned, t)
	}
	if len(cleaned) > maxTagsCount {
		cleaned = cleaned[:maxTagsCount]
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func parseTimeOrZero(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
