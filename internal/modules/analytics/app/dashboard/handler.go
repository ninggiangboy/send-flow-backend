package dashboard

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	analyticsredis "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
	platformconstants "github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Options struct {
	WorkspaceQueryRepo ports.WorkspaceQueryRepository
	AccessChecker      ports.WorkspaceAccessChecker
	Logger             *slog.Logger
	Cache              *analyticsredis.Cache
}

type Handler struct {
	workspaceQueryRepo ports.WorkspaceQueryRepository
	accessChecker      ports.WorkspaceAccessChecker
	log                *slog.Logger
	cache              *analyticsredis.Cache
}

type DashboardQuery struct {
	WorkspaceID string
	UserID      string
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		workspaceQueryRepo: opts.WorkspaceQueryRepo,
		accessChecker:      opts.AccessChecker,
		log:                opts.Logger.With("service", "analytics", "handler", "dashboard"),
		cache:              opts.Cache,
	}
}

func (h *Handler) ExecuteGetDashboardOverview(ctx context.Context, q DashboardQuery) (*domain.DashboardOverview, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, platformconstants.PermissionAnalyticsRead); err != nil {
			return nil, err
		}
	}

	if h.cache != nil {
		cached, err := h.cache.GetOrLoadDashboard(ctx, q.WorkspaceID, func() (any, error) {
			return h.loadDashboardOverview(ctx, q.WorkspaceID)
		})
		if err != nil {
			return nil, err
		}
		if overview, ok := cached.(*domain.DashboardOverview); ok {
			return overview, nil
		}
		if overview, ok := cached.(domain.DashboardOverview); ok {
			return &overview, nil
		}
		return nil, nil
	}

	return h.loadDashboardOverview(ctx, q.WorkspaceID)
}

func (h *Handler) loadDashboardOverview(ctx context.Context, workspaceID string) (*domain.DashboardOverview, error) {
	overview, err := h.workspaceQueryRepo.GetWorkspaceOverview(ctx, workspaceID)
	if err != nil {
		if err == domain.ErrAnalyticsProjectionNotFound {
			return &domain.DashboardOverview{
				Status:      "pending",
				WorkspaceID: workspaceID,
			}, nil
		}
		h.log.Error("failed to get workspace overview",
			"workspace_id", workspaceID,
			"error", err,
		)
		return nil, err
	}

	return &domain.DashboardOverview{
		Status:              "ready",
		WorkspaceID:         overview.WorkspaceID,
		QueuedCount:         overview.QueuedCount,
		AcceptedCount:       overview.AcceptedCount,
		DeliveredCount:      overview.DeliveredCount,
		BouncedCount:        overview.BouncedCount,
		ComplainedCount:     overview.ComplainedCount,
		OpenedCount:         overview.OpenedCount,
		ClickedCount:        overview.ClickedCount,
		UnsubscribedCount:   overview.UnsubscribedCount,
		RetryScheduledCount: overview.RetryScheduledCount,
		LastEventAt:         overview.LastEventAt,
		LastUpdatedAt:       overview.LastUpdatedAt,
	}, nil
}
