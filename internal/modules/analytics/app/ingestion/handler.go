package ingestion

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type Options struct {
	FactRepo              ports.EventFactRepository
	Clock                 func() time.Time
	AccessChecker         ports.WorkspaceAccessChecker
	OperationsEventWriter ports.OperationsEventWriter
	Logger                *slog.Logger
}

type Handler struct {
	factRepo              ports.EventFactRepository
	clock                 func() time.Time
	accessChecker         ports.WorkspaceAccessChecker
	operationsEventWriter ports.OperationsEventWriter
	log                   *slog.Logger
}

type IngestFactCommand struct {
	SourceEventID     string
	SourceEventType   string
	WorkspaceID       string
	CampaignID        string
	MessageID         string
	Provider          string
	ProviderMessageID string
	ProviderEventID   string
	CanonicalType     string
	RecipientDomain   string
	OccurredAt        time.Time
	ReceivedAt        time.Time
	Metadata          map[string]any
}

type IngestOpsCommand struct {
	Source          string
	SourceEventID   string
	SourceEventType string
	OperationType   string
	Status          string
	WorkspaceID     string
	ErrorType       string
	Consumer        string
	Target          string
	Metadata        map[string]any
	OccurredAt      time.Time
}

func New(opts Options) *Handler {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		factRepo:              opts.FactRepo,
		clock:                 opts.Clock,
		accessChecker:         opts.AccessChecker,
		operationsEventWriter: opts.OperationsEventWriter,
		log:                   opts.Logger.With("service", "analytics", "handler", "ingestion"),
	}
}

func (h *Handler) ExecuteIngestEmailEventFact(ctx context.Context, cmd IngestFactCommand) error {
	cmd.WorkspaceID = domain.NormalizeString(cmd.WorkspaceID)
	cmd.CampaignID = domain.NormalizeString(cmd.CampaignID)
	cmd.Provider = domain.NormalizeString(cmd.Provider)
	cmd.RecipientDomain = domain.NormalizeString(cmd.RecipientDomain)

	if err := domain.ValidateFact(cmd.WorkspaceID, cmd.SourceEventID, cmd.CanonicalType, cmd.OccurredAt, cmd.ReceivedAt); err != nil {
		return err
	}

	existing, err := h.factRepo.FindBySourceEventID(ctx, cmd.SourceEventID)
	if err != nil && !errors.Is(err, domain.ErrAnalyticsProjectionNotFound) {
		h.log.Error("failed to check existing fact",
			"source_event_id", cmd.SourceEventID,
			"workspace_id", cmd.WorkspaceID,
			"error", err,
		)
		return err
	}
	if existing != nil {
		h.log.Debug("duplicate analytics event, skipping",
			"source_event_id", cmd.SourceEventID,
			"workspace_id", cmd.WorkspaceID,
			"event_type", cmd.CanonicalType,
		)
		return nil
	}

	fact := domain.EmailEventFact{
		SourceEventID:     cmd.SourceEventID,
		SourceEventType:   cmd.SourceEventType,
		WorkspaceID:       cmd.WorkspaceID,
		CampaignID:        cmd.CampaignID,
		MessageID:         cmd.MessageID,
		Provider:          cmd.Provider,
		ProviderMessageID: cmd.ProviderMessageID,
		ProviderEventID:   cmd.ProviderEventID,
		EventType:         cmd.CanonicalType,
		RecipientDomain:   cmd.RecipientDomain,
		OccurredAt:        cmd.OccurredAt,
		ReceivedAt:        cmd.ReceivedAt,
		Metadata:          cmd.Metadata,
		CreatedAt:         h.clock(),
	}

	if err := h.factRepo.Create(ctx, fact); err != nil {
		h.log.Error("failed to create analytics fact",
			"source_event_id", cmd.SourceEventID,
			"workspace_id", cmd.WorkspaceID,
			"error", err,
		)
		return err
	}

	h.log.Info("analytics event fact recorded",
		"source_event_id", cmd.SourceEventID,
		"workspace_id", cmd.WorkspaceID,
		"campaign_id", cmd.CampaignID,
		"message_id", cmd.MessageID,
		"event_type", cmd.CanonicalType,
	)

	return nil
}

func (h *Handler) ExecuteIngestOperationsEvent(ctx context.Context, cmd IngestOpsCommand) error {
	if h.operationsEventWriter == nil {
		return nil
	}
	sourceEventID := cmd.SourceEventID
	if sourceEventID == "" {
		sourceEventID = cmd.Source + "_" + cmd.OperationType + "_" + cmd.OccurredAt.Format(time.RFC3339Nano)
	}
	return h.operationsEventWriter.Create(ctx, ports.OperationsEvent{
		SourceEventID:   sourceEventID,
		Source:          cmd.Source,
		SourceEventType: cmd.SourceEventType,
		OperationType:   cmd.OperationType,
		Status:          cmd.Status,
		WorkspaceID:     cmd.WorkspaceID,
		ErrorType:       cmd.ErrorType,
		Consumer:        cmd.Consumer,
		Target:          cmd.Target,
		Metadata:        cmd.Metadata,
		OccurredAt:      cmd.OccurredAt,
	})
}
