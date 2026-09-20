// Package sqlite — слой хранения. Наверх он отдаёт типы из domain и ошибки
// entity.ErrNotFound: выше по стеку никто не знает, что под капотом SQL.
package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // CGO-free драйвер, поэтому бинарник статический
)

// DB — соединение с базой плюс общие хелперы. Репозитории берут его в поле.
type DB struct{ sql *sql.DB }

// Open открывает файл базы, включает нужные PRAGMA и накатывает миграции.
func Open(path string) (*DB, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// modernc-драйвер не рассчитан на параллельную запись в один файл,
	// поэтому пишем и читаем через одно соединение.
	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(time.Hour)

	db := &DB{sql: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error { return db.sql.Close() }

// Ping нужен healthcheck-у и тестам.
func (db *DB) Ping() error { return db.sql.Ping() }

func now() string { return time.Now().UTC().Format(time.RFC3339) }
