package main

import (
	"context"
	"fmt"
	"log"

	platformclickhouse "github.com/ninggiangboy/send-flow/backend/internal/platform/clickhouse"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
	"github.com/ninggiangboy/send-flow/backend/migrations/clickhouse"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if !cfg.ClickHouseEnabled() {
		log.Fatal("CLICKHOUSE_DSN is not set")
	}

	client, err := platformclickhouse.New(context.Background(), cfg.ClickHouseDSN)
	if err != nil {
		log.Fatalf("connect to clickhouse: %v", err)
	}
	defer client.Close()

	migrationsFS := clickhouse.MigrationsFS()

	if err := platformclickhouse.Migrate(context.Background(), client.Conn(), migrationsFS); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	fmt.Println("clickhouse migrations applied successfully")
}
