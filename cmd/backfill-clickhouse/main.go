package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/clickhouse"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

type pgFact struct {
	ID                string
	SourceEventID     string
	SourceEventType   string
	WorkspaceID       string
	CampaignID        *string
	MessageID         *string
	Provider          *string
	ProviderMessageID *string
	ProviderEventID   *string
	EventType         string
	RecipientDomain   *string
	OccurredAt        time.Time
	ReceivedAt        time.Time
	MetadataJSON      []byte
	CreatedAt         time.Time
}

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if !cfg.ClickHouseEnabled() {
		log.Fatal("CLICKHOUSE_DSN is not set")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	batchSize := 1000
	if s := os.Getenv("BACKFILL_BATCH_SIZE"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			batchSize = v
		}
	}

	ctx := context.Background()

	pgPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer pgPool.Close()

	chClient, err := clickhouse.New(ctx, cfg.ClickHouseDSN)
	if err != nil {
		log.Fatalf("connect to clickhouse: %v", err)
	}
	defer chClient.Close()

	log.Printf("starting backfill from postgres to clickhouse (batch size: %d)", batchSize)
	backfill(ctx, pgPool, chClient, batchSize)
}

func backfill(ctx context.Context, pgPool *pgxpool.Pool, chClient *clickhouse.Client, batchSize int) {
	lastID := ""
	total := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("context cancelled after %d records", total)
			return
		default:
		}

		rows, err := pgPool.Query(ctx, `
			SELECT id, source_event_id, source_event_type, workspace_id,
			       campaign_id, message_id, provider, provider_message_id, provider_event_id,
			       event_type, recipient_domain, occurred_at, received_at, metadata_json, created_at
			FROM analytics_event_facts
			WHERE id > $1
			ORDER BY id
			LIMIT $2
		`, lastID, batchSize)
		if err != nil {
			log.Fatalf("query postgres: %v", err)
		}

		var facts []pgFact
		for rows.Next() {
			var f pgFact
			if err := rows.Scan(
				&f.ID, &f.SourceEventID, &f.SourceEventType, &f.WorkspaceID,
				&f.CampaignID, &f.MessageID, &f.Provider, &f.ProviderMessageID, &f.ProviderEventID,
				&f.EventType, &f.RecipientDomain, &f.OccurredAt, &f.ReceivedAt, &f.MetadataJSON, &f.CreatedAt,
			); err != nil {
				log.Fatalf("scan row: %v", err)
			}
			facts = append(facts, f)
		}
		rows.Close()

		if err := rows.Err(); err != nil {
			log.Fatalf("iterate postgres rows: %v", err)
		}

		if len(facts) == 0 {
			break
		}

		batch, err := chClient.Conn().PrepareBatch(ctx, `
			INSERT INTO email_events (
				source_event_id, source_event_type, workspace_id, campaign_id, message_id,
				provider, provider_message_id, provider_event_id, event_type, recipient_domain,
				occurred_at, received_at, metadata_json, created_at
			) VALUES (
				?, ?, ?, ?, ?,
				?, ?, ?, ?, ?,
				?, ?, ?, ?
			)
		`)
		if err != nil {
			log.Fatalf("prepare clickhouse batch: %v", err)
		}

		for _, f := range facts {
			mdStr := "{}"
			if f.MetadataJSON != nil {
				var md any
				if err := json.Unmarshal(f.MetadataJSON, &md); err == nil {
					mdBytes, _ := json.Marshal(md)
					mdStr = string(mdBytes)
				}
			}

			campaignID := ""
			if f.CampaignID != nil {
				campaignID = *f.CampaignID
			}
			messageID := ""
			if f.MessageID != nil {
				messageID = *f.MessageID
			}
			provider := ""
			if f.Provider != nil {
				provider = *f.Provider
			}
			providerMessageID := ""
			if f.ProviderMessageID != nil {
				providerMessageID = *f.ProviderMessageID
			}
			providerEventID := ""
			if f.ProviderEventID != nil {
				providerEventID = *f.ProviderEventID
			}
			recipientDomain := ""
			if f.RecipientDomain != nil {
				recipientDomain = *f.RecipientDomain
			}

			if err := batch.Append(
				f.SourceEventID, f.SourceEventType, f.WorkspaceID, campaignID, messageID,
				provider, providerMessageID, providerEventID, f.EventType, recipientDomain,
				f.OccurredAt, f.ReceivedAt, mdStr, f.CreatedAt,
			); err != nil {
				log.Fatalf("append to batch: %v", err)
			}
		}

		if err := batch.Send(); err != nil {
			log.Fatalf("send clickhouse batch: %v", err)
		}

		total += len(facts)
		lastID = facts[len(facts)-1].ID
		log.Printf("backfilled %d records (total: %d, last id: %s)", len(facts), total, lastID)
	}

	if total == 0 {
		fmt.Println("no records to backfill")
	} else {
		fmt.Printf("backfill complete: %d records migrated to clickhouse\n", total)
	}
}


