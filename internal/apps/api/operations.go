package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	operationsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
)

type operationsHTTP struct {
	svc *operationsapp.Service
}

func newOperationsHTTP(svc *operationsapp.Service) *operationsHTTP {
	return &operationsHTTP{svc: svc}
}

// --- Outbox ---

type outboxSummaryResponseDoc struct {
	TotalCount   int            `json:"total_count"`
	OldestAgeSec int64          `json:"oldest_age_seconds"`
	OldestAt     *time.Time     `json:"oldest_at,omitempty"`
	ByEventType  map[string]int `json:"by_event_type,omitempty"`
}

type outboxRecordListItem struct {
	ID            string    `json:"id"`
	EventType     string    `json:"event_type"`
	AggregateType string    `json:"aggregate_type"`
	AggregateID   string    `json:"aggregate_id"`
	Payload       any       `json:"payload,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type outboxRecordListResponse struct {
	Data []outboxRecordListItem `json:"data"`
	Meta requestMetaDoc         `json:"meta"`
	Next string                 `json:"next,omitempty"`
}

type outboxRecordDetailDoc struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	EventType     string    `json:"event_type"`
	AggregateType string    `json:"aggregate_type"`
	AggregateID   string    `json:"aggregate_id"`
	Payload       any       `json:"payload,omitempty"`
	Headers       any       `json:"headers,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type outboxRecordDetailResponse struct {
	Data outboxRecordDetailDoc `json:"data"`
	Meta requestMetaDoc        `json:"meta"`
}

func (h *operationsHTTP) getOutboxSummary(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	filter := buildOutboxFilter(q)

	summary, err := h.svc.GetOutboxSummary(r.Context(), operationsapp.GetOutboxSummaryInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Filter:      filter,
	})
	if err != nil {
		mapOperationsErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, outboxSummaryResponseDoc{
		TotalCount:   summary.TotalCount,
		OldestAgeSec: summary.OldestAgeSec,
		OldestAt:     summary.OldestAt,
		ByEventType:  summary.ByEventType,
	})
}

func (h *operationsHTTP) listOutboxRecords(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	filter := buildOutboxFilter(q)

	records, nextCursor, err := h.svc.ListOutboxRecords(r.Context(), operationsapp.ListOutboxRecordsInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Filter:      filter,
	})
	if err != nil {
		mapOperationsErr(w, r, err)
		return
	}

	items := make([]outboxRecordListItem, 0, len(records))
	for _, rec := range records {
		var payload any
		if len(rec.Payload) > 0 {
			_ = json.Unmarshal(rec.Payload, &payload)
		}
		items = append(items, outboxRecordListItem{
			ID:            rec.ID,
			EventType:     rec.EventType,
			AggregateType: rec.AggregateType,
			AggregateID:   rec.AggregateID,
			Payload:       payload,
			CreatedAt:     rec.CreatedAt,
			OccurredAt:    rec.OccurredAt,
		})
	}

	writeEnvelope(w, r, http.StatusOK, outboxRecordListResponse{
		Data: items,
		Meta: requestMetaDoc{RequestID: requestID(r)},
		Next: nextCursor,
	})
}

func (h *operationsHTTP) getOutboxRecord(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	outboxID := chi.URLParam(r, "outbox_id")
	userID := r.Context().Value(ctxUserID).(string)

	rec, err := h.svc.GetOutboxRecord(r.Context(), operationsapp.GetOutboxRecordInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		OutboxID:    outboxID,
	})
	if err != nil {
		mapOperationsErr(w, r, err)
		return
	}

	var payload, headers any
	if len(rec.Payload) > 0 {
		_ = json.Unmarshal(rec.Payload, &payload)
	}
	if len(rec.Headers) > 0 {
		_ = json.Unmarshal(rec.Headers, &headers)
	}

	writeEnvelope(w, r, http.StatusOK, outboxRecordDetailResponse{
		Data: outboxRecordDetailDoc{
			ID:            rec.ID,
			WorkspaceID:   rec.WorkspaceID,
			EventType:     rec.EventType,
			AggregateType: rec.AggregateType,
			AggregateID:   rec.AggregateID,
			Payload:       payload,
			Headers:       headers,
			CreatedAt:     rec.CreatedAt,
			OccurredAt:    rec.OccurredAt,
		},
		Meta: requestMetaDoc{RequestID: requestID(r)},
	})
}

// --- Dead Letter ---

type deadLetterRecordListItem struct {
	ID           string    `json:"id"`
	Source       string    `json:"source"`
	EventID      string    `json:"event_id"`
	ErrorMessage string    `json:"error_message"`
	Retryable    bool      `json:"retryable"`
	Payload      any       `json:"payload,omitempty"`
	FailedAt     time.Time `json:"failed_at"`
}

