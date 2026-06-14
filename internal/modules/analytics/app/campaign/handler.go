package campaign

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type Options struct {
	ProjectionRead    ports.ProjectionReadRepository
	CampaignQueryRepo ports.CampaignQueryRepository
	AccessChecker     ports.WorkspaceAccessChecker
	Logger            *slog.Logger
}

type Handler struct {
	projectionRead    ports.ProjectionReadRepository
	campaignQueryRepo ports.CampaignQueryRepository
	accessChecker     ports.WorkspaceAccessChecker
	log               *slog.Logger
}

type CampaignAnalyticsQuery struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
}

type CampaignFunnelQuery struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
}

type CampaignTimeSeriesQuery struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
	Interval    string
	EventType   string
}

type CampaignBreakdownQuery struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
	GroupBy     string
}

type CampaignEventsQuery struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
	EventType   string
	Provider    string
	Domain      string
	Limit       int
	Cursor      string
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		projectionRead:    opts.ProjectionRead,
		campaignQueryRepo: opts.CampaignQueryRepo,
		accessChecker:     opts.AccessChecker,
		log:               opts.Logger.With("service", "analytics", "handler", "campaign"),
	}
}

func (h *Handler) ExecuteCampaignAnalytics(ctx context.Context, q CampaignAnalyticsQuery) (*domain.CampaignAnalytics, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	summary, err := h.projectionRead.GetCampaignSummary(ctx, q.WorkspaceID, q.CampaignID)
	if err != nil {
		if err == domain.ErrAnalyticsProjectionNotFound {
			return &domain.CampaignAnalytics{
				Status:      "pending",
				WorkspaceID: q.WorkspaceID,
				CampaignID:  q.CampaignID,
			}, nil
		}
		h.log.Error("failed to get campaign summary",
			"workspace_id", q.WorkspaceID,
			"campaign_id", q.CampaignID,
			"error", err,
		)
		return nil, err
	}

	delivered := summary.DeliveredCount
	accepted := summary.AcceptedCount

	return &domain.CampaignAnalytics{
		Status:              "ready",
		WorkspaceID:         summary.WorkspaceID,
		CampaignID:          summary.CampaignID,
		QueuedCount:         summary.QueuedCount,
		AcceptedCount:       summary.AcceptedCount,
		DeliveredCount:      summary.DeliveredCount,
		BouncedCount:        summary.BouncedCount,
		ComplainedCount:     summary.ComplainedCount,
		OpenedCount:         summary.OpenedCount,
		ClickedCount:        summary.ClickedCount,
		UnsubscribedCount:   summary.UnsubscribedCount,
		RetryScheduledCount: summary.RetryScheduledCount,
		DeliveryRate:        domain.ComputeRate(delivered, accepted),
		BounceRate:          domain.ComputeRate(summary.BouncedCount, delivered),
		ComplaintRate:       domain.ComputeRate(summary.ComplainedCount, delivered),
		OpenRate:            domain.ComputeRate(summary.OpenedCount, delivered),
		ClickRate:           domain.ComputeRate(summary.ClickedCount, delivered),
		UnsubscribeRate:     domain.ComputeRate(summary.UnsubscribedCount, delivered),
		LastEventAt:         summary.LastEventAt,
		LastUpdatedAt:       summary.LastUpdatedAt,
	}, nil
}

func (h *Handler) ExecuteCampaignFunnel(ctx context.Context, q CampaignFunnelQuery) (*domain.CampaignFunnel, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.campaignQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	funnel, err := h.campaignQueryRepo.GetCampaignFunnel(ctx, q.WorkspaceID, q.CampaignID, q.From, q.To)
	if err != nil {
		h.log.Error("failed to get campaign funnel",
			"workspace_id", q.WorkspaceID,
			"campaign_id", q.CampaignID,
			"error", err,
		)
		return nil, err
	}

	return funnel, nil
}

func (h *Handler) ExecuteCampaignTimeSeries(ctx context.Context, q CampaignTimeSeriesQuery) (*domain.CampaignTimeSeriesResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.campaignQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	if err := domain.ValidateInterval(q.Interval); err != nil {
		return nil, err
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}
	if q.EventType != "" && !domain.KnownEventTypes[q.EventType] {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	ts, err := h.campaignQueryRepo.GetCampaignTimeSeries(ctx, q.WorkspaceID, q.CampaignID, q.From, q.To, q.Interval, q.EventType)
	if err != nil {
		h.log.Error("failed to get campaign time series",
			"workspace_id", q.WorkspaceID,
			"campaign_id", q.CampaignID,
			"interval", q.Interval,
			"error", err,
		)
		return nil, err
	}

	return ts, nil
}

func (h *Handler) ExecuteCampaignBreakdown(ctx context.Context, q CampaignBreakdownQuery) (*domain.CampaignBreakdownResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.campaignQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	if err := domain.ValidateGroupBy(q.GroupBy); err != nil {
		return nil, err
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	bd, err := h.campaignQueryRepo.GetCampaignBreakdown(ctx, q.WorkspaceID, q.CampaignID, q.From, q.To, q.GroupBy)
	if err != nil {
		h.log.Error("failed to get campaign breakdown",
			"workspace_id", q.WorkspaceID,
			"campaign_id", q.CampaignID,
			"group_by", q.GroupBy,
			"error", err,
		)
		return nil, err
	}

	return bd, nil
}

func (h *Handler) ExecuteCampaignEvents(ctx context.Context, q CampaignEventsQuery) (*domain.CampaignEventsResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.campaignQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	filter := domain.CampaignQueryFilter{
		WorkspaceID: q.WorkspaceID,
		CampaignID:  q.CampaignID,
		From:        q.From,
		To:          q.To,
		EventType:   q.EventType,
		Provider:    q.Provider,
		Domain:      q.Domain,
		Limit:       q.Limit,
		Cursor:      q.Cursor,
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	events, err := h.campaignQueryRepo.GetCampaignEvents(ctx, filter)
	if err != nil {
		h.log.Error("failed to get campaign events",
			"workspace_id", q.WorkspaceID,
			"campaign_id", q.CampaignID,
			"error", err,
		)
		return nil, err
	}

	return events, nil
}
