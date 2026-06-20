package anomaly

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
	platformconstants "github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Options struct {
	UsageQueryRepo         ports.UsageQueryRepository
	AnomalySignalWriteRepo ports.AnomalySignalWriteRepository
	AccessChecker          ports.WorkspaceAccessChecker
	Logger                 *slog.Logger
	Clock                  func() time.Time
}

type Handler struct {
	usageQueryRepo         ports.UsageQueryRepository
	anomalySignalWriteRepo ports.AnomalySignalWriteRepository
	accessChecker          ports.WorkspaceAccessChecker
	log                    *slog.Logger
	clock                  func() time.Time
}

type AnomaliesQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
}

func New(opts Options) *Handler {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		usageQueryRepo:         opts.UsageQueryRepo,
		anomalySignalWriteRepo: opts.AnomalySignalWriteRepo,
		accessChecker:          opts.AccessChecker,
		log:                    opts.Logger.With("service", "analytics", "handler", "anomaly"),
		clock:                  opts.Clock,
	}
}

func (h *Handler) ExecuteGetAnomalies(ctx context.Context, q AnomaliesQuery) (*domain.AnomaliesResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, platformconstants.PermissionAnalyticsRead); err != nil {
			return nil, err
		}
	}
	if h.usageQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.usageQueryRepo.GetAnomalies(ctx, q.WorkspaceID, q.From, q.To)
	if err != nil {
		h.log.Error("failed to get anomalies",
			"workspace_id", q.WorkspaceID,
			"error", err,
		)
		return nil, err
	}
	return result, nil
}

func (h *Handler) ExecuteDetectAnomaliesAllWorkspaces(ctx context.Context) error {
	log := h.log.With("usecase", "detect_anomalies_all_workspaces")

	if h.usageQueryRepo == nil {
		log.Warn("usage query repo not available, skipping anomaly detection")
		return nil
	}
	if h.anomalySignalWriteRepo == nil {
		log.Warn("anomaly signal write repo not available, skipping anomaly detection")
		return nil
	}

	now := h.clock()
	since := now.Add(-30 * 24 * time.Hour)

	workspaces, err := h.usageQueryRepo.ListDistinctWorkspaces(ctx, since)
	if err != nil {
		log.Error("failed to list workspaces", "error", err)
		return err
	}

	if len(workspaces) == 0 {
		log.Info("no workspaces with recent activity, skipping detection")
		return nil
	}

	log.Info("detecting anomalies across workspaces", "workspace_count", len(workspaces))

	detectFrom := now.Add(-7 * 24 * time.Hour)
	detectTo := now

	for _, ws := range workspaces {
		wsLog := log.With("workspace_id", ws)

		result, err := h.usageQueryRepo.GetAnomalies(ctx, ws, detectFrom, detectTo)
		if err != nil {
			wsLog.Error("failed to detect anomalies", "error", err)
			continue
		}

		if len(result.Anomalies) == 0 {
			wsLog.Debug("no anomalies detected")
			continue
		}

		if err := h.anomalySignalWriteRepo.SaveAnomalySignals(ctx, ws, result.Anomalies); err != nil {
			wsLog.Error("failed to persist anomaly signals", "error", err)
			continue
		}

		wsLog.Info("anomalies detected and persisted", "count", len(result.Anomalies))
	}

	return nil
}