type deadLetterRecordListResponse struct {
	Data []deadLetterRecordListItem `json:"data"`
	Meta requestMetaDoc             `json:"meta"`
	Next string                     `json:"next,omitempty"`
}

type deadLetterRecordDetailDoc struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Source       string    `json:"source"`
	EventID      string    `json:"event_id"`
	ErrorMessage string    `json:"error_message"`
	Retryable    bool      `json:"retryable"`
	Payload      any       `json:"payload,omitempty"`
	FailedAt     time.Time `json:"failed_at"`
}

type deadLetterRecordDetailResponse struct {
	Data deadLetterRecordDetailDoc `json:"data"`
	Meta requestMetaDoc            `json:"meta"`
}

func (h *operationsHTTP) listDeadLetterRecords(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	filter := buildDeadLetterFilter(q)

	records, nextCursor, err := h.svc.ListDeadLetterRecords(r.Context(), operationsapp.ListDeadLetterRecordsInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Filter:      filter,
	})
	if err != nil {
		mapOperationsErr(w, r, err)
		return
	}

	items := make([]deadLetterRecordListItem, 0, len(records))
	for _, rec := range records {
		var payload any
		if len(rec.Payload) > 0 {
			_ = json.Unmarshal(rec.Payload, &payload)
		}
		items = append(items, deadLetterRecordListItem{
			ID:           rec.ID,
			Source:       rec.Source,
			EventID:      rec.EventID,
			ErrorMessage: rec.ErrorMessage,
			Retryable:    rec.Retryable,
			Payload:      payload,
			FailedAt:     rec.FailedAt,
		})
	}

	writeEnvelope(w, r, http.StatusOK, deadLetterRecordListResponse{
		Data: items,
		Meta: requestMetaDoc{RequestID: requestID(r)},
		Next: nextCursor,
	})
}

func (h *operationsHTTP) getDeadLetterRecord(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	recordID := chi.URLParam(r, "dead_letter_id")
	userID := r.Context().Value(ctxUserID).(string)

	rec, err := h.svc.GetDeadLetterRecord(r.Context(), operationsapp.GetDeadLetterRecordInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		RecordID:    recordID,
	})
	if err != nil {
		mapOperationsErr(w, r, err)
		return
	}

	var payload any
	if len(rec.Payload) > 0 {
		_ = json.Unmarshal(rec.Payload, &payload)
	}

	writeEnvelope(w, r, http.StatusOK, deadLetterRecordDetailResponse{
		Data: deadLetterRecordDetailDoc{
			ID:           rec.ID,
			WorkspaceID:  rec.WorkspaceID,
			Source:       rec.Source,
			EventID:      rec.EventID,
			ErrorMessage: rec.ErrorMessage,
			Retryable:    rec.Retryable,
			Payload:      payload,
			FailedAt:     rec.FailedAt,
		},
		Meta: requestMetaDoc{RequestID: requestID(r)},
	})
}

// --- Replay Jobs ---

