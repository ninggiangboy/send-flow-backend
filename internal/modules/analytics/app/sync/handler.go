package sync

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type Options struct {
	FactBatchRepo         ports.FactBatchRepository
	SyncStateRepo         ports.SyncStateRepository
	ClickHouseBatchWriter ports.ClickHouseBatchWriter
	Clock                 func() time.Time
	Logger                *slog.Logger
}

type Handler struct {
	factBatchRepo         ports.FactBatchRepository
	syncStateRepo         ports.SyncStateRepository
	clickHouseBatchWriter ports.ClickHouseBatchWriter
	clock                 func() time.Time
	log                   *slog.Logger
}

type SyncCommand struct {
	StreamName string
	BatchSize  int
}

type SyncResult struct {
	SyncedCount int
	LastFactID  string
	LagSeconds  int64
}

type CursorQuery struct {
	StreamName string
}

type CursorStatusResult struct {
	StreamName   string
	LastSyncedAt *time.Time
	LastFactID   string
	LagSeconds   int64
}

func New(opts Options) *Handler {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		factBatchRepo:         opts.FactBatchRepo,
		syncStateRepo:         opts.SyncStateRepo,
		clickHouseBatchWriter: opts.ClickHouseBatchWriter,
		clock:                 opts.Clock,
		log:                   opts.Logger.With("service", "analytics", "handler", "sync"),
	}
}

func (h *Handler) ExecuteSyncFactsToClickHouse(ctx context.Context, cmd SyncCommand) (*SyncResult, error) {
	if h.syncStateRepo == nil || h.factBatchRepo == nil || h.clickHouseBatchWriter == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}

	cursor, err := h.syncStateRepo.GetSyncCursor(ctx, cmd.StreamName)
	if err != nil {
		h.log.Error("failed to get sync cursor",
			"stream_name", cmd.StreamName,
			"error", err,
		)
		return nil, err
	}

	batchSize := cmd.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	facts, err := h.factBatchRepo.ListFactsAfterCursor(ctx, cursor.LastCreatedAt, cursor.LastFactID, batchSize)
	if err != nil {
		h.log.Error("failed to list facts after cursor",
			"stream_name", cmd.StreamName,
			"error", err,
		)
		return nil, err
	}

	if len(facts) == 0 {
		return &SyncResult{}, nil
	}

	if err := h.clickHouseBatchWriter.CreateBatch(ctx, facts); err != nil {
		h.log.Error("failed to write batch to clickhouse",
			"stream_name", cmd.StreamName,
			"batch_size", len(facts),
			"error", err,
		)
		return nil, err
	}

	last := facts[len(facts)-1]
	now := h.clock()

	lagSeconds := int64(0)
	if cursor.LastSyncedAt != nil {
		lagSeconds = int64(now.Sub(*cursor.LastSyncedAt).Seconds())
	}

	cursor.LastCreatedAt = &last.CreatedAt
	cursor.LastFactID = last.ID
	cursor.LastSyncedAt = &now

	if err := h.syncStateRepo.UpdateSyncCursor(ctx, cursor); err != nil {
		h.log.Error("failed to update sync cursor",
			"stream_name", cmd.StreamName,
			"error", err,
		)
		return nil, err
	}

	return &SyncResult{
		SyncedCount: len(facts),
		LastFactID:  last.ID,
		LagSeconds:  lagSeconds,
	}, nil
}

func (h *Handler) ExecuteGetSyncCursorStatus(ctx context.Context, cmd CursorQuery) (*CursorStatusResult, error) {
	if h.syncStateRepo == nil {
		return nil, domain.ErrAnalyticsStoreUnavailable
	}
	cursor, err := h.syncStateRepo.GetSyncCursor(ctx, cmd.StreamName)
	if err != nil {
		return nil, err
	}
	lagSeconds := int64(0)
	if cursor.LastSyncedAt != nil {
		lagSeconds = int64(h.clock().Sub(*cursor.LastSyncedAt).Seconds())
	}
	return &CursorStatusResult{
		StreamName:   cursor.StreamName,
		LastSyncedAt: cursor.LastSyncedAt,
		LastFactID:   cursor.LastFactID,
		LagSeconds:   lagSeconds,
	}, nil
}
