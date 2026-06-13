package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

var _ ports.FactBatchRepository = (*FactBatchRepository)(nil)
var _ ports.SyncStateRepository = (*SyncStateRepository)(nil)

type FactBatchRepository struct {
	db platformpostgres.DBTX
}

func NewFactBatchRepository(db platformpostgres.DBTX) *FactBatchRepository {
	return &FactBatchRepository{db: db}
}

func (r *FactBatchRepository) ListFactsAfterCursor(ctx context.Context, cursorCreatedAt *time.Time, cursorID string, limit int) ([]domain.EmailEventFact, error) {
	db := getDB(ctx, r.db)

	var rows pgx.Rows
	var err error
	if cursorCreatedAt == nil {
		rows, err = db.Query(ctx, `
			SELECT id, source_event_id, source_event_type, workspace_id,
			       campaign_id, message_id, provider, provider_message_id, provider_event_id,
			       event_type, recipient_domain, occurred_at, received_at, metadata_json, created_at
			FROM analytics_event_facts
			ORDER BY created_at, id
			LIMIT $1
		`, limit)
	} else {
		rows, err = db.Query(ctx, `
			SELECT id, source_event_id, source_event_type, workspace_id,
			       campaign_id, message_id, provider, provider_message_id, provider_event_id,
			       event_type, recipient_domain, occurred_at, received_at, metadata_json, created_at
			FROM analytics_event_facts
			WHERE (created_at, id) > ($1::timestamptz, $2)
			ORDER BY created_at, id
			LIMIT $3
		`, cursorCreatedAt, cursorID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []domain.EmailEventFact
	for rows.Next() {
		var f domain.EmailEventFact
		var campaignID, messageID, provider, providerMessageID, providerEventID, recipientDomain *string
		var md []byte

		if err := rows.Scan(
			&f.ID, &f.SourceEventID, &f.SourceEventType, &f.WorkspaceID,
			&campaignID, &messageID, &provider, &providerMessageID, &providerEventID,
			&f.EventType, &recipientDomain, &f.OccurredAt, &f.ReceivedAt,
			&md, &f.CreatedAt,
		); err != nil {
			return nil, err
		}

		if campaignID != nil {
			f.CampaignID = *campaignID
		}
		if messageID != nil {
			f.MessageID = *messageID
		}
		if provider != nil {
			f.Provider = *provider
		}
		if providerMessageID != nil {
			f.ProviderMessageID = *providerMessageID
		}
		if providerEventID != nil {
			f.ProviderEventID = *providerEventID
		}
		if recipientDomain != nil {
			f.RecipientDomain = *recipientDomain
		}
		if md != nil {
			json.Unmarshal(md, &f.Metadata)
		}

		facts = append(facts, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(facts) == 0 {
		return facts, nil
	}

	return facts, nil
}

type SyncStateRepository struct {
	db platformpostgres.DBTX
}

func NewSyncStateRepository(db platformpostgres.DBTX) *SyncStateRepository {
	return &SyncStateRepository{db: db}
}

func (r *SyncStateRepository) GetSyncCursor(ctx context.Context, streamName string) (*ports.SyncCursor, error) {
	db := getDB(ctx, r.db)
	var cursor ports.SyncCursor
	err := db.QueryRow(ctx,
		`SELECT stream_name, last_created_at, last_fact_id, last_synced_at
		 FROM clickhouse_sync_offsets WHERE stream_name = $1`,
		streamName,
	).Scan(&cursor.StreamName, &cursor.LastCreatedAt, &cursor.LastFactID, &cursor.LastSyncedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &ports.SyncCursor{
				StreamName: streamName,
			}, nil
		}
		return nil, err
	}
	return &cursor, nil
}

func (r *SyncStateRepository) UpdateSyncCursor(ctx context.Context, cursor *ports.SyncCursor) error {
	db := getDB(ctx, r.db)
	_, err := db.Exec(ctx,
		`INSERT INTO clickhouse_sync_offsets (stream_name, last_created_at, last_fact_id, last_synced_at, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (stream_name) DO UPDATE SET
		   last_created_at = EXCLUDED.last_created_at,
		   last_fact_id = EXCLUDED.last_fact_id,
		   last_synced_at = EXCLUDED.last_synced_at,
		   updated_at = NOW()`,
		cursor.StreamName, cursor.LastCreatedAt, cursor.LastFactID, cursor.LastSyncedAt,
	)
	return err
}

func (r *FactBatchRepository) WithTx(tx pgx.Tx) *FactBatchRepository {
	return &FactBatchRepository{db: tx}
}

func (r *SyncStateRepository) WithTx(tx pgx.Tx) *SyncStateRepository {
	return &SyncStateRepository{db: tx}
}
