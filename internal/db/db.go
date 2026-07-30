// Package db provides the MariaDB/MySQL persistence layer: connection pool,
// schema bootstrap, account/player repositories and a table-polled async job
// queue. The database is shared with MyAAC and the login-server, so the schema
// is the canonical Canary MySQL schema.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/kv"
)

// DB wraps a *sql.DB (MariaDB).
type DB struct {
	SQL *sql.DB

	// KV is the key/value store backed by the `kv_store` table. It holds the
	// state C++ keeps out of the players table (weapon proficiency, achievements,
	// wheel gems, attached effects, ...). Nil disables every KV-backed load/save.
	KV *kv.Store
}

// dsn builds a go-sql-driver DSN. multiStatements enables running the schema
// file (multiple statements) in one Exec.
func dsn(cfg *config.Config, multiStatements bool) string {
	extra := "parseTime=true&charset=utf8mb4&loc=UTC"
	if multiStatements {
		extra += "&multiStatements=true"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, extra)
}

// Connect opens the pool and verifies connectivity.
func Connect(ctx context.Context, cfg *config.Config) (*DB, error) {
	pool, err := sql.Open("mysql", dsn(cfg, false))
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	pool.SetConnMaxLifetime(3 * time.Minute)
	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(10)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.PingContext(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return &DB{SQL: pool, KV: kv.New(pool)}, nil
}

// Close releases the pool.
func (d *DB) Close() { _ = d.SQL.Close() }

// ApplySchema runs a MySQL schema file (may contain multiple statements). It
// opens a dedicated multiStatements connection so it does not require the pool
// to be configured for it.
func (d *DB) ApplySchema(ctx context.Context, cfg *config.Config, path string) error {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("db: read schema: %w", err)
	}
	conn, err := sql.Open("mysql", dsn(cfg, true))
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("db: apply schema %s: %w", path, err)
	}
	return nil
}

// ApplySchemaIfPresent applies path only if it exists (best-effort dev helper).
func (d *DB) ApplySchemaIfPresent(ctx context.Context, cfg *config.Config, path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return d.ApplySchema(ctx, cfg, path)
}

func (d *DB) GetBoostedCreature(ctx context.Context) (string, error) {
	var name string
	err := d.SQL.QueryRowContext(ctx, "SELECT boostname FROM boosted_creature WHERE boostname != 'default' AND boostname != '' ORDER BY date DESC LIMIT 1").Scan(&name)
	if err != nil {
		_ = d.SQL.QueryRowContext(ctx, "SELECT boostname FROM boosted_creature LIMIT 1").Scan(&name)
	}
	return name, nil
}

func (d *DB) GetBoostedBoss(ctx context.Context) (string, error) {
	var name string
	err := d.SQL.QueryRowContext(ctx, "SELECT boostname FROM boosted_boss WHERE boostname != 'default' AND boostname != '' ORDER BY date DESC LIMIT 1").Scan(&name)
	if err != nil {
		_ = d.SQL.QueryRowContext(ctx, "SELECT boostname FROM boosted_boss LIMIT 1").Scan(&name)
	}
	return name, nil
}

// AddPlayerOnline registers a player as online in the players_online table.
func (d *DB) AddPlayerOnline(ctx context.Context, playerID uint32) error {
	_, err := d.SQL.ExecContext(ctx, "INSERT IGNORE INTO players_online (player_id) VALUES (?)", playerID)
	return err
}

// RemovePlayerOnline removes a player from the players_online table.
func (d *DB) RemovePlayerOnline(ctx context.Context, playerID uint32) error {
	_, err := d.SQL.ExecContext(ctx, "DELETE FROM players_online WHERE player_id = ?", playerID)
	return err
}
