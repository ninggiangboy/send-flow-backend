package app

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/createreplayjob"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/getdeadletterrecord"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/getoutboxrecord"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/getoutboxsummary"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/getreplayjob"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/listdeadletterrecords"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/listoutboxrecords"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/listreplayjobs"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app/runreplayjob"
)

// CommandBus sends commands that mutate state.
type CommandBus interface {
	CreateReplayJob(ctx context.Context, input createreplayjob.Input) (*ReplayJobResult, error)
	RunReplayJob(ctx context.Context, input runreplayjob.Input) (*ReplayJobResult, error)
}

// QueryBus dispatches read-only queries.
type QueryBus interface {
	GetOutboxSummary(ctx context.Context, input getoutboxsummary.Input) (OutboxSummaryResult, error)
	ListOutboxRecords(ctx context.Context, input listoutboxrecords.Input) ([]OutboxEventResult, string, error)
	GetOutboxRecord(ctx context.Context, input getoutboxrecord.Input) (*OutboxEventResult, error)
	ListDeadLetterRecords(ctx context.Context, input listdeadletterrecords.Input) ([]DeadLetterResult, string, error)
	GetDeadLetterRecord(ctx context.Context, input getdeadletterrecord.Input) (*DeadLetterResult, error)
	GetReplayJob(ctx context.Context, input getreplayjob.Input) (*ReplayJobResult, error)
	ListReplayJobs(ctx context.Context, input listreplayjobs.Input) ([]ReplayJobResult, string, error)
}

type commandBus struct {
	createReplayJob *createreplayjob.Handler
	runReplayJob    *runreplayjob.Handler
}

func newCommandBus(
	createReplayJobH *createreplayjob.Handler,
	runReplayJobH *runreplayjob.Handler,
) CommandBus {
	return &commandBus{
		createReplayJob: createReplayJobH,
		runReplayJob:    runReplayJobH,
	}
}

func (b *commandBus) CreateReplayJob(ctx context.Context, input createreplayjob.Input) (*ReplayJobResult, error) {
	job, err := b.createReplayJob.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	result := ReplayJobToResult(*job)
	return &result, nil
}

func (b *commandBus) RunReplayJob(ctx context.Context, input runreplayjob.Input) (*ReplayJobResult, error) {
	job, err := b.runReplayJob.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	result := ReplayJobToResult(*job)
	return &result, nil
}

type queryBus struct {
	getOutboxSummary      *getoutboxsummary.Handler
	listOutboxRecords     *listoutboxrecords.Handler
	getOutboxRecord       *getoutboxrecord.Handler
	listDeadLetterRecords *listdeadletterrecords.Handler
	getDeadLetterRecord   *getdeadletterrecord.Handler
	getReplayJob          *getreplayjob.Handler
	listReplayJobs        *listreplayjobs.Handler
}

func newQueryBus(
	getOutboxSummaryH *getoutboxsummary.Handler,
	listOutboxRecordsH *listoutboxrecords.Handler,
	getOutboxRecordH *getoutboxrecord.Handler,
	listDeadLetterRecordsH *listdeadletterrecords.Handler,
	getDeadLetterRecordH *getdeadletterrecord.Handler,
	getReplayJobH *getreplayjob.Handler,
	listReplayJobsH *listreplayjobs.Handler,
) QueryBus {
	return &queryBus{
		getOutboxSummary:      getOutboxSummaryH,
		listOutboxRecords:     listOutboxRecordsH,
		getOutboxRecord:       getOutboxRecordH,
		listDeadLetterRecords: listDeadLetterRecordsH,
		getDeadLetterRecord:   getDeadLetterRecordH,
		getReplayJob:          getReplayJobH,
		listReplayJobs:        listReplayJobsH,
	}
}

func (b *queryBus) GetOutboxSummary(ctx context.Context, input getoutboxsummary.Input) (OutboxSummaryResult, error) {
	summary, err := b.getOutboxSummary.Execute(ctx, input)
	if err != nil {
		return OutboxSummaryResult{}, err
	}
	return OutboxSummaryToResult(summary), nil
}

func (b *queryBus) ListOutboxRecords(ctx context.Context, input listoutboxrecords.Input) ([]OutboxEventResult, string, error) {
	records, cursor, err := b.listOutboxRecords.Execute(ctx, input)
	if err != nil {
		return nil, "", err
	}
	results := make([]OutboxEventResult, len(records))
	for i := range records {
		results[i] = OutboxRecordToResult(records[i])
	}
	return results, cursor, nil
}

func (b *queryBus) GetOutboxRecord(ctx context.Context, input getoutboxrecord.Input) (*OutboxEventResult, error) {
	rec, err := b.getOutboxRecord.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	result := OutboxRecordToResult(*rec)
	return &result, nil
}

func (b *queryBus) ListDeadLetterRecords(ctx context.Context, input listdeadletterrecords.Input) ([]DeadLetterResult, string, error) {
	records, cursor, err := b.listDeadLetterRecords.Execute(ctx, input)
	if err != nil {
		return nil, "", err
	}
	results := make([]DeadLetterResult, len(records))
	for i := range records {
		results[i] = DeadLetterToResult(records[i])
	}
	return results, cursor, nil
}

func (b *queryBus) GetDeadLetterRecord(ctx context.Context, input getdeadletterrecord.Input) (*DeadLetterResult, error) {
	rec, err := b.getDeadLetterRecord.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	result := DeadLetterToResult(*rec)
	return &result, nil
}

func (b *queryBus) GetReplayJob(ctx context.Context, input getreplayjob.Input) (*ReplayJobResult, error) {
	job, err := b.getReplayJob.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	result := ReplayJobToResult(*job)
	return &result, nil
}

func (b *queryBus) ListReplayJobs(ctx context.Context, input listreplayjobs.Input) ([]ReplayJobResult, string, error) {
	jobs, cursor, err := b.listReplayJobs.Execute(ctx, input)
	if err != nil {
		return nil, "", err
	}
	results := make([]ReplayJobResult, len(jobs))
	for i := range jobs {
		results[i] = ReplayJobToResult(jobs[i])
	}
	return results, cursor, nil
}
