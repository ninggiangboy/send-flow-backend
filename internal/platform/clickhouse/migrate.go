package clickhouse

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func Migrate(ctx context.Context, conn driver.Conn, migrationsFS fs.FS) error {
	if err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version String,
		applied_at DateTime DEFAULT now()
	) ENGINE = TinyLog`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		var count uint64
		if err := conn.QueryRow(ctx, "SELECT count() FROM schema_migrations WHERE version = ?", file).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", file, err)
		}
		if count > 0 {
			continue
		}

		content, err := fs.ReadFile(migrationsFS, file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		statements := splitStatements(string(content))
		for _, stmt := range statements {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if err := conn.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("execute migration %s: %w", file, err)
			}
		}

		if err := conn.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", file); err != nil {
			return fmt.Errorf("record migration %s: %w", file, err)
		}
	}

	return nil
}

func splitStatements(s string) []string {
	var stmts []string
	var current strings.Builder
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(strings.TrimRight(trimmed, " \r"), ";") {
			stmts = append(stmts, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		remaining := strings.TrimSpace(current.String())
		if remaining != "" {
			stmts = append(stmts, remaining)
		}
	}
	return stmts
}
