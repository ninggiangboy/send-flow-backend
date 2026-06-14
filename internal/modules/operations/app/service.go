package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/createreplayjob"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/getdeadletterrecord"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/getoutboxrecord"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/getoutboxsummary"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/getreplayjob"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/listdeadletterrecords"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/listoutboxrecords"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/listreplayjobs"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/runreplayjob"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
)

// --- Shared Result Types ---

type OutboxEventResult struct {
	ID            string          `json:"id"`
	WorkspaceID   string          `json:"workspace_id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	Headers       json.RawMessage `json:"headers"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

type DeadLetterResult struct {
	ID           string          `json:"id"`
	WorkspaceID  string          `json:"workspace_id"`
	Source       string          `json:"source"`
	EventID      string          `json:"event_id"`
	Payload      json.RawMessage `json:"payload"`
	ErrorMessage string          `json:"error_message"`
	Retryable    bool            `json:"retryable"`
	FailedAt     time.Time       `json:"failed_at"`
}

type ReplayJobResult struct {
	ID                string                  `json:"id"`
	WorkspaceID       string                  `json:"workspace_id"`
	TargetType        domain.ReplayTargetType `json:"target_type"`
	TargetID          string                  `json:"target_id"`
	Source            string                  `json:"source"`
	Status            domain.ReplayJobStatus  `json:"status"`
	RequestedByUserID string                  `json:"requested_by_user_id"`
	Reason            string                  `json:"reason"`
	Filter            json.RawMessage         `json:"filter"`
	Result            json.RawMessage         `json:"result"`
	ErrorMessage      string                  `json:"error_message"`
	CreatedAt         time.Time               `json:"created_at"`
	StartedAt         *time.Time              `json:"started_at"`
	CompletedAt       *time.Time              `json:"completed_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

type OutboxSummaryResult struct {
	TotalCount   int            `json:"total_count"`
	OldestAgeSec int64          `json:"oldest_age_seconds"`
	OldestAt     *time.Time     `json:"oldest_at,omitempty"`
	ByEventType  map[string]int `json:"by_event_type,omitempty"`
}

// --- Exported Mappers ---

func OutboxRecordToResult(r domain.OutboxRecord) OutboxEventResult {
	return OutboxEventResult{
		ID:            r.ID,
		WorkspaceID:   r.WorkspaceID,
		AggregateType: r.AggregateType,
		AggregateID:   r.AggregateID,
		EventType:     r.EventType,
		Payload:       r.Payload,
		Headers:       r.Headers,
		OccurredAt:    r.OccurredAt,
		CreatedAt:     r.CreatedAt,
	}
}

func DeadLetterToResult(r domain.DeadLetterRecord) DeadLetterResult {
	return DeadLetterResult{
		ID:           r.ID,
		WorkspaceID:  r.WorkspaceID,
		Source:       r.Source,
		EventID:      r.EventID,
		Payload:      r.Payload,
		ErrorMessage: r.ErrorMessage,
		Retryable:    r.Retryable,
		FailedAt:     r.FailedAt,
	}
}

func ReplayJobToResult(j domain.ReplayJob) ReplayJobResult {
	return ReplayJobResult{
		ID:                j.ID,
		WorkspaceID:       j.WorkspaceID,
		TargetType:        j.TargetType,
		TargetID:          j.TargetID,
		Source:            j.Source,
		Status:            j.Status,
		RequestedByUserID: j.RequestedByUserID,
		Reason:            j.Reason,
		Filter:            j.Filter,
		Result:            j.Result,
		ErrorMessage:      j.ErrorMessage,
		CreatedAt:         j.CreatedAt,
		StartedAt:         j.StartedAt,
		CompletedAt:       j.CompletedAt,
		UpdatedAt:         j.UpdatedAt,
	}
}

func OutboxSummaryToResult(s domain.OutboxSummary) OutboxSummaryResult {
	return OutboxSummaryResult{
		TotalCount:   s.TotalCount,
		OldestAgeSec: s.OldestAgeSec,
		OldestAt:     s.OldestAt,
		ByEventType:  s.ByEventType,
	}
}

// --- Input Type Aliases (for API layer backward compat) ---

type GetOutboxSummaryInput = getoutboxsummary.Input
type ListOutboxRecordsInput = listoutboxrecords.Input
type GetOutboxRecordInput = getoutboxrecord.Input
type ListDeadLetterRecordsInput = listdeadletterrecords.Input
type GetDeadLetterRecordInput = getdeadletterrecord.Input
type CreateReplayJobInput = createreplayjob.Input
type RunReplayJobInput = runreplayjob.Input
type GetReplayJobInput = getreplayjob.Input
type ListReplayJobsInput = listreplayjobs.Input

// --- Options ---

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

// --- Service (facade) ---

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

	getOutboxSummaryH := getoutboxsummary.New(getoutboxsummary.Options{
		OutboxRepo:    opts.OutboxRepo,
		AccessChecker: opts.AccessChecker,
		Logger:        logger,
	})
	listOutboxRecordsH := listoutboxrecords.New(listoutboxrecords.Options{
		OutboxRepo:    opts.OutboxRepo,
		AccessChecker: opts.AccessChecker,
		Logger:        logger,
	})
	getOutboxRecordH := getoutboxrecord.New(getoutboxrecord.Options{
		OutboxRepo:    opts.OutboxRepo,
		AccessChecker: opts.AccessChecker,
		Logger:        logger,
	})
	listDeadLetterRecordsH := listdeadletterrecords.New(listdeadletterrecords.Options{
		DeadLetterRepo: opts.DeadLetterRepo,
		AccessChecker:  opts.AccessChecker,
		Logger:         logger,
	})
	getDeadLetterRecordH := getdeadletterrecord.New(getdeadletterrecord.Options{
		DeadLetterRepo: opts.DeadLetterRepo,
		AccessChecker:  opts.AccessChecker,
		Logger:         logger,
	})
	createReplayJobH := createreplayjob.New(createreplayjob.Options{
		DeadLetterRepo: opts.DeadLetterRepo,
		OutboxRepo:     opts.OutboxRepo,
		ReplayJobRepo:  opts.ReplayJobRepo,
		OutboxWriter:   opts.OutboxWriter,
		AccessChecker:  opts.AccessChecker,
		IDGen:          opts.IDGen,
		Clock:          opts.Clock,
		Logger:         logger,
	})
	runReplayJobH := runreplayjob.New(runreplayjob.Options{
		DeadLetterRepo: opts.DeadLetterRepo,
		OutboxRepo:     opts.OutboxRepo,
		ReplayJobRepo:  opts.ReplayJobRepo,
		OutboxWriter:   opts.OutboxWriter,
		AccessChecker:  opts.AccessChecker,
		IDGen:          opts.IDGen,
		Clock:          opts.Clock,
		Logger:         logger,
	})
	getReplayJobH := getreplayjob.New(getreplayjob.Options{
		ReplayJobRepo: opts.ReplayJobRepo,
		AccessChecker: opts.AccessChecker,
		Logger:        logger,
	})
	listReplayJobsH := listreplayjobs.New(listreplayjobs.Options{
		ReplayJobRepo: opts.ReplayJobRepo,
		AccessChecker: opts.AccessChecker,
		Logger:        logger,
	})

	return &Service{
		commands: newCommandBus(
			createReplayJobH,
			runReplayJobH,
		),
		queries: newQueryBus(
			getOutboxSummaryH,
			listOutboxRecordsH,
			getOutboxRecordH,
			listDeadLetterRecordsH,
			getDeadLetterRecordH,
			getReplayJobH,
			listReplayJobsH,
		),
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
