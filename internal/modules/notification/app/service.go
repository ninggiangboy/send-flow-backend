package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type Options struct {
	MessagesRead  ports.MessageReadRepository
	MessagesWrite ports.MessageWriteRepository
	AttemptsRead  ports.AttemptReadRepository
	AttemptsWrite ports.AttemptWriteRepository
	OutboxWriter  ports.OutboxWriter
	TxManager     ports.TransactionManager
	AccessChecker ports.WorkspaceAccessChecker
	EmailSender   ports.EmailSender
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type Service struct {
	messagesRead  ports.MessageReadRepository
	messagesWrite ports.MessageWriteRepository
	attemptsRead  ports.AttemptReadRepository
	attemptsWrite ports.AttemptWriteRepository
	outboxWriter  ports.OutboxWriter
	txManager     ports.TransactionManager
	accessChecker ports.WorkspaceAccessChecker
	emailSender   ports.EmailSender
	idGen         func() (string, error)
	log           *slog.Logger
}

func mustID(gen func() (string, error)) string {
	id, err := gen()
	if err != nil {
		panic(err)
	}
	return id
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}
	return &Service{
		messagesRead:  opts.MessagesRead,
		messagesWrite: opts.MessagesWrite,
		attemptsRead:  opts.AttemptsRead,
		attemptsWrite: opts.AttemptsWrite,
		outboxWriter:  opts.OutboxWriter,
		txManager:     opts.TxManager,
		accessChecker: opts.AccessChecker,
		emailSender:   opts.EmailSender,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("module", "notification"),
	}
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

type GetNotificationStatusResult struct {
	Message  domain.NotificationMessage
	Attempts []domain.NotificationAttempt
}

type ListNotificationsResult struct {
	Messages   []domain.NotificationMessage
	NextCursor string
}

func (s *Service) sendEmail(ctx context.Context, msg *domain.NotificationMessage, attemptNumber int, provider string) (sent bool, final bool, retErr error) {
	now := time.Now().UTC()

	if domain.CanTransitionTo(msg.Status, domain.NotificationStatusSending) {
		msg.Status = domain.NotificationStatusSending
		msg.UpdatedAt = now
		if err := s.messagesWrite.Update(ctx, *msg); err != nil {
			return false, false, err
		}
	}

	sendErr := s.emailSender.SendNotificationEmail(ctx, []string{msg.RecipientEmail}, msg.Subject, msg.BodyText, nullIfEmpty(msg.BodyHTML))

	attempt := domain.NotificationAttempt{
		ID:                    mustID(s.idGen),
		NotificationMessageID: msg.ID,
		AttemptNumber:         attemptNumber,
		Status:                domain.AttemptStatusSent,
		Provider:              provider,
		AttemptedAt:           now,
	}

	if sendErr != nil {
		attempt.Status = domain.AttemptStatusFailed
		errMsg := sendErr.Error()
		attempt.ErrorMessage = &errMsg
	}

	if err := s.attemptsWrite.Create(ctx, attempt); err != nil {
		return false, false, err
	}

	if sendErr != nil {
		msg.AttemptCount++
		msg.LastAttemptAt = &now
		msg.UpdatedAt = now

		if msg.AttemptCount < msg.MaxAttempts {
			if !domain.CanTransitionTo(msg.Status, domain.NotificationStatusRetrying) {
				return false, true, nil
			}
			msg.Status = domain.NotificationStatusRetrying
		} else {
			if !domain.CanTransitionTo(msg.Status, domain.NotificationStatusFailed) {
				return false, true, nil
			}
			msg.Status = domain.NotificationStatusFailed
		}

		if err := s.messagesWrite.Update(ctx, *msg); err != nil {
			return false, false, err
		}

		if msg.Status == domain.NotificationStatusFailed {
			if err := s.publishFailedEvent(ctx, *msg, attemptNumber, sendErr.Error(), true); err != nil {
				return false, false, err
			}
		}
		return false, msg.Status == domain.NotificationStatusFailed, nil
	}

	msg.Status = domain.NotificationStatusSent
	msg.AttemptCount++
	msg.LastAttemptAt = &now
	msg.UpdatedAt = now

	if err := s.messagesWrite.Update(ctx, *msg); err != nil {
		return false, false, err
	}

	if err := s.publishSentEvent(ctx, *msg, attemptNumber, provider); err != nil {
		return false, false, err
	}

	return true, false, nil
}

func (s *Service) publishQueuedEvent(ctx context.Context, msg domain.NotificationMessage) error {
	eventID := mustID(s.idGen)
	ws := ""
	if msg.WorkspaceID != nil {
		ws = *msg.WorkspaceID
	}
	payload := contracts.MessageQueuedPayload{
		MessageID:      msg.ID,
		WorkspaceID:    ws,
		Type:           string(msg.Type),
		RecipientEmail: msg.RecipientEmail,
		Status:         string(msg.Status),
		CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
	}
	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventMessageQueuedV1,
		EventVersion:  1,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		WorkspaceID:   ws,
		OccurredAt:    msg.CreatedAt,
	}, payload)
	if err != nil {
		return err
	}
	payloadBytes, err := events.Marshal(envelope)
	if err != nil {
		return err
	}
	return s.outboxWriter.Save(ctx, ports.OutboxEvent{
		ID:            eventID,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		EventType:     contracts.EventMessageQueuedV1,
		Payload:       payloadBytes,
		WorkspaceID:   ws,
		OccurredAt:    msg.CreatedAt,
	})
}

