package forensics

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type Options struct {
	ForensicQueryRepo ports.ForensicQueryRepository
	AccessChecker     ports.WorkspaceAccessChecker
	Logger            *slog.Logger
}

type Handler struct {
	forensicQueryRepo ports.ForensicQueryRepository
	accessChecker     ports.WorkspaceAccessChecker
	log               *slog.Logger
}

type SearchQuery struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	EventType   string
	CampaignID  string
	MessageID   string
	Provider    string
	Domain      string
	Status      string
	Limit       int
	Cursor      string
}

type MessageTimelineQuery struct {
	WorkspaceID string
	MessageID   string
	UserID      string
}

type ProviderEventTraceQuery struct {
	WorkspaceID     string
	ProviderEventID string
	UserID          string
}

type IncidentTimelineQuery struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
}

func New(opts Options) *Handler {
	return &Handler{
		forensicQueryRepo: opts.ForensicQueryRepo,
		accessChecker:     opts.AccessChecker,
		log:               opts.Logger.With("service", "analytics", "handler", "forensics"),
	}
}

func (h *Handler) ExecuteSearchEvents(ctx context.Context, q SearchQuery) (*domain.ForensicEventsResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.forensicQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	filter := domain.ForensicQueryFilter{
		WorkspaceID: q.WorkspaceID,
		From:        q.From,
		To:          q.To,
		EventType:   q.EventType,
		Provider:    q.Provider,
		Domain:      q.Domain,
		CampaignID:  q.CampaignID,
		MessageID:   q.MessageID,
		Limit:       q.Limit,
		Cursor:      q.Cursor,
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	if err := filter.Validate(); err != nil {
		return nil, err
	}

	result, err := h.forensicQueryRepo.SearchEvents(ctx, filter)
	if err != nil {
		h.log.Error("failed to search forensic events",
			"workspace_id", q.WorkspaceID,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (h *Handler) ExecuteGetMessageTimeline(ctx context.Context, q MessageTimelineQuery) (*domain.MessageTimelineResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.forensicQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	result, err := h.forensicQueryRepo.GetMessageTimeline(ctx, q.WorkspaceID, q.MessageID)
	if err != nil {
		h.log.Error("failed to get message timeline",
			"workspace_id", q.WorkspaceID,
			"message_id", q.MessageID,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (h *Handler) ExecuteGetProviderEventTrace(ctx context.Context, q ProviderEventTraceQuery) (*domain.ProviderEventTrace, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.forensicQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	result, err := h.forensicQueryRepo.GetProviderEventTrace(ctx, q.WorkspaceID, q.ProviderEventID)
	if err != nil {
		h.log.Error("failed to get provider event trace",
			"workspace_id", q.WorkspaceID,
			"provider_event_id", q.ProviderEventID,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}

func (h *Handler) ExecuteGetCampaignIncidentTimeline(ctx context.Context, q IncidentTimelineQuery) (*domain.CampaignIncidentTimelineResult, error) {
	if h.accessChecker != nil {
		if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "analytics.read"); err != nil {
			return nil, err
		}
	}

	if h.forensicQueryRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	if err := domain.ValidateTimeRange(q.From, q.To); err != nil {
		return nil, err
	}

	result, err := h.forensicQueryRepo.GetCampaignIncidentTimeline(ctx, q.WorkspaceID, q.CampaignID, q.From, q.To)
	if err != nil {
		h.log.Error("failed to get campaign incident timeline",
			"workspace_id", q.WorkspaceID,
			"campaign_id", q.CampaignID,
			"error", err,
		)
		return nil, err
	}

	return result, nil
}
