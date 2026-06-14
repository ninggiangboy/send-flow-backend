package getmessage

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
)

type Input struct {
	UserID      string
	WorkspaceID string
	MessageID   string
}

type Handler struct {
	messagesRead  ports.MessageReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(messagesRead ports.MessageReadRepository, accessChecker ports.WorkspaceAccessChecker, logger *slog.Logger) *Handler {
	return &Handler{messagesRead: messagesRead, accessChecker: accessChecker, log: logger.With("usecase", "get_message")}
}

func (h *Handler) Execute(ctx context.Context, input Input) (*domain.Message, error) {
	if input.WorkspaceID == "" || input.MessageID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "delivery.read"); err != nil {
			return nil, err
		}
	}

	msg, err := h.messagesRead.FindByID(ctx, input.WorkspaceID, input.MessageID)
	if err != nil {
		if errors.Is(err, domain.ErrMessageNotFound) {
			return nil, err
		}
		h.log.Error("failed to find message", "workspace_id", input.WorkspaceID, "message_id", input.MessageID, "error", err)
		return nil, err
	}

	return msg, nil
}
