package listdeliveries

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type Options struct {
	DeliveryRead  ports.DeliveryReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
	Status      string
	EventType   string
	From        *time.Time
	To          *time.Time
	Limit       int
	Cursor      string
}

type Handler struct {
	deliveryRead  ports.DeliveryReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		deliveryRead:  opts.DeliveryRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_webhook_deliveries"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.WebhookDelivery, string, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "webhook.delivery.read"); err != nil {
		return nil, "", err
	}

	filter := ports.DeliveryFilter{
		WebhookID: cmd.WebhookID,
		Status:    cmd.Status,
		EventType: cmd.EventType,
		From:      cmd.From,
		To:        cmd.To,
		Limit:     cmd.Limit,
		Cursor:    cmd.Cursor,
	}

	deliveries, cursor, err := h.deliveryRead.ListByWorkspace(ctx, cmd.WorkspaceID, filter)
	if err != nil {
		log.Error("failed to list deliveries", "error", err)
		return nil, "", err
	}

	return deliveries, cursor, nil
}