type createReplayJobRequest struct {
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	Source     string          `json:"source,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	Filter     json.RawMessage `json:"filter,omitempty"`
}

type replayJobDetailDoc struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspace_id"`
	TargetType        string     `json:"target_type"`
	TargetID          string     `json:"target_id"`
	Source            string     `json:"source,omitempty"`
	Status            string     `json:"status"`
	RequestedByUserID string     `json:"requested_by_user_id,omitempty"`
	Reason            string     `json:"reason,omitempty"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type replayJobDetailResponse struct {
	Data replayJobDetailDoc `json:"data"`
	Meta requestMetaDoc     `json:"meta"`
}

type replayJobListResponse struct {
	Data []replayJobDetailDoc `json:"data"`
	Meta requestMetaDoc       `json:"meta"`
	Next string               `json:"next,omitempty"`
}

func (h *operationsHTTP) createReplayJob(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID := r.Context().Value(ctxUserID).(string)

	var req createReplayJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "auth.invalid_request_body", "invalid request body", nil)
		return
	}

	job, err := h.svc.CreateReplayJob(r.Context(), operationsapp.CreateReplayJobInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		TargetType:  domain.ReplayTargetType(req.TargetType),
		TargetID:    req.TargetID,
		Source:      req.Source,
		Reason:      req.Reason,
		Filter:      req.Filter,
	})
	if err != nil {
		mapOperationsErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, replayJobDetailDoc{
		ID:                job.ID,
		WorkspaceID:       job.WorkspaceID,
		TargetType:        string(job.TargetType),
		TargetID:          job.TargetID,
		Source:            job.Source,
		Status:            string(job.Status),
		RequestedByUserID: job.RequestedByUserID,
		Reason:            job.Reason,
		CreatedAt:         job.CreatedAt,
		UpdatedAt:         job.UpdatedAt,
	})
}

func (h *operationsHTTP) listReplayJobs(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	var limit int
	parseLimit(q.Get("limit"), &limit)

	filter := domain.ReplayJobFilter{
		Status: q.Get("status"),
		Limit:  limit,
		Cursor: q.Get("cursor"),
	}

	jobs, nextCursor, err := h.svc.ListReplayJobs(r.Context(), operationsapp.ListReplayJobsInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Filter:      filter,
	})
	if err != nil {
		mapOperationsErr(w, r, err)
		return
	}

	items := make([]replayJobDetailDoc, 0, len(jobs))
	for _, j := range jobs {
		items = append(items, replayJobDetailDoc{
			ID:                j.ID,
			WorkspaceID:       j.WorkspaceID,
			TargetType:        string(j.TargetType),
			TargetID:          j.TargetID,
			Source:            j.Source,
			Status:            string(j.Status),
			RequestedByUserID: j.RequestedByUserID,
			Reason:            j.Reason,
			ErrorMessage:      j.ErrorMessage,
			CreatedAt:         j.CreatedAt,
			StartedAt:         j.StartedAt,
			CompletedAt:       j.CompletedAt,
			UpdatedAt:         j.UpdatedAt,
		})
	}

	writeEnvelope(w, r, http.StatusOK, replayJobListResponse{
		Data: items,
		Meta: requestMetaDoc{RequestID: requestID(r)},
		Next: nextCursor,
	})
}

func (h *operationsHTTP) getReplayJob(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	jobID := chi.URLParam(r, "replay_job_id")
	userID := r.Context().Value(ctxUserID).(string)

	job, err := h.svc.GetReplayJob(r.Context(), operationsapp.GetReplayJobInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		JobID:       jobID,
	})
	if err != nil {
		mapOperationsErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, replayJobDetailResponse{
		Data: replayJobDetailDoc{
			ID:                job.ID,
			WorkspaceID:       job.WorkspaceID,
			TargetType:        string(job.TargetType),
			TargetID:          job.TargetID,
			Source:            job.Source,
			Status:            string(job.Status),
			RequestedByUserID: job.RequestedByUserID,
			Reason:            job.Reason,
			ErrorMessage:      job.ErrorMessage,
			CreatedAt:         job.CreatedAt,
			StartedAt:         job.StartedAt,
			CompletedAt:       job.CompletedAt,
			UpdatedAt:         job.UpdatedAt,
		},
		Meta: requestMetaDoc{RequestID: requestID(r)},
	})
}

// --- Error Mapping ---

func mapOperationsErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrQueueReadDenied):
		writeError(w, r, http.StatusForbidden, "operations.queue_read_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrDLQReadDenied):
		writeError(w, r, http.StatusForbidden, "operations.dlq_read_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrReplayManageDenied):
		writeError(w, r, http.StatusForbidden, "operations.replay_manage_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrFilterInvalid):
		writeError(w, r, http.StatusBadRequest, "operations.filter_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrReplayTargetInvalid):
		writeError(w, r, http.StatusBadRequest, "operations.replay_target_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrOutboxRecordNotFound):
		writeError(w, r, http.StatusNotFound, "operations.outbox_record_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrDeadLetterRecordNotFound):
		writeError(w, r, http.StatusNotFound, "operations.dead_letter_record_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrReplayJobNotFound):
		writeError(w, r, http.StatusNotFound, "operations.replay_job_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrReplayConflict):
		writeError(w, r, http.StatusConflict, "operations.replay_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrWorkspaceRequired):
		writeError(w, r, http.StatusBadRequest, "operations.filter_invalid", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", err.Error(), nil)
	}
}

// --- Helpers ---

func buildOutboxFilter(q params) domain.OutboxFilter {
	var from, to *time.Time
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			from = &t
		}
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			to = &t
		}
	}
	var limit int
	parseLimit(q.Get("limit"), &limit)
	return domain.OutboxFilter{
		EventType:     q.Get("event_type"),
		AggregateType: q.Get("aggregate_type"),
		AggregateID:   q.Get("aggregate_id"),
		From:          from,
		To:            to,
		Limit:         limit,
		Cursor:        q.Get("cursor"),
	}
}

func buildDeadLetterFilter(q params) domain.DeadLetterFilter {
	var from, to *time.Time
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			from = &t
		}
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			to = &t
		}
	}
	var limit int
	parseLimit(q.Get("limit"), &limit)
	filter := domain.DeadLetterFilter{
		Source: q.Get("source"),
		From:   from,
		To:     to,
		Limit:  limit,
		Cursor: q.Get("cursor"),
	}
	if v := q.Get("retryable"); v != "" {
		b := v == "true"
		filter.Retryable = &b
	}
	return filter
}

type params interface {
	Get(string) string
}
