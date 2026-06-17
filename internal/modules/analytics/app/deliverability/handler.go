package deliverability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	analyticsredis "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type Options struct {
	DeliverabilityQueryRepo ports.DeliverabilityQueryRepository
	AccessChecker           ports.WorkspaceAccessChecker
	Logger                  *slog.Logger
	Cache                   *analyticsredis.Cache
}

type Handler struct {
	deliverabilityQueryRepo ports.DeliverabilityQueryRepository
	accessChecker           ports.WorkspaceAccessChecker
	log                     *slog.Logger
	cache                   *analyticsredis.Cache
}

// ── Query structs ──

type DeliverabilityQuery struct {
	WorkspaceID     string
	Provider        string
	RecipientDomain string
	UserID          string
}

type TimeSeriesQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
	Interval    string
}

type BreakdownQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
	GroupBy     string
}

type LatencyQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
}

type IncidentsQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		deliverabilityQueryRepo: opts.DeliverabilityQueryRepo,
		accessChecker:           opts.AccessChecker,
		log:                     opts.Logger.With("service", "analytics", "handler", "deliverability"),
		cache:                   opts.Cache,
	}
}

func (h *Handler) ExecuteGetDeliverability(ctx context.Context, q DeliverabilityQuery) (*domain.DeliverabilityResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.cache != nil {
		filter := domain.DeliverabilityFilter{
			Provider:        domain.NormalizeString(q.Provider),
			RecipientDomain: domain.NormalizeString(q.RecipientDomain),
		}
		cached, err := h.cache.GetOrLoadDeliverability(ctx, q.WorkspaceID, func() (any, error) {
			return h.loadDeliverability(ctx, q.WorkspaceID, filter)
		})
		if err != nil {
			return nil, err
		}
		if result, ok := cached.(*domain.DeliverabilityResult); ok {
			return result, nil
		}
		if result, ok := cached.(domain.DeliverabilityResult); ok {
			return &result, nil
		}
		return nil, nil
	}

	return h.loadDeliverability(ctx, q.WorkspaceID, domain.DeliverabilityFilter{
		Provider:        domain.NormalizeString(q.Provider),
		RecipientDomain: domain.NormalizeString(q.RecipientDomain),
	})
}

func (h *Handler) loadDeliverability(ctx context.Context, workspaceID string, filter domain.DeliverabilityFilter) (*domain.DeliverabilityResult, error) {
	projections, err := h.deliverabilityQueryRepo.ListDeliverability(ctx, workspaceID, filter)
	if err != nil {
		h.log.Error("failed to list deliverability projections",
			"workspace_id", workspaceID,
			"error", err,
		)
		return nil, err
	}

	if len(projections) == 0 {
		return &domain.DeliverabilityResult{
			Status: "pending",
			Items:  nil,
		}, nil
	}

	rows := make([]domain.DeliverabilityRow, 0, len(projections))
	for _, p := range projections {
		rows = append(rows, domain.DeliverabilityRow{
			Provider:        p.Provider,
			RecipientDomain: p.RecipientDomain,
			DeliveredCount:  p.DeliveredCount,
			BouncedCount:    p.BouncedCount,
			ComplainedCount: p.ComplainedCount,
			OpenedCount:     p.OpenedCount,
			ClickedCount:    p.ClickedCount,
			BounceRate:      domain.ComputeRate(p.BouncedCount, p.DeliveredCount),
			ComplaintRate:   domain.ComputeRate(p.ComplainedCount, p.DeliveredCount),
			LastEventAt:     p.LastEventAt,
			LastUpdatedAt:   p.LastUpdatedAt,
		})
	}

	return &domain.DeliverabilityResult{
		Status: "ready",
		Items:  rows,
	}, nil
}

func (h *Handler) ExecuteGetDeliverabilityTimeSeries(ctx context.Context, q TimeSeriesQuery) (*domain.DeliverabilityTimeSeriesResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.deliverabilityQueryRepo == nil {
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

	result, err := h.deliverabilityQueryRepo.GetDeliverabilityTimeSeries(ctx, q.WorkspaceID, q.From, q.To, q.Provider, q.Domain, q.Interval)
	if err != nil {
		h.log.Error("failed to get deliverability time series",
			"workspace_id", q.WorkspaceID,
			"provider", q.Provider,
			"domain", q.Domain,
			"interval", q.Interval,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (h *Handler) ExecuteGetDeliverabilityBreakdown(ctx context.Context, q BreakdownQuery) (*domain.DeliverabilityBreakdownResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.deliverabilityQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	if q.GroupBy == "" {
		q.GroupBy = "provider"
	}
	if err := domain.ValidateGroupBy(q.GroupBy); err != nil {
		return nil, err
	}
	if q.GroupBy == "event_type" {
		return nil, fmt.Errorf("%w: group_by must be provider or recipient_domain for deliverability", domain.ErrAnalyticsQueryInvalid)
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.deliverabilityQueryRepo.GetDeliverabilityBreakdown(ctx, q.WorkspaceID, q.From, q.To, q.GroupBy, q.Provider, q.Domain)
	if err != nil {
		h.log.Error("failed to get deliverability breakdown",
			"workspace_id", q.WorkspaceID,
			"group_by", q.GroupBy,
			"provider", q.Provider,
			"domain", q.Domain,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (h *Handler) ExecuteGetDeliverabilityLatency(ctx context.Context, q LatencyQuery) (*domain.DeliverabilityLatencyResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.deliverabilityQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.deliverabilityQueryRepo.GetDeliverabilityLatency(ctx, q.WorkspaceID, q.From, q.To, q.Provider, q.Domain)
	if err != nil {
		h.log.Error("failed to get deliverability latency",
			"workspace_id", q.WorkspaceID,
			"provider", q.Provider,
			"domain", q.Domain,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (h *Handler) ExecuteGetDeliverabilityIncidents(ctx context.Context, q IncidentsQuery) (*domain.DeliverabilityIncidentResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.deliverabilityQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.deliverabilityQueryRepo.GetDeliverabilityIncidents(ctx, q.WorkspaceID, q.From, q.To, q.Provider, q.Domain)
	if err != nil {
		h.log.Error("failed to get deliverability incidents",
			"workspace_id", q.WorkspaceID,
			"provider", q.Provider,
			"domain", q.Domain,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}
