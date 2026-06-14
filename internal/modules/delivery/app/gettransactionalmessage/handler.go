package gettransactionalmessage

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
)

type Result struct {
	MessageID         string     `json:"message_id"`
	Status            string     `json:"status"`
	Provider          string     `json:"provider,omitempty"`
	ProviderMessageID string     `json:"provider_message_id,omitempty"`
	LastUpdatedAt     *time.Time `json:"last_updated_at"`
}

type Handler struct {
	messagesRead ports.MessageReadRepository
	log          *slog.Logger
}

func New(messagesRead ports.MessageReadRepository, logger *slog.Logger) *Handler {
	return &Handler{messagesRead: messagesRead, log: logger.With("usecase", "get_transactional_message")}
}

func (h *Handler) Execute(ctx context.Context, workspaceID, messageID string) (*Result, error) {
	if workspaceID == "" || messageID == "" {
		return nil, domain.ErrPayloadInvalid
	}

	msg, err := h.messagesRead.FindByID(ctx, workspaceID, messageID)
	if err != nil {
		if errors.Is(err, domain.ErrMessageNotFound) {
			return nil, err
		}
		h.log.Error("failed to find transactional message", "workspace_id", workspaceID, "message_id", messageID, "error", err)
		return nil, err
	}

	if msg.SourceType != domain.MessageSourceTransactional {
		return nil, domain.ErrMessageNotFound
	}

	ts := msg.UpdatedAt
	if msg.AcceptedAt != nil {
		ts = *msg.AcceptedAt
	}

	return &Result{
		MessageID:         msg.ID,
		Status:            msg.Status,
		Provider:          msg.Provider,
		ProviderMessageID: msg.ProviderMessageID,
		LastUpdatedAt:     &ts,
	}, nil
}
