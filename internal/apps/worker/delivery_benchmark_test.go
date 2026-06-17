//go:build integration

package worker

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	deliverypostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/postgres"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func benchmarkSetupDB(b *testing.B) *pgxpool.Pool {
	b.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("sendflow_bench"),
		tcpostgres.WithUsername("sendflow"),
		tcpostgres.WithPassword("sendflow"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		b.Fatalf("start postgres container: %v", err)
	}
	b.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		b.Fatalf("build dsn: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		b.Fatalf("new pg pool: %v", err)
	}
	b.Cleanup(pool.Close)

	schema := []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id), campaign_id TEXT, campaign_candidate_id TEXT, transactional_request_id TEXT, contact_id TEXT, recipient_email_normalized TEXT NOT NULL, recipient_snapshot JSONB DEFAULT '{}'::jsonb, template_id TEXT, template_version_id TEXT, sender_domain_id TEXT, message_type TEXT NOT NULL, source_type TEXT NOT NULL, status TEXT NOT NULL, scheduled_at TIMESTAMPTZ, queued_at TIMESTAMPTZ, processing_started_at TIMESTAMPTZ, accepted_at TIMESTAMPTZ, delivered_at TIMESTAMPTZ, bounced_at TIMESTAMPTZ, complained_at TIMESTAMPTZ, failed_at TIMESTAMPTZ, last_error_class TEXT, last_error_message TEXT, provider TEXT, provider_message_id TEXT, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
	}
	for _, stmt := range schema {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			b.Fatalf("create schema: %v", err)
		}
	}
	pool.Exec(ctx, `INSERT INTO workspaces (id, name, created_at, updated_at) VALUES ($1,$2,$3,$4)`, "ws_bench", "Benchmark", time.Now().UTC(), time.Now().UTC())

	return pool
}

// BenchmarkMailLogListing measures mail log query performance
func BenchmarkMailLogListing(b *testing.B) {
	pool := benchmarkSetupDB(b)
	ctx := context.Background()
	repo := deliverypostgres.NewMessageReadRepository(pool)
	now := time.Now().UTC()

	// Pre-populate 1000 messages
	b.Log("Pre-populating 1000 messages...")
	for i := 0; i < 1000; i++ {
		ts := now.Add(time.Duration(i) * time.Second)
		statuses := []string{"queued", "processing", "accepted", "delivered", "failed", "bounced"}
		status := statuses[i%len(statuses)]
		_, err := pool.Exec(ctx,
			`INSERT INTO messages (id, workspace_id, recipient_email_normalized, message_type, source_type, status, recipient_role, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			benchMsgID(i), "ws_bench", benchEmail(i), "transactional", "api", status, "to", ts, ts)
		if err != nil {
			b.Fatalf("seed message %d: %v", i, err)
		}
	}

	b.ResetTimer()

	b.Run("ListAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _, err := repo.List(ctx, ports.MessageListQuery{WorkspaceID: "ws_bench", Limit: 50})
			if err != nil {
				b.Fatalf("List: %v", err)
			}
		}
	})

	b.Run("FilterByStatus", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _, err := repo.List(ctx, ports.MessageListQuery{WorkspaceID: "ws_bench", Status: "delivered", Limit: 50})
			if err != nil {
				b.Fatalf("List: %v", err)
			}
		}
	})
}

func benchMsgID(i int) string {
	return "bm_" + string(rune('a'+i%26)) + string(rune('0'+i%10))
}

func benchEmail(i int) string {
	return "user" + string(rune('0'+i%10)) + "@bench.com"
}
