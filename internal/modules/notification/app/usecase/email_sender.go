package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

type EmailSender struct {
	messagesWrite ports.MessageWriteRepository
	attemptsWrite ports.AttemptWriteRepository
	emailSender   ports.EmailSender
	eventPub      *EventPublisher
	idGen         func() (string, error)
	log           *slog.Logger
}

func NewEmailSender(
	messagesWrite ports.MessageWriteRepository,
	attemptsWrite ports.AttemptWriteRepository,
	emailSender ports.EmailSender,
	eventPub *EventPublisher,
	idGen func() (string, error),
	log *slog.Logger,
) *EmailSender {
	return &EmailSender{
		messagesWrite: messagesWrite,
		attemptsWrite: attemptsWrite,
		emailSender:   emailSender,
		eventPub:      eventPub,
		idGen:         idGen,
		log:           log,
	}
}

func nullIfEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Send attempts to send a notification email and returns (sent, final, err).
// sent=true means the email was delivered successfully.
// final=true means all retries exhausted and the message is permanently failed.
// err is set when a database error occurs during the send flow.
func (s *EmailSender) Send(ctx context.Context, msg *domain.NotificationMessage, attemptNumber int, provider string) (sent bool, final bool, retErr error) {
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
		ID:                    MustID(s.idGen),
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
			if err := s.eventPub.PublishFailed(ctx, *msg, attemptNumber, sendErr.Error(), true); err != nil {
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

	if err := s.eventPub.PublishSent(ctx, *msg, attemptNumber, provider); err != nil {
		return false, false, err
	}

	return true, false, nil
}
