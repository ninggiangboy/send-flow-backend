package getdelivery

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type Options struct {
	DeliveryRead  ports.DeliveryReadRepository
	AttemptRead   ports.AttemptReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
	DeliveryID  string
}

type Handler struct {
	deliveryRead  ports.DeliveryReadRepository
	attemptRead   ports.AttemptReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		deliveryRead:  opts.DeliveryRead,
		attemptRead:   opts.AttemptRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "get_webhook_delivery"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.WebhookDelivery, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "delivery_id", cmd.DeliveryID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "webhook.delivery.read"); err != nil {
		return nil, err
	}

	delivery, err := h.deliveryRead.FindByID(ctx, cmd.WorkspaceID, cmd.DeliveryID)
	if err != nil {
		if errors.Is(err, domain.ErrDeliveryNotFound) {
			return nil, err
		}
		log.Error("failed to find delivery", "error", err)
		return nil, err
	}

	attempts, err := h.attemptRead.ListByDelivery(ctx, cmd.DeliveryID)
	if err != nil {
		log.Error("failed to list delivery attempts", "error", err)
	}
	delivery.Attempts = attempts

	return delivery, nil
}
