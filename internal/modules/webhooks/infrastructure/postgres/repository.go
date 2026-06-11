package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type ConfigWriteRepository struct {
	pool *pgxpool.Pool
}

func NewConfigWriteRepository(pool *pgxpool.Pool) *ConfigWriteRepository {
	return &ConfigWriteRepository{pool: pool}
}

func (r *ConfigWriteRepository) db(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.pool
}

func (r *ConfigWriteRepository) Create(ctx context.Context, config domain.WebhookConfig) error {
	subs, err := json.Marshal(config.Subscriptions)
	if err != nil {
		return err
	}
	_, err = r.db(ctx).Exec(ctx,
		`INSERT INTO customer_webhooks (id, workspace_id, name, target_url, status, subscriptions_json, secret_hash, secret_hint, version, created_by_user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11, $12)`,
		config.ID, config.WorkspaceID, config.Name, config.TargetURL, string(config.Status),
		subs, config.SecretHash, config.SecretHint, config.Version, config.CreatedByUserID,
		config.CreatedAt, config.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrConfigNameConflict
		}
		return err
	}
	return nil
}

func (r *ConfigWriteRepository) Update(ctx context.Context, config domain.WebhookConfig) error {
	subs, err := json.Marshal(config.Subscriptions)
	if err != nil {
		return err
	}
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE customer_webhooks SET name=$1, target_url=$2, status=$3, subscriptions_json=$4::jsonb, secret_hash=$5, secret_hint=$6, version=$7, updated_at=$8, disabled_at=$9
		WHERE id=$10 AND workspace_id=$11 AND version=$12`,
		config.Name, config.TargetURL, string(config.Status), subs,
		config.SecretHash, config.SecretHint, config.Version, config.UpdatedAt, config.DisabledAt,
		config.ID, config.WorkspaceID, config.Version-1,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrConfigNameConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRotateConflict
	}
	return nil
}

func (r *ConfigWriteRepository) Disable(ctx context.Context, workspaceID, webhookID string, disabledAt time.Time) error {
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE customer_webhooks SET status='disabled', disabled_at=$1, updated_at=$1, version=version+1 WHERE id=$2 AND workspace_id=$3 AND status='active'`,
		disabledAt, webhookID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConfigNotFound
	}
	return nil
}

type ConfigReadRepository struct {
	db platformpostgres.DBTX
}

func NewConfigReadRepository(pool *pgxpool.Pool) *ConfigReadRepository {
	return &ConfigReadRepository{db: pool}
}

func NewConfigReadRepositoryWithDBTX(db platformpostgres.DBTX) *ConfigReadRepository {
	return &ConfigReadRepository{db: db}
}

func (r *ConfigReadRepository) FindByID(ctx context.Context, workspaceID, webhookID string) (*domain.WebhookConfig, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, name, target_url, status, subscriptions_json, secret_hash, secret_hint, version, created_by_user_id, created_at, updated_at, disabled_at
		FROM customer_webhooks WHERE id=$1 AND workspace_id=$2`,
		webhookID, workspaceID,
	)
	return scanConfig(row)
}

func (r *ConfigReadRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.WebhookConfig, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, name, target_url, status, subscriptions_json, secret_hash, secret_hint, version, created_by_user_id, created_at, updated_at, disabled_at
		FROM customer_webhooks WHERE workspace_id=$1 ORDER BY created_at DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConfigs(rows)
}

func (r *ConfigReadRepository) ListSubscribed(ctx context.Context, workspaceID, eventType string) ([]domain.WebhookConfig, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, name, target_url, status, subscriptions_json, secret_hash, secret_hint, version, created_by_user_id, created_at, updated_at, disabled_at
		FROM customer_webhooks WHERE workspace_id=$1 AND status='active' AND subscriptions_json @> $2::jsonb ORDER BY created_at ASC`,
		workspaceID, `"`+eventType+`"`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConfigs(rows)
}

