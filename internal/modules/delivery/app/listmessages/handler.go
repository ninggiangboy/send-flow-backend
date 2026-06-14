package listmessages

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Input struct {
	UserID                   string
	WorkspaceID              string
	CampaignID               string
	TransactionalRequestID   string
	Status                   string
	RecipientEmailNormalized string
	ProviderMessageID        string
	From                     *time.Time
	To                       *time.Time
	Limit                    int
	Cursor                   string
}

type Result struct {
	Messages   []domain.Message
	NextCursor string
}

type Handler struct {
	messagesRead  ports.MessageReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(messagesRead ports.MessageReadRepository, accessChecker ports.WorkspaceAccessChecker, logger *slog.Logger) *Handler {
	return &Handler{messagesRead: messagesRead, accessChecker: accessChecker, log: logger.With("usecase", "list_messages")}
}

func (h *Handler) Execute(ctx context.Context, input Input) (*Result, error) {
	if input.WorkspaceID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "delivery.read"); err != nil {
			return nil, err
		}
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	emailNormalized := strings.TrimSpace(strings.ToLower(input.RecipientEmailNormalized))

	query := ports.MessageListQuery{
		WorkspaceID:              input.WorkspaceID,
		CampaignID:               input.CampaignID,
		TransactionalRequestID:   input.TransactionalRequestID,
		Status:                   input.Status,
		RecipientEmailNormalized: emailNormalized,
		ProviderMessageID:        input.ProviderMessageID,
		From:                     input.From,
		To:                       input.To,
		Limit:                    limit,
		Cursor:                   input.Cursor,
	}

	messages, cursor, err := h.messagesRead.List(ctx, query)
	if err != nil {
		h.log.Error("failed to list messages", "workspace_id", input.WorkspaceID, "error", err)
		return nil, err
	}

	return &Result{Messages: messages, NextCursor: cursor}, nil
}
