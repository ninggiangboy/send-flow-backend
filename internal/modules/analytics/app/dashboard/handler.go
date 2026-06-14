package dashboard

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type Options struct {
	ProjectionRead ports.ProjectionReadRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type Handler struct {
	projectionRead ports.ProjectionReadRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
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
		projectionRead: opts.ProjectionRead,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("service", "analytics", "handler", "dashboard"),
	}
}

func (h *Handler) ExecuteGetDashboardOverview(ctx context.Context, q DashboardQuery) (*domain.DashboardOverview, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	overview, err := h.projectionRead.GetWorkspaceOverview(ctx, q.WorkspaceID)
	if err != nil {
		if err == domain.ErrAnalyticsProjectionNotFound {
			return &domain.DashboardOverview{
				Status:      "pending",
				WorkspaceID: q.WorkspaceID,
			}, nil
		}
		h.log.Error("failed to get workspace overview",
			"workspace_id", q.WorkspaceID,
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