func scanConfig(row pgx.Row) (*domain.WebhookConfig, error) {
	var cfg domain.WebhookConfig
	var subsJSON []byte
	var disabledAt *time.Time
	err := row.Scan(
		&cfg.ID, &cfg.WorkspaceID, &cfg.Name, &cfg.TargetURL, (*string)(&cfg.Status),
		&subsJSON, &cfg.SecretHash, &cfg.SecretHint, &cfg.Version, &cfg.CreatedByUserID,
		&cfg.CreatedAt, &cfg.UpdatedAt, &disabledAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrConfigNotFound
		}
		return nil, err
	}
	if len(subsJSON) > 0 {
		if err := json.Unmarshal(subsJSON, &cfg.Subscriptions); err != nil {
			return nil, err
		}
	}
	cfg.DisabledAt = disabledAt
	return &cfg, nil
}

func scanConfigs(rows pgx.Rows) ([]domain.WebhookConfig, error) {
	var configs []domain.WebhookConfig
	for rows.Next() {
		cfg, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *cfg)
	}
	if configs == nil {
		configs = []domain.WebhookConfig{}
	}
	return configs, rows.Err()
}

type DeliveryWriteRepository struct {
	pool *pgxpool.Pool
}

func NewDeliveryWriteRepository(pool *pgxpool.Pool) *DeliveryWriteRepository {
	return &DeliveryWriteRepository{pool: pool}
}

func (r *DeliveryWriteRepository) db(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.pool
}

func (r *DeliveryWriteRepository) Create(ctx context.Context, delivery domain.WebhookDelivery) error {
	reqH, _ := json.Marshal(delivery.RequestHeaders)
	respH, _ := json.Marshal(delivery.ResponseHeaders)
	payloadJSON, _ := json.Marshal(delivery.EventPayloadJSON)
	_, err := r.db(ctx).Exec(ctx,
		`INSERT INTO customer_webhook_deliveries (id, workspace_id, webhook_id, source_event_id, source_event_type, status, target_url, attempt_count, next_attempt_at, last_attempt_at, last_status_code, last_error, request_headers_json, response_headers_json, event_payload_json, created_at, updated_at)
		VALUES ($1, $2, $3, $4::uuid, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14::jsonb, $15::jsonb, $16, $17)
		ON CONFLICT (webhook_id, source_event_id) DO NOTHING`,
		delivery.ID, delivery.WorkspaceID, delivery.WebhookID, delivery.SourceEventID,
		delivery.SourceEventType, string(delivery.Status), delivery.TargetURL, delivery.AttemptCount,
		delivery.NextAttemptAt, delivery.LastAttemptAt, delivery.LastStatusCode,
		delivery.LastError, reqH, respH, payloadJSON, delivery.CreatedAt, delivery.UpdatedAt,
	)
	return err
}

func (r *DeliveryWriteRepository) MarkDelivering(ctx context.Context, deliveryID string, now time.Time) error {
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE customer_webhook_deliveries SET status='delivering', attempt_count=attempt_count+1, last_attempt_at=$1, updated_at=$1 WHERE id=$2`,
		now, deliveryID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeliveryNotFound
	}
	return nil
}

func (r *DeliveryWriteRepository) MarkSucceeded(ctx context.Context, deliveryID string, result domain.DeliveryResult) error {
	respH, _ := json.Marshal(domain.SanitizeHeaders(result.ResponseHeaders))
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE customer_webhook_deliveries SET status='succeeded', last_status_code=$1, last_error='', request_headers_json=$2::jsonb, response_headers_json=$3::jsonb, updated_at=NOW() WHERE id=$4`,
		result.StatusCode, "{}", respH, deliveryID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeliveryNotFound
	}
	return nil
}