func (s *Service) publishSentEvent(ctx context.Context, msg domain.NotificationMessage, attemptNumber int, provider string) error {
	eventID := mustID(s.idGen)
	ws := ""
	if msg.WorkspaceID != nil {
		ws = *msg.WorkspaceID
	}
	now := time.Now().UTC()
	payload := contracts.MessageSentPayload{
		MessageID:      msg.ID,
		WorkspaceID:    ws,
		Type:           string(msg.Type),
		RecipientEmail: msg.RecipientEmail,
		AttemptNumber:  attemptNumber,
		Provider:       provider,
		SentAt:         now.Format(time.RFC3339),
	}
	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventMessageSentV1,
		EventVersion:  1,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		WorkspaceID:   ws,
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
		ID:            eventID,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		EventType:     contracts.EventMessageSentV1,
		Payload:       payloadBytes,
		WorkspaceID:   ws,
		OccurredAt:    now,
	})
}

func (s *Service) publishFailedEvent(ctx context.Context, msg domain.NotificationMessage, attemptNumber int, errorMessage string, finalFailure bool) error {
	eventID := mustID(s.idGen)
	ws := ""
	if msg.WorkspaceID != nil {
		ws = *msg.WorkspaceID
	}
	now := time.Now().UTC()
	payload := contracts.MessageFailedPayload{
		MessageID:      msg.ID,
		WorkspaceID:    ws,
		Type:           string(msg.Type),
		RecipientEmail: msg.RecipientEmail,
		AttemptNumber:  attemptNumber,
		ErrorMessage:   errorMessage,
		FinalFailure:   finalFailure,
	}
	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventMessageFailedV1,
		EventVersion:  1,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		WorkspaceID:   ws,
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
		ID:            eventID,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		EventType:     contracts.EventMessageFailedV1,
		Payload:       payloadBytes,
		WorkspaceID:   ws,
		OccurredAt:    now,
	})
}

func (s *Service) SendWelcomeEmail(ctx context.Context, input domain.SendWelcomeEmailInput) (*domain.NotificationMessage, error) {
	log := s.log.With("usecase", "send_welcome_email")

	if err := domain.ValidateRecipientEmail(input.Email); err != nil {
		log.Warn("invalid recipient email", "error", err)
		return nil, err
	}

	now := time.Now().UTC()
	msgID := mustID(s.idGen)

	subject := WelcomeEmailSubject()
	textBody := WelcomeEmailTextBody(input.FrontendBaseURL)
	htmlBody := WelcomeEmailHTMLBody(input.FrontendBaseURL)

	msg := domain.NotificationMessage{
		ID:              msgID,
		Type:            domain.NotificationTypeWelcomeEmail,
		Status:          domain.NotificationStatusQueued,
		RecipientEmail:  input.Email,
		RecipientUserID: &input.UserID,
		Subject:         subject,
		BodyText:        textBody,
		BodyHTML:        &htmlBody,
		MaxAttempts:     3,
		AttemptCount:    0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.messagesWrite.Create(txCtx, msg); err != nil {
			return err
		}
		return s.publishQueuedEvent(txCtx, msg)
	}); err != nil {
		log.Error("failed to create welcome email message", "error", err)
		return nil, err
	}

	_, final, sendErr := s.sendEmail(ctx, &msg, 1, "smtp")
	if sendErr != nil {
		log.Error("failed to send welcome email after persistence", "message_id", msgID, "error", sendErr)
		return nil, sendErr
	}
	if final {
		log.Warn("welcome email permanently failed after retries", "message_id", msgID, "user_id", input.UserID)
		return nil, &NonRetryableError{Err: errors.New("welcome email permanently failed")}
	}

	log.Info("welcome email processed", "message_id", msgID, "user_id", input.UserID)
	return &msg, nil
}

