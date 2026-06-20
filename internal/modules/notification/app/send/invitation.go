package send

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

type InvitationNonRetryableError struct {
	Err error
}

func (e *InvitationNonRetryableError) Error() string {
	return e.Err.Error()
}

func (e *InvitationNonRetryableError) Unwrap() error {
	return e.Err
}

func (e *InvitationNonRetryableError) NonRetryable() bool {
	return true
}

type InvitationOptions struct {
	MessagesWrite ports.MessageWriteRepository
	TxManager     ports.TransactionManager
	EmailSender   *shared.EmailSender
	EventPub      *shared.EventPublisher
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type InvitationHandler struct {
	messagesWrite ports.MessageWriteRepository
	txManager     ports.TransactionManager
	emailSender   *shared.EmailSender
	eventPub      *shared.EventPublisher
	idGen         func() (string, error)
	log           *slog.Logger
}

func NewInvitation(opts InvitationOptions) *InvitationHandler {
	return &InvitationHandler{
		messagesWrite: opts.MessagesWrite,
		txManager:     opts.TxManager,
		emailSender:   opts.EmailSender,
		eventPub:      opts.EventPub,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("shared", "send_invitation_email"),
	}
}

func (h *InvitationHandler) Execute(ctx context.Context, input domain.SendInvitationEmailInput) (*domain.NotificationMessage, error) {
	if err := domain.ValidateRecipientEmail(input.InviteeEmail); err != nil {
		h.log.Warn("invalid invitee email", "error", err)
		return nil, err
	}

	now := time.Now().UTC()
	msgID := shared.MustID(h.idGen)

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

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.messagesWrite.Create(txCtx, msg); err != nil {
			return err
		}
		return h.eventPub.PublishQueued(txCtx, msg)
	}); err != nil {
		h.log.Error("failed to create invitation email message", "error", err)
		return nil, err
	}

	_, final, sendErr := h.emailSender.Send(ctx, &msg, 1, "smtp")
	if sendErr != nil {
		h.log.Error("failed to send invitation email after persistence", "message_id", msgID, "error", sendErr)
		return nil, sendErr
	}
	if final {
		h.log.Warn("invitation email permanently failed after retries", "message_id", msgID, "workspace_id", input.WorkspaceID)
		return nil, &InvitationNonRetryableError{Err: errors.New("invitation email permanently failed")}
	}

	h.log.Info("invitation email processed", "message_id", msgID, "workspace_id", input.WorkspaceID, "invitee_email", input.InviteeEmail)
	return &msg, nil
}
