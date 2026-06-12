package clickhouse

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var migrationFiles embed.FS

func MigrationsFS() fs.FS {
	return migrationFiles
}
