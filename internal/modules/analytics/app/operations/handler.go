package operations

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type Options struct {
	OperationsQueryRepo ports.OperationsQueryRepository
	AccessChecker       ports.WorkspaceAccessChecker
	Logger              *slog.Logger
}

type Handler struct {
	operationsQueryRepo ports.OperationsQueryRepository
	accessChecker       ports.WorkspaceAccessChecker
	log                 *slog.Logger
}

type OutboxLagQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Source      string
}

type ConsumerFailuresQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Source      string
}

type DLQVolumeQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Source      string
}

type WebhookDeliveryTimeSeriesQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Status      string
	Interval    string
}

type WebhookReliabilityQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Target      string
}

func New(opts Options) *Handler {
	return &Handler{
		operationsQueryRepo: opts.OperationsQueryRepo,
		accessChecker:       opts.AccessChecker,
		log:                 opts.Logger.With("service", "analytics", "handler", "operations"),
	}
}

func (h *Handler) ExecuteGetOutboxLag(ctx context.Context, q OutboxLagQuery) (*domain.OutboxLagResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if h.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.operationsQueryRepo.GetOutboxLag(ctx, q.WorkspaceID, q.From, q.To, q.Source)
	if err != nil {
		h.log.Error("failed to get outbox lag",
			"workspace_id", q.WorkspaceID,
			"source", q.Source,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (h *Handler) ExecuteGetConsumerFailures(ctx context.Context, q ConsumerFailuresQuery) (*domain.ConsumerFailureResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if h.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.operationsQueryRepo.GetConsumerFailures(ctx, q.WorkspaceID, q.From, q.To, q.Source)
	if err != nil {
		h.log.Error("failed to get consumer failures",
			"workspace_id", q.WorkspaceID,
			"source", q.Source,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (h *Handler) ExecuteGetDLQVolume(ctx context.Context, q DLQVolumeQuery) (*domain.DLQResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if h.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.operationsQueryRepo.GetDLQVolume(ctx, q.WorkspaceID, q.From, q.To, q.Source)
	if err != nil {
		h.log.Error("failed to get dlq volume",
			"workspace_id", q.WorkspaceID,
			"source", q.Source,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (h *Handler) ExecuteGetWebhookDeliveryTimeSeries(ctx context.Context, q WebhookDeliveryTimeSeriesQuery) (*domain.WebhookDeliveryTimeSeriesResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if h.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	interval := q.Interval
	if interval == "" {
		interval = "day"
	}
	if err := domain.ValidateInterval(interval); err != nil {
		return nil, err
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.operationsQueryRepo.GetWebhookDeliveryTimeSeries(ctx, q.WorkspaceID, q.From, q.To, q.Status, interval)
	if err != nil {
		h.log.Error("failed to get webhook delivery time series",
			"workspace_id", q.WorkspaceID,
			"status", q.Status,
			"interval", interval,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (h *Handler) ExecuteGetWebhookReliability(ctx context.Context, q WebhookReliabilityQuery) (*domain.WebhookReliabilityResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if h.operationsQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.operationsQueryRepo.GetWebhookReliability(ctx, q.WorkspaceID, q.From, q.To, q.Target)
	if err != nil {
		h.log.Error("failed to get webhook reliability",
			"workspace_id", q.WorkspaceID,
			"target", q.Target,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}
