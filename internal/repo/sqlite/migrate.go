package sqlite

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrate накатывает файлы из migrations/ по одному разу, в порядке имён.
// Применённые версии запоминаются в schema_migrations — тот же приём, что у
// goose или golang-migrate, только без зависимости ради трёх файлов.
func (db *DB) migrate() error {
	if _, err := db.sql.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return err
	}

	applied := map[string]bool{}
	rows, err := db.sql.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return err
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	files, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, f := range files {
		version := strings.TrimSuffix(strings.TrimPrefix(f, "migrations/"), ".sql")
		if applied[version] {
			continue
		}
		body, err := migrationFS.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := db.sql.Exec(string(body)); err != nil {
			// База могла появиться до schema_migrations и уже содержать колонку.
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("%s: %w", version, err)
			}
		}
		if _, err := db.sql.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`, version, now()); err != nil {
			return err
		}
	}
	return nil
}
