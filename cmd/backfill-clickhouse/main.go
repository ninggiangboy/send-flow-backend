package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	analyticsclickhouse "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/infrastructure/clickhouse"
	analyticspostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/infrastructure/postgres"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/clickhouse"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

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

	var fromCreatedAt, toCreatedAt *time.Time
	if s := os.Getenv("BACKFILL_FROM_CREATED_AT"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			log.Fatalf("invalid BACKFILL_FROM_CREATED_AT: %v", err)
		}
		fromCreatedAt = &t
	}
	if s := os.Getenv("BACKFILL_TO_CREATED_AT"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			log.Fatalf("invalid BACKFILL_TO_CREATED_AT: %v", err)
		}
		toCreatedAt = &t
	}

	resetClickHouse, _ := strconv.ParseBool(os.Getenv("BACKFILL_RESET_CLICKHOUSE"))

	updateCursor, _ := strconv.ParseBool(os.Getenv("BACKFILL_UPDATE_LIVE_CURSOR"))

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

	if resetClickHouse {
		log.Println("truncating clickhouse email_events table")
		if err := chClient.Conn().Exec(ctx, "TRUNCATE TABLE email_events"); err != nil {
			log.Fatalf("truncate clickhouse: %v", err)
		}
	}

	factRepo := analyticspostgres.NewFactBatchRepository(pgPool)
	batchWriter := analyticsclickhouse.NewBatchWriter(chClient.Conn())
	syncStateRepo := analyticspostgres.NewSyncStateRepository(pgPool)

	log.Printf("starting backfill from postgres to clickhouse (batch size: %d)", batchSize)

	cursorCreatedAt := fromCreatedAt
	cursorID := ""
	total := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("context cancelled after %d records", total)
			return
		default:
		}

		if toCreatedAt != nil && cursorCreatedAt != nil && cursorCreatedAt.After(*toCreatedAt) {
			break
		}

		facts, err := factRepo.ListFactsAfterCursor(ctx, cursorCreatedAt, cursorID, batchSize)
		if err != nil {
			log.Fatalf("query postgres: %v", err)
		}

		if len(facts) == 0 {
			break
		}

		if toCreatedAt != nil {
			cut := 0
			for i, f := range facts {
				if f.CreatedAt.After(*toCreatedAt) {
					break
				}
				cut = i + 1
			}
			if cut == 0 {
				break
			}
			facts = facts[:cut]
		}

		if err := batchWriter.CreateBatch(ctx, facts); err != nil {
			log.Fatalf("send clickhouse batch: %v", err)
		}

		total += len(facts)
		last := facts[len(facts)-1]
		cursorCreatedAt = &last.CreatedAt
		cursorID = last.ID
		log.Printf("backfilled %d records (total: %d, last_id: %s)", len(facts), total, last.ID)
	}

	if total == 0 {
		fmt.Println("no records to backfill")
	} else {
		fmt.Printf("backfill complete: %d records migrated to clickhouse\n", total)
	}

	if updateCursor {
		now := time.Now()
		err := syncStateRepo.UpdateSyncCursor(ctx, &ports.SyncCursor{
			StreamName:    "analytics_email_events",
			LastCreatedAt: cursorCreatedAt,
			LastFactID:    cursorID,
			LastSyncedAt:  &now,
		})
		if err != nil {
			log.Printf("warning: failed to update live sync cursor: %v", err)
		} else {
			fmt.Println("live sync cursor updated to reflect backfilled position")
		}
	}
}
