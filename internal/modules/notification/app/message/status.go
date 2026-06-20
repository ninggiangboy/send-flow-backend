package message

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

type StatusOptions struct {
	MessagesRead  ports.MessageReadRepository
	AttemptsRead  ports.AttemptReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type StatusQuery struct {
	WorkspaceID string
	MessageID   string
	UserID      string
}

type StatusResult struct {
	Message  domain.NotificationMessage
	Attempts []domain.NotificationAttempt
}

type StatusHandler struct {
	messagesRead  ports.MessageReadRepository
	attemptsRead  ports.AttemptReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewStatus(opts StatusOptions) *StatusHandler {
	return &StatusHandler{
		messagesRead:  opts.MessagesRead,
		attemptsRead:  opts.AttemptsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "get_notification_status"),
	}
}

func (h *StatusHandler) Execute(ctx context.Context, q StatusQuery) (*StatusResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "notification.read"); err != nil {
			if errors.Is(err, domain.ErrNotificationReadDenied) {
				return nil, domain.ErrNotificationReadDenied
			}
			return nil, err
		}
	}

	msg, err := h.messagesRead.FindByID(ctx, q.MessageID)
	if err != nil {
		if errors.Is(err, domain.ErrNotificationNotFound) {
			return nil, err
		}
		h.log.Error("failed to find notification message", "message_id", q.MessageID, "error", err)
		return nil, err
	}

	attempts, err := h.attemptsRead.FindByMessageID(ctx, q.MessageID)
	if err != nil {
		h.log.Error("failed to find notification attempts", "message_id", q.MessageID, "error", err)
		return nil, err
	}

	return &StatusResult{
		Message:  *msg,
		Attempts: attempts,
	}, nil
}
