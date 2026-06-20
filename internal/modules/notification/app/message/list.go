package message

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

type ListOptions struct {
	MessagesRead  ports.MessageReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type ListQuery struct {
	Filter domain.NotificationFilter
	UserID string
}

type ListResult struct {
	Messages   []domain.NotificationMessage
	NextCursor string
}

type ListHandler struct {
	messagesRead  ports.MessageReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewList(opts ListOptions) *ListHandler {
	return &ListHandler{
		messagesRead:  opts.MessagesRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_notifications"),
	}
}

func (h *ListHandler) Execute(ctx context.Context, q ListQuery) (*ListResult, error) {
	if q.Filter.WorkspaceID != nil {
		if h.accessChecker != nil {
			if err := h.accessChecker.RequirePermission(ctx, *q.Filter.WorkspaceID, q.UserID, "notification.read"); err != nil {
				return nil, err
			}
		}
	}

	if q.Filter.Limit <= 0 || q.Filter.Limit > 100 {
		q.Filter.Limit = 50
	}

	msgs, cursor, err := h.messagesRead.List(ctx, q.Filter)
	if err != nil {
		h.log.Error("failed to list notifications", "error", err)
		return nil, err
	}

	return &ListResult{
		Messages:   msgs,
		NextCursor: cursor,
	}, nil
}
