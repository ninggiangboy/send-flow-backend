package sendsystemalert

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return e.Err.Error()
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

func (e *NonRetryableError) NonRetryable() bool {
	return true
}

type Options struct {
	MessagesWrite ports.MessageWriteRepository
	TxManager     ports.TransactionManager
	EmailSender   *usecase.EmailSender
	EventPub      *usecase.EventPublisher
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type Handler struct {
	messagesWrite ports.MessageWriteRepository
	txManager     ports.TransactionManager
	emailSender   *usecase.EmailSender
	eventPub      *usecase.EventPublisher
	idGen         func() (string, error)
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		messagesWrite: opts.MessagesWrite,
		txManager:     opts.TxManager,
		emailSender:   opts.EmailSender,
		eventPub:      opts.EventPub,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("usecase", "send_system_alert"),
	}
}

func (h *Handler) Execute(ctx context.Context, input domain.SendSystemAlertInput) (*domain.NotificationMessage, error) {
	if err := domain.ValidateRecipientEmail(input.RecipientEmail); err != nil {
		h.log.Warn("invalid recipient email", "error", err)
		return nil, err
	}
	if err := domain.ValidateSubject(input.Subject); err != nil {
		h.log.Warn("invalid subject", "error", err)
		return nil, err
	}

	now := time.Now().UTC()
	msgID := usecase.MustID(h.idGen)

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

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.messagesWrite.Create(txCtx, msg); err != nil {
			return err
		}
		return h.eventPub.PublishQueued(txCtx, msg)
	}); err != nil {
		h.log.Error("failed to create system alert message", "error", err)
		return nil, err
	}

	_, final, sendErr := h.emailSender.Send(ctx, &msg, 1, "smtp")
	if sendErr != nil {
		h.log.Error("failed to send system alert after persistence", "message_id", msgID, "error", sendErr)
		return nil, sendErr
	}
	if final {
		h.log.Warn("system alert permanently failed after retries", "message_id", msgID, "workspace_id", input.WorkspaceID)
		return nil, &NonRetryableError{Err: errors.New("system alert permanently failed")}
	}

	h.log.Info("system alert processed", "message_id", msgID, "workspace_id", input.WorkspaceID)
	return &msg, nil
}
