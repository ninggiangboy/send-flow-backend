package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/deadletter"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/outbox"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/replay"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
)

// Options for the operations service facade.
type Options struct {
	OutboxRepo     ports.OutboxRepository
	DeadLetterRepo ports.DeadLetterRepository
	ReplayJobRepo  ports.ReplayJobRepository
	TxManager      ports.TransactionManager
	OutboxWriter   ports.OutboxWriter
	AccessChecker  ports.WorkspaceAccessChecker
	IDGen          func() (string, error)
	Clock          func() time.Time
	Logger         *slog.Logger
}

// Service is the application facade for the operations module.
type Service struct {
	commands CommandBus
	queries  QueryBus
}

func NewService(opts Options) *Service {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	logger := opts.Logger.With("module", "operations")

	outboxQS := outbox.NewQueryService(outbox.Options{
		OutboxRepo:    opts.OutboxRepo,
		AccessChecker: opts.AccessChecker,
		Logger:        logger,
	})
	deadletterQS := deadletter.NewQueryService(deadletter.Options{
		DeadLetterRepo: opts.DeadLetterRepo,
		AccessChecker:  opts.AccessChecker,
		Logger:         logger,
	})
	createReplayJobH := replay.NewCreateHandler(replay.CreateOptions{
		DeadLetterRepo: opts.DeadLetterRepo,
		OutboxRepo:     opts.OutboxRepo,
		ReplayJobRepo:  opts.ReplayJobRepo,
		OutboxWriter:   opts.OutboxWriter,
		AccessChecker:  opts.AccessChecker,
		IDGen:          opts.IDGen,
		Clock:          opts.Clock,
		Logger:         logger,
	})
	runReplayJobH := replay.NewRunHandler(replay.RunOptions{
		DeadLetterRepo: opts.DeadLetterRepo,
		OutboxRepo:     opts.OutboxRepo,
		ReplayJobRepo:  opts.ReplayJobRepo,
		OutboxWriter:   opts.OutboxWriter,
		AccessChecker:  opts.AccessChecker,
		IDGen:          opts.IDGen,
		Clock:          opts.Clock,
		Logger:         logger,
	})
	replayQS := replay.NewQueryService(replay.QueryOptions{
		ReplayJobRepo: opts.ReplayJobRepo,
		AccessChecker: opts.AccessChecker,
		Logger:        logger,
	})

	return &Service{
		commands: newCommandBus(createReplayJobH, runReplayJobH),
		queries:  newQueryBus(outboxQS, deadletterQS, replayQS),
	}
}

// --- Facade Methods ---

func (s *Service) GetOutboxSummary(ctx context.Context, input GetOutboxSummaryInput) (OutboxSummaryResult, error) {
	return s.queries.GetOutboxSummary(ctx, input)
}

func (s *Service) ListOutboxRecords(ctx context.Context, input ListOutboxRecordsInput) ([]OutboxEventResult, string, error) {
	return s.queries.ListOutboxRecords(ctx, input)
}

func (s *Service) GetOutboxRecord(ctx context.Context, input GetOutboxRecordInput) (*OutboxEventResult, error) {
	return s.queries.GetOutboxRecord(ctx, input)
}

func (s *Service) ListDeadLetterRecords(ctx context.Context, input ListDeadLetterRecordsInput) ([]DeadLetterResult, string, error) {
	return s.queries.ListDeadLetterRecords(ctx, input)
}

func (s *Service) GetDeadLetterRecord(ctx context.Context, input GetDeadLetterRecordInput) (*DeadLetterResult, error) {
	return s.queries.GetDeadLetterRecord(ctx, input)
}

func (s *Service) CreateReplayJob(ctx context.Context, input CreateReplayJobInput) (*ReplayJobResult, error) {
	return s.commands.CreateReplayJob(ctx, input)
}

func (s *Service) RunReplayJob(ctx context.Context, input RunReplayJobInput) (*ReplayJobResult, error) {
	return s.commands.RunReplayJob(ctx, input)
}

func (s *Service) GetReplayJob(ctx context.Context, input GetReplayJobInput) (*ReplayJobResult, error) {
	return s.queries.GetReplayJob(ctx, input)
}

func (s *Service) ListReplayJobs(ctx context.Context, input ListReplayJobsInput) ([]ReplayJobResult, string, error) {
	return s.queries.ListReplayJobs(ctx, input)
}
