package migration

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

const dir = "migrations"

func Up(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open sql db: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sql db: %w", err)
	}

	sessionLocker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create session locker: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		os.DirFS(dir),
		goose.WithSessionLocker(sessionLocker),
	)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