func (r *DeliveryWriteRepository) MarkFailed(ctx context.Context, deliveryID string, result domain.DeliveryResult) error {
	respH, _ := json.Marshal(domain.SanitizeHeaders(result.ResponseHeaders))
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE customer_webhook_deliveries SET status='failed', last_status_code=$1, last_error=$2, request_headers_json=$3::jsonb, response_headers_json=$4::jsonb, updated_at=NOW() WHERE id=$5`,
		result.StatusCode, domain.SanitizeError(result.Error), "{}", respH, deliveryID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDeliveryNotFound
	}
	return nil
}

func (r *DeliveryWriteRepository) ClaimPendingDeliveries(ctx context.Context, limit int, now time.Time) ([]domain.WebhookDelivery, error) {
	rows, err := r.db(ctx).Query(ctx, `
		UPDATE customer_webhook_deliveries SET status='delivering', attempt_count=attempt_count+1, last_attempt_at=$1, updated_at=$1
		WHERE id IN (
			SELECT id FROM customer_webhook_deliveries
			WHERE status IN ('pending', 'retry_scheduled') AND (next_attempt_at IS NULL OR next_attempt_at <= $1)
			ORDER BY created_at ASC LIMIT $2 FOR UPDATE SKIP LOCKED
		)
		RETURNING id, workspace_id, webhook_id, source_event_id, source_event_type, status, target_url, attempt_count, next_attempt_at, last_attempt_at, last_status_code, COALESCE(last_error, ''), request_headers_json, response_headers_json, event_payload_json, created_at, updated_at`,
		now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeliveries(rows)
}

func (r *DeliveryWriteRepository) ScheduleRetry(ctx context.Context, deliveryID string, nextAttemptAt time.Time) error {
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE customer_webhook_deliveries SET status='retry_scheduled', next_attempt_at=$1, updated_at=NOW() WHERE id=$2 AND status='failed'`,
		nextAttemptAt, deliveryID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRetryConflict
	}
	return nil
}

type DeliveryReadRepository struct {
	db platformpostgres.DBTX
}

func NewDeliveryReadRepository(pool *pgxpool.Pool) *DeliveryReadRepository {
	return &DeliveryReadRepository{db: pool}
}

func NewDeliveryReadRepositoryWithDBTX(db platformpostgres.DBTX) *DeliveryReadRepository {
	return &DeliveryReadRepository{db: db}
}

func (r *DeliveryReadRepository) FindByID(ctx context.Context, workspaceID, deliveryID string) (*domain.WebhookDelivery, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, webhook_id, source_event_id, source_event_type, status, target_url, attempt_count, next_attempt_at, last_attempt_at, last_status_code, last_error, request_headers_json, response_headers_json, event_payload_json, created_at, updated_at
		FROM customer_webhook_deliveries WHERE id=$1 AND workspace_id=$2`,
		deliveryID, workspaceID,
	)
	return scanDelivery(row)
}

func (r *DeliveryReadRepository) FindByWebhookAndEvent(ctx context.Context, webhookID, sourceEventID string) (*domain.WebhookDelivery, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, webhook_id, source_event_id, source_event_type, status, target_url, attempt_count, next_attempt_at, last_attempt_at, last_status_code, last_error, request_headers_json, response_headers_json, event_payload_json, created_at, updated_at
		FROM customer_webhook_deliveries WHERE webhook_id=$1 AND source_event_id=$2::uuid`,
		webhookID, sourceEventID,
	)
	return scanDelivery(row)
}