func (s *Service) SendWorkspaceInvitationEmail(ctx context.Context, input domain.SendInvitationEmailInput) (*domain.NotificationMessage, error) {
	log := s.log.With("usecase", "send_invitation_email")

	if err := domain.ValidateRecipientEmail(input.InviteeEmail); err != nil {
		log.Warn("invalid invitee email", "error", err)
		return nil, err
	}

	now := time.Now().UTC()
	msgID := mustID(s.idGen)

	workspaceName := input.WorkspaceID

	subject := InvitationEmailSubject()
	textBody := InvitationEmailTextBody(input.InvitedByEmail, workspaceName, input.Role, input.FrontendBaseURL)
	htmlBody := InvitationEmailHTMLBody(input.InvitedByEmail, workspaceName, input.Role, input.FrontendBaseURL)

	msg := domain.NotificationMessage{
		ID:             msgID,
		WorkspaceID:    &input.WorkspaceID,
		Type:           domain.NotificationTypeInvitationEmail,
		Status:         domain.NotificationStatusQueued,
		RecipientEmail: input.InviteeEmail,
		Subject:        subject,
		BodyText:       textBody,
		BodyHTML:       &htmlBody,
		MaxAttempts:    3,
		AttemptCount:   0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.messagesWrite.Create(txCtx, msg); err != nil {
			return err
		}
		return s.publishQueuedEvent(txCtx, msg)
	}); err != nil {
		log.Error("failed to create invitation email message", "error", err)
		return nil, err
	}

	_, final, sendErr := s.sendEmail(ctx, &msg, 1, "smtp")
	if sendErr != nil {
		log.Error("failed to send invitation email after persistence", "message_id", msgID, "error", sendErr)
		return nil, sendErr
	}
	if final {
		log.Warn("invitation email permanently failed after retries", "message_id", msgID, "workspace_id", input.WorkspaceID)
		return nil, &NonRetryableError{Err: errors.New("invitation email permanently failed")}
	}

	log.Info("invitation email processed", "message_id", msgID, "workspace_id", input.WorkspaceID, "invitee_email", input.InviteeEmail)
	return &msg, nil
}

func (s *Service) SendSystemAlert(ctx context.Context, input domain.SendSystemAlertInput) (*domain.NotificationMessage, error) {
	log := s.log.With("usecase", "send_system_alert")

	if err := domain.ValidateRecipientEmail(input.RecipientEmail); err != nil {
		log.Warn("invalid recipient email", "error", err)
		return nil, err
	}
	if err := domain.ValidateSubject(input.Subject); err != nil {
		log.Warn("invalid subject", "error", err)
		return nil, err
	}

	now := time.Now().UTC()
	msgID := mustID(s.idGen)

	msg := domain.NotificationMessage{
		ID:             msgID,
		WorkspaceID:    &input.WorkspaceID,
		Type:           domain.NotificationTypeSystemAlert,
		Status:         domain.NotificationStatusQueued,
		RecipientEmail: input.RecipientEmail,
		Subject:        input.Subject,
		BodyText:       input.Body,
		MaxAttempts:    3,
		AttemptCount:   0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.messagesWrite.Create(txCtx, msg); err != nil {
			return err
		}
		return s.publishQueuedEvent(txCtx, msg)
	}); err != nil {
		log.Error("failed to create system alert message", "error", err)
		return nil, err
	}

	_, final, sendErr := s.sendEmail(ctx, &msg, 1, "smtp")
	if sendErr != nil {
		log.Error("failed to send system alert after persistence", "message_id", msgID, "error", sendErr)
		return nil, sendErr
	}
	if final {
		log.Warn("system alert permanently failed after retries", "message_id", msgID, "workspace_id", input.WorkspaceID)
		return nil, &NonRetryableError{Err: errors.New("system alert permanently failed")}
	}

	log.Info("system alert processed", "message_id", msgID, "workspace_id", input.WorkspaceID)
	return &msg, nil
}

func (s *Service) GetNotificationStatus(ctx context.Context, workspaceID, messageID, userID string) (*GetNotificationStatusResult, error) {
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "notification.read"); err != nil {
			if errors.Is(err, domain.ErrNotificationReadDenied) {
				return nil, domain.ErrNotificationReadDenied
			}
			return nil, err
		}
	}

	msg, err := s.messagesRead.FindByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, domain.ErrNotificationNotFound) {
			return nil, err
		}
		s.log.Error("failed to find notification message", "message_id", messageID, "error", err)
		return nil, err
	}

	attempts, err := s.attemptsRead.FindByMessageID(ctx, messageID)
	if err != nil {
		s.log.Error("failed to find notification attempts", "message_id", messageID, "error", err)
		return nil, err
	}

	return &GetNotificationStatusResult{
		Message:  *msg,
		Attempts: attempts,
	}, nil
}

func (s *Service) ListNotifications(ctx context.Context, filter domain.NotificationFilter, userID string) (*ListNotificationsResult, error) {
	if filter.WorkspaceID != nil {
		if s.accessChecker != nil {
			if err := s.accessChecker.RequirePermission(ctx, *filter.WorkspaceID, userID, "notification.read"); err != nil {
				return nil, err
			}
		}
	}

	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	msgs, cursor, err := s.messagesRead.List(ctx, filter)
	if err != nil {
		s.log.Error("failed to list notifications", "error", err)
		return nil, err
	}

	return &ListNotificationsResult{
		Messages:   msgs,
		NextCursor: cursor,
	}, nil
}

func nullIfEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
