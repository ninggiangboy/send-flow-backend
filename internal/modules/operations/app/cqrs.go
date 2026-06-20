package app

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/deadletter"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/outbox"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/replay"
)

// CommandBus sends commands that mutate state.
type CommandBus interface {
	CreateReplayJob(ctx context.Context, input CreateReplayJobInput) (*ReplayJobResult, error)
	RunReplayJob(ctx context.Context, input RunReplayJobInput) (*ReplayJobResult, error)
}

// QueryBus dispatches read-only queries.
type QueryBus interface {
	GetOutboxSummary(ctx context.Context, input GetOutboxSummaryInput) (OutboxSummaryResult, error)
	ListOutboxRecords(ctx context.Context, input ListOutboxRecordsInput) ([]OutboxEventResult, string, error)
	GetOutboxRecord(ctx context.Context, input GetOutboxRecordInput) (*OutboxEventResult, error)
	ListDeadLetterRecords(ctx context.Context, input ListDeadLetterRecordsInput) ([]DeadLetterResult, string, error)
	GetDeadLetterRecord(ctx context.Context, input GetDeadLetterRecordInput) (*DeadLetterResult, error)
	GetReplayJob(ctx context.Context, input GetReplayJobInput) (*ReplayJobResult, error)
	ListReplayJobs(ctx context.Context, input ListReplayJobsInput) ([]ReplayJobResult, string, error)
}

type commandBus struct {
	createReplayJob *replay.CreateHandler
	runReplayJob    *replay.RunHandler
}

func newCommandBus(
	createReplayJobH *replay.CreateHandler,
	runReplayJobH *replay.RunHandler,
) CommandBus {
	return &commandBus{
		createReplayJob: createReplayJobH,
		runReplayJob:    runReplayJobH,
	}
}

func (b *commandBus) CreateReplayJob(ctx context.Context, input replay.CreateInput) (*ReplayJobResult, error) {
	job, err := b.createReplayJob.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	result := ReplayJobToResult(*job)
	return &result, nil
}

func (b *commandBus) RunReplayJob(ctx context.Context, input replay.RunInput) (*ReplayJobResult, error) {
	job, err := b.runReplayJob.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	result := ReplayJobToResult(*job)
	return &result, nil
}

type queryBus struct {
	outboxQS     *outbox.QueryService
	deadletterQS *deadletter.QueryService
	replayQS     *replay.QueryService
}

func newQueryBus(
	outboxQS *outbox.QueryService,
	deadletterQS *deadletter.QueryService,
	replayQS *replay.QueryService,
) QueryBus {
	return &queryBus{
		outboxQS:     outboxQS,
		deadletterQS: deadletterQS,
		replayQS:     replayQS,
	}
}

func (b *queryBus) GetOutboxSummary(ctx context.Context, input outbox.SummaryInput) (OutboxSummaryResult, error) {
	summary, err := b.outboxQS.GetSummary(ctx, input)
	if err != nil {
		return OutboxSummaryResult{}, err
	}
	return OutboxSummaryToResult(summary), nil
}

func (b *queryBus) ListOutboxRecords(ctx context.Context, input outbox.ListInput) ([]OutboxEventResult, string, error) {
	records, cursor, err := b.outboxQS.ListRecords(ctx, input)
	if err != nil {
		return nil, "", err
	}
	results := make([]OutboxEventResult, len(records))
	for i := range records {
		results[i] = OutboxRecordToResult(records[i])
	}
	return results, cursor, nil
}

func (b *queryBus) GetOutboxRecord(ctx context.Context, input outbox.GetInput) (*OutboxEventResult, error) {
	rec, err := b.outboxQS.GetRecord(ctx, input)
	if err != nil {
		return nil, err
	}
	result := OutboxRecordToResult(*rec)
	return &result, nil
}

func (b *queryBus) ListDeadLetterRecords(ctx context.Context, input deadletter.ListInput) ([]DeadLetterResult, string, error) {
	records, cursor, err := b.deadletterQS.ListRecords(ctx, input)
	if err != nil {
		return nil, "", err
	}
	results := make([]DeadLetterResult, len(records))
	for i := range records {
		results[i] = DeadLetterToResult(records[i])
	}
	return results, cursor, nil
}

func (b *queryBus) GetDeadLetterRecord(ctx context.Context, input deadletter.GetInput) (*DeadLetterResult, error) {
	rec, err := b.deadletterQS.GetRecord(ctx, input)
	if err != nil {
		return nil, err
	}
	result := DeadLetterToResult(*rec)
	return &result, nil
}

func (b *queryBus) GetReplayJob(ctx context.Context, input replay.GetInput) (*ReplayJobResult, error) {
	job, err := b.replayQS.GetJob(ctx, input)
	if err != nil {
		return nil, err
	}
	result := ReplayJobToResult(*job)
	return &result, nil
}

func (b *queryBus) ListReplayJobs(ctx context.Context, input replay.ListInput) ([]ReplayJobResult, string, error) {
	jobs, cursor, err := b.replayQS.ListJobs(ctx, input)
	if err != nil {
		return nil, "", err
	}
	results := make([]ReplayJobResult, len(jobs))
	for i := range jobs {
		results[i] = ReplayJobToResult(jobs[i])
	}
	return results, cursor, nil
}
