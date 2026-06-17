package postgres

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type MessageEventRepository struct {
	db platformpostgres.DBTX
}

func NewMessageEventRepository(db platformpostgres.DBTX) *MessageEventRepository {
	return &MessageEventRepository{db: db}
}

func (r *MessageEventRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *MessageEventRepository) Create(ctx context.Context, event domain.MessageEvent) error {
	db := r.getDB(ctx)
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}

	var txReqID *string
	if event.TransactionalRequestID != "" {
		txReqID = &event.TransactionalRequestID
	}

	_, err = db.Exec(ctx,
		`INSERT INTO message_events (id, workspace_id, message_id, transactional_request_id,
		                             event_type, status, reason_code, reason_message,
		                             metadata, occurred_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		event.ID, event.WorkspaceID, event.MessageID,
		txReqID,
		event.EventType, event.Status,
		platformpostgres.Nullable(event.ReasonCode),
		platformpostgres.Nullable(event.ReasonMessage),
		metadataJSON, event.OccurredAt, event.CreatedAt,
	)
	return err
}

func (r *MessageEventRepository) CreateMany(ctx context.Context, events []domain.MessageEvent) error {
	db := r.getDB(ctx)
	if len(events) == 0 {
		return nil
	}

	values := make([]string, 0, len(events))
	args := make([]any, 0, len(events)*11)
	idx := 1

	for _, event := range events {
		metadataJSON, err := json.Marshal(event.Metadata)
		if err != nil {
			return err
		}

		var txReqID *string
		if event.TransactionalRequestID != "" {
			txReqID = &event.TransactionalRequestID
		}

		values = append(values, "($"+
			platformpostgres.Itoa(idx)+",$"+platformpostgres.Itoa(idx+1)+
			",$"+platformpostgres.Itoa(idx+2)+",$"+platformpostgres.Itoa(idx+3)+
			",$"+platformpostgres.Itoa(idx+4)+",$"+platformpostgres.Itoa(idx+5)+
			",$"+platformpostgres.Itoa(idx+6)+",$"+platformpostgres.Itoa(idx+7)+
			",$"+platformpostgres.Itoa(idx+8)+",$"+platformpostgres.Itoa(idx+9)+
			",$"+platformpostgres.Itoa(idx+10)+")")

		args = append(args,
			event.ID, event.WorkspaceID, event.MessageID,
			txReqID,
			event.EventType, event.Status,
			platformpostgres.Nullable(event.ReasonCode),
			platformpostgres.Nullable(event.ReasonMessage),
			metadataJSON, event.OccurredAt, event.CreatedAt,
		)
		idx += 11
	}

	_, err := db.Exec(ctx,
		`INSERT INTO message_events (id, workspace_id, message_id, transactional_request_id,
		                             event_type, status, reason_code, reason_message,
		                             metadata, occurred_at, created_at)
		 VALUES `+strings.Join(values, ","), args...)
	return err
}

func scanMessageEventRow(row scannable) (*domain.MessageEvent, error) {
	var event domain.MessageEvent
	var metadataJSON []byte
	var txReqID *string

	err := row.Scan(
		&event.ID, &event.WorkspaceID, &event.MessageID, &txReqID,
		&event.EventType, &event.Status,
		&event.ReasonCode, &event.ReasonMessage,
		&metadataJSON, &event.OccurredAt, &event.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if txReqID != nil {
		event.TransactionalRequestID = *txReqID
	}
	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, err
		}
	}

	return &event, nil
}

func scanMessageEventRows(rows pgx.Rows) ([]domain.MessageEvent, error) {
	var results []domain.MessageEvent
	for rows.Next() {
		event, err := scanMessageEventRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.MessageEvent{}
	}
	return results, nil
}

func (r *MessageEventRepository) ListByMessage(ctx context.Context, workspaceID, messageID string, limit int, cursor string) ([]domain.MessageEvent, string, error) {
	db := r.getDB(ctx)
	args := []any{workspaceID, messageID}
	where := "WHERE workspace_id = $1 AND message_id = $2"
	argIdx := 3

	if cursor != "" {
		where += " AND (occurred_at, id) < (SELECT occurred_at, id FROM message_events WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, cursor)
		argIdx++
	}

	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	if limit > 100 {
		limit = 100
	}
	where += " ORDER BY occurred_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := db.Query(ctx,
		`SELECT id, workspace_id, message_id, transactional_request_id,
		        event_type, status, reason_code, reason_message,
		        metadata, occurred_at, created_at
		 FROM message_events `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	results, err := scanMessageEventRows(rows)
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > limit {
		nextCursor = results[limit-1].ID
		results = results[:limit]
	}
	if results == nil {
		results = []domain.MessageEvent{}
	}

	return results, nextCursor, nil
}

func (r *MessageEventRepository) ListByRequest(ctx context.Context, workspaceID, requestID string, limit int, cursor string) ([]domain.MessageEvent, string, error) {
	db := r.getDB(ctx)
	args := []any{workspaceID, requestID}
	where := "WHERE workspace_id = $1 AND transactional_request_id = $2"
	argIdx := 3

	if cursor != "" {
		where += " AND (occurred_at, id) < (SELECT occurred_at, id FROM message_events WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, cursor)
		argIdx++
	}

	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	if limit > 100 {
		limit = 100
	}
	where += " ORDER BY occurred_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := db.Query(ctx,
		`SELECT id, workspace_id, message_id, transactional_request_id,
		        event_type, status, reason_code, reason_message,
		        metadata, occurred_at, created_at
		 FROM message_events `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	results, err := scanMessageEventRows(rows)
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > limit {
		nextCursor = results[limit-1].ID
		results = results[:limit]
	}
	if results == nil {
		results = []domain.MessageEvent{}
	}

	return results, nextCursor, nil
}