func (r *DeliveryReadRepository) ListByWorkspace(ctx context.Context, workspaceID string, filter ports.DeliveryFilter) ([]domain.WebhookDelivery, string, error) {
	args := []any{workspaceID}
	query := `SELECT id, workspace_id, webhook_id, source_event_id, source_event_type, status, target_url, attempt_count, next_attempt_at, last_attempt_at, last_status_code, last_error, request_headers_json, response_headers_json, event_payload_json, created_at, updated_at
		FROM customer_webhook_deliveries WHERE workspace_id=$1`
	argIdx := 2

	if filter.WebhookID != "" {
		query += ` AND webhook_id=$` + platformpostgres.Itoa(argIdx)
		args = append(args, filter.WebhookID)
		argIdx++
	}
	if filter.Status != "" {
		query += ` AND status=$` + platformpostgres.Itoa(argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.EventType != "" {
		query += ` AND source_event_type=$` + platformpostgres.Itoa(argIdx)
		args = append(args, filter.EventType)
		argIdx++
	}
	if filter.From != nil {
		query += ` AND created_at>=$` + platformpostgres.Itoa(argIdx)
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		query += ` AND created_at<=$` + platformpostgres.Itoa(argIdx)
		args = append(args, *filter.To)
		argIdx++
	}

	limit := 50
	if filter.Limit > 0 && filter.Limit <= 100 {
		limit = filter.Limit
	}
	if filter.Cursor != "" {
		query += ` AND (created_at, id) < (SELECT created_at, id FROM customer_webhook_deliveries WHERE id=$` + platformpostgres.Itoa(argIdx) + `)`
		args = append(args, filter.Cursor)
		argIdx++
	}

	query += ` ORDER BY created_at DESC LIMIT $` + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	deliveries, err := scanDeliveries(rows)
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(deliveries) > limit {
		nextCursor = deliveries[limit].ID
		deliveries = deliveries[:limit]
	}

	return deliveries, nextCursor, nil
}

func scanDelivery(row pgx.Row) (*domain.WebhookDelivery, error) {
	var d domain.WebhookDelivery
	var reqH, respH, payloadJSON []byte
	var nextAttemptAt, lastAttemptAt *time.Time
	var lastStatusCode *int
	err := row.Scan(
		&d.ID, &d.WorkspaceID, &d.WebhookID, &d.SourceEventID, &d.SourceEventType,
		(*string)(&d.Status), &d.TargetURL, &d.AttemptCount,
		&nextAttemptAt, &lastAttemptAt, &lastStatusCode, &d.LastError,
		&reqH, &respH, &payloadJSON, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDeliveryNotFound
		}
		return nil, err
	}
	d.NextAttemptAt = nextAttemptAt
	d.LastAttemptAt = lastAttemptAt
	d.LastStatusCode = lastStatusCode
	if len(reqH) > 0 {
		json.Unmarshal(reqH, &d.RequestHeaders)
	}
	if len(respH) > 0 {
		json.Unmarshal(respH, &d.ResponseHeaders)
	}
	if len(payloadJSON) > 0 {
		json.Unmarshal(payloadJSON, &d.EventPayloadJSON)
	}
	if d.RequestHeaders == nil {
		d.RequestHeaders = map[string]string{}
	}
	if d.ResponseHeaders == nil {
		d.ResponseHeaders = map[string]string{}
	}
	return &d, nil
}

func scanDeliveries(rows pgx.Rows) ([]domain.WebhookDelivery, error) {
	var deliveries []domain.WebhookDelivery
	for rows.Next() {
		var d domain.WebhookDelivery
		var reqH, respH, payloadJSON []byte
		var nextAttemptAt, lastAttemptAt *time.Time
		var lastStatusCode *int
		err := rows.Scan(
			&d.ID, &d.WorkspaceID, &d.WebhookID, &d.SourceEventID, &d.SourceEventType,
			(*string)(&d.Status), &d.TargetURL, &d.AttemptCount,
			&nextAttemptAt, &lastAttemptAt, &lastStatusCode, &d.LastError,
			&reqH, &respH, &payloadJSON, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		d.NextAttemptAt = nextAttemptAt
		d.LastAttemptAt = lastAttemptAt
		d.LastStatusCode = lastStatusCode
		if len(reqH) > 0 {
			json.Unmarshal(reqH, &d.RequestHeaders)
		}
		if len(respH) > 0 {
			json.Unmarshal(respH, &d.ResponseHeaders)
		}
		if len(payloadJSON) > 0 {
			json.Unmarshal(payloadJSON, &d.EventPayloadJSON)
		}
		if d.RequestHeaders == nil {
			d.RequestHeaders = map[string]string{}
		}
		if d.ResponseHeaders == nil {
			d.ResponseHeaders = map[string]string{}
		}
		deliveries = append(deliveries, d)
	}
	if deliveries == nil {
		deliveries = []domain.WebhookDelivery{}
	}
	return deliveries, rows.Err()
}

type AttemptWriteRepository struct {
	pool *pgxpool.Pool
}

func NewAttemptWriteRepository(pool *pgxpool.Pool) *AttemptWriteRepository {
	return &AttemptWriteRepository{pool: pool}
}

func (r *AttemptWriteRepository) db(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.pool
}

func (r *AttemptWriteRepository) Create(ctx context.Context, attempt domain.WebhookDeliveryAttempt) error {
	reqH, _ := json.Marshal(attempt.RequestHeaders)
	respH, _ := json.Marshal(attempt.ResponseHeaders)
	_, err := r.db(ctx).Exec(ctx,
		`INSERT INTO customer_webhook_delivery_attempts (id, delivery_id, attempt_number, status, status_code, error, duration_ms, request_headers_json, response_headers_json, attempted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10)`,
		attempt.ID, attempt.DeliveryID, attempt.AttemptNumber, attempt.Status,
		attempt.StatusCode, attempt.Error, attempt.DurationMs, reqH, respH, attempt.AttemptedAt,
	)
	return err
}

type AttemptReadRepository struct {
	db platformpostgres.DBTX
}

func NewAttemptReadRepository(pool *pgxpool.Pool) *AttemptReadRepository {
	return &AttemptReadRepository{db: pool}
}

func NewAttemptReadRepositoryWithDBTX(db platformpostgres.DBTX) *AttemptReadRepository {
	return &AttemptReadRepository{db: db}
}

func (r *AttemptReadRepository) ListByDelivery(ctx context.Context, deliveryID string) ([]domain.WebhookDeliveryAttempt, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, delivery_id, attempt_number, status, status_code, error, duration_ms, request_headers_json, response_headers_json, attempted_at
		FROM customer_webhook_delivery_attempts WHERE delivery_id=$1 ORDER BY attempt_number ASC`,
		deliveryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []domain.WebhookDeliveryAttempt
	for rows.Next() {
		var a domain.WebhookDeliveryAttempt
		var reqH, respH []byte
		err := rows.Scan(
			&a.ID, &a.DeliveryID, &a.AttemptNumber, &a.Status, &a.StatusCode,
			&a.Error, &a.DurationMs, &reqH, &respH, &a.AttemptedAt,
		)
		if err != nil {
			return nil, err
		}
		if len(reqH) > 0 {
			json.Unmarshal(reqH, &a.RequestHeaders)
		}
		if len(respH) > 0 {
			json.Unmarshal(respH, &a.ResponseHeaders)
		}
		if a.RequestHeaders == nil {
			a.RequestHeaders = map[string]string{}
		}
		if a.ResponseHeaders == nil {
			a.ResponseHeaders = map[string]string{}
		}
		attempts = append(attempts, a)
	}
	if attempts == nil {
		attempts = []domain.WebhookDeliveryAttempt{}
	}
	return attempts, rows.Err()
}

type OutboxRepository struct {
	db platformpostgres.DBTX
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{db: pool}
}

func (r *OutboxRepository) Save(ctx context.Context, event ports.OutboxEvent) error {
	db := r.db
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		db = tx
	}
	_, err := db.Exec(ctx,
		`INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, workspace_id, occurred_at) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)`,
		event.ID, event.AggregateType, event.AggregateID, event.EventType, event.Payload, event.WorkspaceID, event.OccurredAt,
	)
	return err
}
