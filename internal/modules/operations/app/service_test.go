package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

// ---------------------------------------------------------------------------
// Stub types
// ---------------------------------------------------------------------------

type stubOutboxRepo struct {
	getSummaryFn         func(ctx context.Context, workspaceID string, filter domain.OutboxFilter) (domain.OutboxSummary, error)
	findByIDFn           func(ctx context.Context, workspaceID, outboxID string) (*domain.OutboxRecord, error)
	listFn               func(ctx context.Context, workspaceID string, filter domain.OutboxFilter) ([]domain.OutboxRecord, string, error)
	createReplayOutboxFn func(ctx context.Context, event domain.OutboxRecord) error
}

func (s *stubOutboxRepo) GetSummary(ctx context.Context, workspaceID string, filter domain.OutboxFilter) (domain.OutboxSummary, error) {
	return s.getSummaryFn(ctx, workspaceID, filter)
}
func (s *stubOutboxRepo) FindByID(ctx context.Context, workspaceID, outboxID string) (*domain.OutboxRecord, error) {
	return s.findByIDFn(ctx, workspaceID, outboxID)
}
func (s *stubOutboxRepo) List(ctx context.Context, workspaceID string, filter domain.OutboxFilter) ([]domain.OutboxRecord, string, error) {
	return s.listFn(ctx, workspaceID, filter)
}
func (s *stubOutboxRepo) CreateReplayOutboxEvent(ctx context.Context, event domain.OutboxRecord) error {
	return s.createReplayOutboxFn(ctx, event)
}

type stubDeadLetterRepo struct {
	findByIDFn func(ctx context.Context, workspaceID, id string) (*domain.DeadLetterRecord, error)
	listFn     func(ctx context.Context, workspaceID string, filter domain.DeadLetterFilter) ([]domain.DeadLetterRecord, string, error)
}

func (s *stubDeadLetterRepo) FindByID(ctx context.Context, workspaceID, id string) (*domain.DeadLetterRecord, error) {
	return s.findByIDFn(ctx, workspaceID, id)
}
func (s *stubDeadLetterRepo) List(ctx context.Context, workspaceID string, filter domain.DeadLetterFilter) ([]domain.DeadLetterRecord, string, error) {
	return s.listFn(ctx, workspaceID, filter)
}

type stubReplayJobRepo struct {
	createFn        func(ctx context.Context, job domain.ReplayJob) error
	findByIDFn      func(ctx context.Context, workspaceID, jobID string) (*domain.ReplayJob, error)
	listFn          func(ctx context.Context, workspaceID string, filter domain.ReplayJobFilter) ([]domain.ReplayJob, string, error)
	markRunningFn   func(ctx context.Context, workspaceID, jobID string, startedAt time.Time) error
	markCompletedFn func(ctx context.Context, workspaceID, jobID string, result map[string]any, completedAt time.Time) error
	markFailedFn    func(ctx context.Context, workspaceID, jobID, errorMessage string, completedAt time.Time) error
}

func (s *stubReplayJobRepo) Create(ctx context.Context, job domain.ReplayJob) error {
	return s.createFn(ctx, job)
}
func (s *stubReplayJobRepo) FindByID(ctx context.Context, workspaceID, jobID string) (*domain.ReplayJob, error) {
	return s.findByIDFn(ctx, workspaceID, jobID)
}
func (s *stubReplayJobRepo) List(ctx context.Context, workspaceID string, filter domain.ReplayJobFilter) ([]domain.ReplayJob, string, error) {
	return s.listFn(ctx, workspaceID, filter)
}
func (s *stubReplayJobRepo) MarkRunning(ctx context.Context, workspaceID, jobID string, startedAt time.Time) error {
	return s.markRunningFn(ctx, workspaceID, jobID, startedAt)
}
func (s *stubReplayJobRepo) MarkCompleted(ctx context.Context, workspaceID, jobID string, result map[string]any, completedAt time.Time) error {
	return s.markCompletedFn(ctx, workspaceID, jobID, result, completedAt)
}
func (s *stubReplayJobRepo) MarkFailed(ctx context.Context, workspaceID, jobID, errorMessage string, completedAt time.Time) error {
	return s.markFailedFn(ctx, workspaceID, jobID, errorMessage, completedAt)
}

