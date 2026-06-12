package clickhouse

import (
	"context"
	"encoding/json"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type FactRepository struct {
	conn driver.Conn
}

func NewFactRepository(conn driver.Conn) *FactRepository {
	return &FactRepository{conn: conn}
}

func (r *FactRepository) Create(ctx context.Context, fact domain.EmailEventFact) error {
	md, err := json.Marshal(fact.Metadata)
	if err != nil {
		return err
	}

	return r.conn.Exec(ctx,
		`INSERT INTO email_events (
			source_event_id, source_event_type, workspace_id, campaign_id, message_id,
			provider, provider_message_id, provider_event_id, event_type, recipient_domain,
			occurred_at, received_at, metadata_json, created_at
		) VALUES (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?
		)`,
		fact.SourceEventID, fact.SourceEventType, fact.WorkspaceID, fact.CampaignID, fact.MessageID,
		fact.Provider, fact.ProviderMessageID, fact.ProviderEventID, fact.EventType, fact.RecipientDomain,
		fact.OccurredAt, fact.ReceivedAt, string(md), fact.CreatedAt,
	)
}

func (r *FactRepository) FindBySourceEventID(ctx context.Context, sourceEventID string) (*domain.EmailEventFact, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT
			source_event_id, source_event_type, workspace_id, campaign_id, message_id,
			provider, provider_message_id, provider_event_id, event_type, recipient_domain,
			occurred_at, received_at, metadata_json, created_at
		FROM email_events
		WHERE source_event_id = ?
		ORDER BY created_at DESC
		LIMIT 1`,
		sourceEventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, domain.ErrAnalyticsProjectionNotFound
	}

	var fact domain.EmailEventFact
	var campaignID, messageID, provider, providerMessageID, providerEventID, recipientDomain *string
	var md string

	if err := rows.Scan(
		&fact.SourceEventID, &fact.SourceEventType, &fact.WorkspaceID,
		&campaignID, &messageID, &provider, &providerMessageID, &providerEventID,
		&fact.EventType, &recipientDomain,
		&fact.OccurredAt, &fact.ReceivedAt, &md, &fact.CreatedAt,
	); err != nil {
		return nil, err
	}

	if campaignID != nil {
		fact.CampaignID = *campaignID
	}
	if messageID != nil {
		fact.MessageID = *messageID
	}
	if provider != nil {
		fact.Provider = *provider
	}
	if providerMessageID != nil {
		fact.ProviderMessageID = *providerMessageID
	}
	if providerEventID != nil {
		fact.ProviderEventID = *providerEventID
	}
	if recipientDomain != nil {
		fact.RecipientDomain = *recipientDomain
	}
	if md != "" {
		json.Unmarshal([]byte(md), &fact.Metadata)
	}

	return &fact, nil
}
