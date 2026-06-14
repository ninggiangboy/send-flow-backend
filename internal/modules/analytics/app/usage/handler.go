package usage

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type Options struct {
	UsageQueryRepo ports.UsageQueryRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type Handler struct {
	usageQueryRepo ports.UsageQueryRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

type UsageTimeSeriesQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Interval    string
}

type UsageFeaturesQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
}

type RiskSignalsQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
}

type SendVolumeForecastQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		usageQueryRepo: opts.UsageQueryRepo,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("service", "analytics", "handler", "usage"),
	}
}

func (h *Handler) ExecuteGetUsageTimeSeries(ctx context.Context, q UsageTimeSeriesQuery) (*domain.UsageTimeSeriesResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if h.usageQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	if q.Interval == "" {
		q.Interval = "day"
	}
	if err := domain.ValidateInterval(q.Interval); err != nil {
		return nil, err
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.usageQueryRepo.GetUsageTimeSeries(ctx, q.WorkspaceID, q.From, q.To, q.Interval)
	if err != nil {
		h.log.Error("failed to get usage time series",
			"workspace_id", q.WorkspaceID,
			"interval", q.Interval,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (h *Handler) ExecuteGetUsageFeatures(ctx context.Context, q UsageFeaturesQuery) (*domain.UsageFeaturesResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if h.usageQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.usageQueryRepo.GetUsageFeatures(ctx, q.WorkspaceID, q.From, q.To)
	if err != nil {
		h.log.Error("failed to get usage features",
			"workspace_id", q.WorkspaceID,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (h *Handler) ExecuteGetRiskSignals(ctx context.Context, q RiskSignalsQuery) (*domain.RiskSignalsResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if h.usageQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.usageQueryRepo.GetRiskSignals(ctx, q.WorkspaceID, q.From, q.To)
	if err != nil {
		h.log.Error("failed to get risk signals",
			"workspace_id", q.WorkspaceID,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (h *Handler) ExecuteGetSendVolumeForecast(ctx context.Context, q SendVolumeForecastQuery) (*domain.SendVolumeForecastResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}
	if h.usageQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.usageQueryRepo.GetSendVolumeForecast(ctx, q.WorkspaceID, q.From, q.To)
	if err != nil {
		h.log.Error("failed to get send volume forecast",
			"workspace_id", q.WorkspaceID,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}