type stubTxManager struct {
	withinTxFn func(ctx context.Context, fn func(context.Context) error) error
}

func (s *stubTxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return s.withinTxFn(ctx, fn)
}

type stubOutboxWriter struct {
	writeFn func(ctx context.Context, eventType, aggregateID, workspaceID string, payload any) error
}

func (s *stubOutboxWriter) Write(ctx context.Context, eventType, aggregateID, workspaceID string, payload any) error {
	return s.writeFn(ctx, eventType, aggregateID, workspaceID, payload)
}

type stubAccessChecker struct {
	requirePermissionFn func(ctx context.Context, workspaceID, userID, permission string) error
}

func (s *stubAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return s.requirePermissionFn(ctx, workspaceID, userID, permission)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type serviceStubs struct {
	outboxRepo     *stubOutboxRepo
	deadLetterRepo *stubDeadLetterRepo
	replayJobRepo  *stubReplayJobRepo
	txManager      *stubTxManager
	outboxWriter   *stubOutboxWriter
	accessChecker  *stubAccessChecker
}

func newTestService(t *testing.T, stubs serviceStubs) *Service {
	t.Helper()
	if stubs.outboxRepo == nil {
		stubs.outboxRepo = &stubOutboxRepo{}
	}
	if stubs.deadLetterRepo == nil {
		stubs.deadLetterRepo = &stubDeadLetterRepo{}
	}
	if stubs.replayJobRepo == nil {
		stubs.replayJobRepo = &stubReplayJobRepo{}
	}
	if stubs.txManager == nil {
		stubs.txManager = &stubTxManager{}
	}
	if stubs.outboxWriter == nil {
		stubs.outboxWriter = &stubOutboxWriter{}
	}
	if stubs.accessChecker == nil {
		stubs.accessChecker = &stubAccessChecker{}
	}

	return NewService(Options{
		OutboxRepo:     stubs.outboxRepo,
		DeadLetterRepo: stubs.deadLetterRepo,
		ReplayJobRepo:  stubs.replayJobRepo,
		TxManager:      stubs.txManager,
		OutboxWriter:   stubs.outboxWriter,
		AccessChecker:  stubs.accessChecker,
		IDGen:          func() (string, error) { return "test-id-" + fmt.Sprint(time.Now().UnixNano()), nil },
		Clock:          func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
		Logger:         slog.New(slog.DiscardHandler),
	})
}

func okAccess(ctx context.Context, _, _, _ string) error { return nil }

var errPermissionDenied = errors.New("permission denied")

func denyAccess(_ context.Context, _, _, _ string) error { return errPermissionDenied }

// ---------------------------------------------------------------------------
// Tests: GetOutboxSummary
// ---------------------------------------------------------------------------

func TestGetOutboxSummary_Success(t *testing.T) {
	var (
		workspaceID = "ws-1"
		userID      = "user-1"
		filter      = domain.OutboxFilter{Limit: 10}
	)
	outboxRepo := &stubOutboxRepo{
		getSummaryFn: func(_ context.Context, wid string, _ domain.OutboxFilter) (domain.OutboxSummary, error) {
			if wid != workspaceID {
				t.Fatalf("expected workspace %s, got %s", workspaceID, wid)
			}
			return domain.OutboxSummary{
				TotalCount:   5,
				OldestAgeSec: 120,
				ByEventType:  map[string]int{"msg.sent.v1": 3, "msg.delivered.v1": 2},
			}, nil
		},
	}
	svc := newTestService(t, serviceStubs{
		outboxRepo:    outboxRepo,
		accessChecker: &stubAccessChecker{requirePermissionFn: okAccess},
	})

	result, err := svc.GetOutboxSummary(context.Background(), GetOutboxSummaryInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Filter:      filter,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalCount != 5 {
		t.Fatalf("expected TotalCount=5, got %d", result.TotalCount)
	}
	if result.ByEventType["msg.sent.v1"] != 3 {
		t.Fatalf("expected msg.sent.v1 count=3, got %d", result.ByEventType["msg.sent.v1"])
	}
}

func TestGetOutboxSummary_PermissionDenied(t *testing.T) {
	svc := newTestService(t, serviceStubs{
		accessChecker: &stubAccessChecker{requirePermissionFn: denyAccess},
	})
	_, err := svc.GetOutboxSummary(context.Background(), GetOutboxSummaryInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
}

func TestGetOutboxSummary_FilterValidationError(t *testing.T) {
	svc := newTestService(t, serviceStubs{
		accessChecker: &stubAccessChecker{requirePermissionFn: okAccess},
	})
	_, err := svc.GetOutboxSummary(context.Background(), GetOutboxSummaryInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		Filter:      domain.OutboxFilter{Limit: 999},
	})
	if err == nil {
		t.Fatal("expected filter validation error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: ListOutboxRecords
// ---------------------------------------------------------------------------

func TestListOutboxRecords_Success(t *testing.T) {
	workspaceID := "ws-1"
	outboxRepo := &stubOutboxRepo{
		listFn: func(_ context.Context, wid string, _ domain.OutboxFilter) ([]domain.OutboxRecord, string, error) {
			if wid != workspaceID {
				t.Fatalf("expected workspace %s", workspaceID)
			}
			return []domain.OutboxRecord{
				{
					ID: "evt-1", WorkspaceID: workspaceID,
					EventType: "msg.sent.v1", Payload: json.RawMessage(`{"msg":"hello"}`),
					Headers: json.RawMessage(`{"x-correlation":"abc"}`),
				},
				{
					ID: "evt-2", WorkspaceID: workspaceID,
					EventType: "msg.delivered.v1", Payload: json.RawMessage(`{"msg":"world"}`),
					Headers: json.RawMessage(`{"x-correlation":"abc"}`),
				},
			}, "cursor-next", nil
		},
	}
	svc := newTestService(t, serviceStubs{
		outboxRepo:    outboxRepo,
		accessChecker: &stubAccessChecker{requirePermissionFn: okAccess},
	})

	results, cursor, err := svc.ListOutboxRecords(context.Background(), ListOutboxRecordsInput{
		WorkspaceID: workspaceID,
		UserID:      "user-1",
		Filter:      domain.OutboxFilter{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 records, got %d", len(results))
	}
	if cursor != "cursor-next" {
		t.Fatalf("expected cursor cursor-next, got %s", cursor)
	}
}

func TestListOutboxRecords_PermissionDenied(t *testing.T) {
	svc := newTestService(t, serviceStubs{
		accessChecker: &stubAccessChecker{requirePermissionFn: denyAccess},
	})
	_, _, err := svc.ListOutboxRecords(context.Background(), ListOutboxRecordsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestListOutboxRecords_Empty(t *testing.T) {
	workspaceID := "ws-1"
	outboxRepo := &stubOutboxRepo{
		listFn: func(_ context.Context, wid string, _ domain.OutboxFilter) ([]domain.OutboxRecord, string, error) {
			return nil, "", nil
		},
	}
	svc := newTestService(t, serviceStubs{
		outboxRepo:    outboxRepo,
		accessChecker: &stubAccessChecker{requirePermissionFn: okAccess},
	})

	results, cursor, err := svc.ListOutboxRecords(context.Background(), ListOutboxRecordsInput{
		WorkspaceID: workspaceID,
		UserID:      "user-1",
		Filter:      domain.OutboxFilter{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 records, got %d", len(results))
	}
	if cursor != "" {
		t.Fatalf("expected empty cursor, got %s", cursor)
	}
}

func TestListOutboxRecords_PayloadRedaction(t *testing.T) {
	workspaceID := "ws-1"
	rawPayload := json.RawMessage(`{"msg":"hello","token":"secret123"}`)
	outboxRepo := &stubOutboxRepo{
		listFn: func(_ context.Context, wid string, _ domain.OutboxFilter) ([]domain.OutboxRecord, string, error) {
			return []domain.OutboxRecord{
				{ID: "evt-1", WorkspaceID: workspaceID, EventType: "msg.sent.v1", Payload: rawPayload},
			}, "", nil
		},
	}
	svc := newTestService(t, serviceStubs{
		outboxRepo:    outboxRepo,
		accessChecker: &stubAccessChecker{requirePermissionFn: okAccess},
	})

	results, _, err := svc.ListOutboxRecords(context.Background(), ListOutboxRecordsInput{
		WorkspaceID: workspaceID,
		UserID:      "user-1",
		Filter:      domain.OutboxFilter{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 record, got %d", len(results))
	}
	var payloadMap map[string]any
	if err := json.Unmarshal(results[0].Payload, &payloadMap); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if payloadMap["token"] != "[REDACTED]" {
		t.Fatalf("expected token to be redacted, got %v", payloadMap["token"])
	}
	if payloadMap["msg"] != "hello" {
		t.Fatalf("expected msg to be unchanged, got %v", payloadMap["msg"])
	}
}

// ---------------------------------------------------------------------------
// Tests: ListDeadLetterRecords
// ---------------------------------------------------------------------------

func TestListDeadLetterRecords_Success(t *testing.T) {
	workspaceID := "ws-1"
	deadLetterRepo := &stubDeadLetterRepo{
		listFn: func(_ context.Context, wid string, _ domain.DeadLetterFilter) ([]domain.DeadLetterRecord, string, error) {
			return []domain.DeadLetterRecord{
				{ID: "dlq-1", WorkspaceID: workspaceID, ErrorMessage: "timeout", Retryable: true},
				{ID: "dlq-2", WorkspaceID: workspaceID, ErrorMessage: "rejected", Retryable: false},
			}, "dlq-cursor", nil
		},
	}
	svc := newTestService(t, serviceStubs{
		deadLetterRepo: deadLetterRepo,
		accessChecker:  &stubAccessChecker{requirePermissionFn: okAccess},
	})

	results, cursor, err := svc.ListDeadLetterRecords(context.Background(), ListDeadLetterRecordsInput{
		WorkspaceID: workspaceID,
		UserID:      "user-1",
		Filter:      domain.DeadLetterFilter{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 records, got %d", len(results))
	}
	if cursor != "dlq-cursor" {
		t.Fatalf("expected cursor dlq-cursor, got %s", cursor)
	}
	if !results[0].Retryable {
		t.Fatal("expected first record to be retryable")
	}
}

func TestListDeadLetterRecords_PermissionDenied(t *testing.T) {
	svc := newTestService(t, serviceStubs{
		accessChecker: &stubAccessChecker{requirePermissionFn: denyAccess},
	})
	_, _, err := svc.ListDeadLetterRecords(context.Background(), ListDeadLetterRecordsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestListDeadLetterRecords_Empty(t *testing.T) {
	deadLetterRepo := &stubDeadLetterRepo{
		listFn: func(_ context.Context, _ string, _ domain.DeadLetterFilter) ([]domain.DeadLetterRecord, string, error) {
			return nil, "", nil
		},
	}
	svc := newTestService(t, serviceStubs{
		deadLetterRepo: deadLetterRepo,
		accessChecker:  &stubAccessChecker{requirePermissionFn: okAccess},
	})

	results, cursor, err := svc.ListDeadLetterRecords(context.Background(), ListDeadLetterRecordsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		Filter:      domain.DeadLetterFilter{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 records, got %d", len(results))
	}
	if cursor != "" {
		t.Fatalf("expected empty cursor, got %s", cursor)
	}
}

// ---------------------------------------------------------------------------
// Tests: GetDeadLetterRecord
// ---------------------------------------------------------------------------

func TestGetDeadLetterRecord_Found(t *testing.T) {
	workspaceID := "ws-1"
	deadLetterRepo := &stubDeadLetterRepo{
		findByIDFn: func(_ context.Context, wid, id string) (*domain.DeadLetterRecord, error) {
			return &domain.DeadLetterRecord{
				ID: id, WorkspaceID: wid,
				Payload:      json.RawMessage(`{"msg":"hello","password":"s3cret"}`),
				ErrorMessage: "timeout",
			}, nil
		},
	}
	svc := newTestService(t, serviceStubs{
		deadLetterRepo: deadLetterRepo,
		accessChecker:  &stubAccessChecker{requirePermissionFn: okAccess},
	})

	result, err := svc.GetDeadLetterRecord(context.Background(), GetDeadLetterRecordInput{
		WorkspaceID: workspaceID,
		UserID:      "user-1",
		RecordID:    "dlq-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "dlq-1" || result.ErrorMessage != "timeout" {
		t.Fatalf("unexpected result: %+v", result)
	}
	var payloadMap map[string]any
	if err := json.Unmarshal(result.Payload, &payloadMap); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if payloadMap["password"] != "[REDACTED]" {
		t.Fatalf("expected password to be redacted, got %v", payloadMap["password"])
	}
}

func TestGetDeadLetterRecord_NotFound(t *testing.T) {
	deadLetterRepo := &stubDeadLetterRepo{
		findByIDFn: func(_ context.Context, _, _ string) (*domain.DeadLetterRecord, error) {
			return nil, domain.ErrDeadLetterRecordNotFound
		},
	}
	svc := newTestService(t, serviceStubs{
		deadLetterRepo: deadLetterRepo,
		accessChecker:  &stubAccessChecker{requirePermissionFn: okAccess},
	})

	_, err := svc.GetDeadLetterRecord(context.Background(), GetDeadLetterRecordInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		RecordID:    "does-not-exist",
	})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

// ---------------------------------------------------------------------------
// Tests: CreateReplayJob
// ---------------------------------------------------------------------------

func TestCreateReplayJob_Success(t *testing.T) {
	workspaceID := "ws-1"
	var (
		writtenEventType string
		writtenJob       domain.ReplayJob
	)

	deadLetterRepo := &stubDeadLetterRepo{
		findByIDFn: func(_ context.Context, wid, id string) (*domain.DeadLetterRecord, error) {
			return &domain.DeadLetterRecord{ID: id, WorkspaceID: wid}, nil
		},
	}
	replayJobRepo := &stubReplayJobRepo{
		createFn: func(_ context.Context, job domain.ReplayJob) error {
			writtenJob = job
			return nil
		},
	}
	outboxWriter := &stubOutboxWriter{
		writeFn: func(_ context.Context, eventType, _, _ string, _ any) error {
			writtenEventType = eventType
			return nil
		},
	}
	svc := newTestService(t, serviceStubs{
		deadLetterRepo: deadLetterRepo,
		replayJobRepo:  replayJobRepo,
		outboxWriter:   outboxWriter,
		accessChecker:  &stubAccessChecker{requirePermissionFn: okAccess},
	})

	result, err := svc.CreateReplayJob(context.Background(), CreateReplayJobInput{
		WorkspaceID: workspaceID,
		UserID:      "user-1",
		TargetType:  domain.ReplayTargetDeadLetter,
		TargetID:    "dlq-1",
		Source:      "manual",
		Reason:      "testing replay",
		Filter:      json.RawMessage(`{"event_type":"msg.sent.v1"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.TargetType != domain.ReplayTargetDeadLetter {
		t.Fatalf("expected target type dead_letter_record, got %s", result.TargetType)
	}
	if result.Status != domain.ReplayJobQueued {
		t.Fatalf("expected status queued, got %s", result.Status)
	}
	if writtenJob.ID != result.ID {
		t.Fatal("written job ID does not match result ID")
	}
	if writtenEventType != "operations.replay_job.created.v1" {
		t.Fatalf("expected event type operations.replay_job.created.v1, got %s", writtenEventType)
	}
}

func TestCreateReplayJob_PermissionDenied(t *testing.T) {
	svc := newTestService(t, serviceStubs{
		accessChecker: &stubAccessChecker{requirePermissionFn: denyAccess},
	})
	_, err := svc.CreateReplayJob(context.Background(), CreateReplayJobInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestCreateReplayJob_InvalidTargetType(t *testing.T) {
	svc := newTestService(t, serviceStubs{
		accessChecker: &stubAccessChecker{requirePermissionFn: okAccess},
	})
	_, err := svc.CreateReplayJob(context.Background(), CreateReplayJobInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		TargetType:  "invalid_target",
		TargetID:    "some-id",
		Source:      "manual",
		Reason:      "testing",
	})
	if err == nil {
		t.Fatal("expected invalid target type error")
	}
}
