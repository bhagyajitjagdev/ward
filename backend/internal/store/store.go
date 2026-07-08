// Package store owns the database connection, dialect selection, and migrations.
//
// SQLite first (pure-Go modernc driver via bun's sqliteshim → no CGO, single
// static binary); Postgres when a postgres:// DSN is configured. Migrations run
// on Open via goose (embedded).
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"

	"github.com/bhagyajitjagdev/ward/backend/migrations"
)

// Store wraps the bun DB handle and the resolved dialect.
type Store struct {
	DB      *bun.DB
	dialect string
}

// Open selects the dialect from the DSN, connects, runs migrations, and returns a Store.
func Open(dsn string) (*Store, error) {
	var (
		sqldb   *sql.DB
		bdb     *bun.DB
		dialect string
		err     error
	)

	switch {
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		sqldb = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		bdb = bun.NewDB(sqldb, pgdialect.New())
		dialect = "postgres"
	default:
		sqldb, err = sql.Open(sqliteshim.ShimName, dsn)
		if err != nil {
			return nil, fmt.Errorf("open sqlite: %w", err)
		}
		sqldb.SetMaxOpenConns(1) // SQLite is single-writer; serialize
		bdb = bun.NewDB(sqldb, sqlitedialect.New())
		dialect = "sqlite3"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqldb.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect(dialect); err != nil {
		return nil, fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(sqldb, "."); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{DB: bdb, dialect: dialect}, nil
}

// Dialect reports the resolved dialect ("sqlite3" or "postgres").
func (s *Store) Dialect() string { return s.dialect }

// Close closes the underlying connection.
func (s *Store) Close() error { return s.DB.Close() }
