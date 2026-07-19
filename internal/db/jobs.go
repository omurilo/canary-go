package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Enqueue inserts an async job for the worker to pick up.
func (d *DB) Enqueue(ctx context.Context, kind string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = d.SQL.ExecContext(ctx,
		`INSERT INTO async_jobs (kind, payload) VALUES (?, ?)`, kind, string(raw))
	return err
}

// JobHandler processes a single job payload.
type JobHandler func(ctx context.Context, kind string, payload json.RawMessage) error

// RunJobWorker polls the async_jobs table and dispatches pending jobs. MariaDB
// lacks LISTEN/NOTIFY, so this uses a short poll with SELECT ... FOR UPDATE SKIP
// LOCKED so multiple workers can share the queue safely. Blocks until ctx ends.
func (d *DB) RunJobWorker(ctx context.Context, log *slog.Logger, handle JobHandler) error {
	log.Info("async job worker polling table async_jobs")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		d.drainJobs(ctx, log, handle)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (d *DB) drainJobs(ctx context.Context, log *slog.Logger, handle JobHandler) {
	for {
		claimed, err := d.claimAndRun(ctx, handle)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Warn("job processing error", "err", err)
			}
			return
		}
		if !claimed {
			return
		}
	}
}

// claimAndRun claims one pending job in a transaction and runs it. Returns
// false when the queue is empty.
func (d *DB) claimAndRun(ctx context.Context, handle JobHandler) (bool, error) {
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck

	var id int64
	var kind string
	var payload []byte
	err = tx.QueryRowContext(ctx,
		`SELECT id, kind, payload FROM async_jobs WHERE status='pending'
		 ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &kind, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE async_jobs SET status='running', updated_at=NOW() WHERE id=?`, id); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}

	status := "done"
	if herr := handle(ctx, kind, payload); herr != nil {
		status = "error"
	}
	_, _ = d.SQL.ExecContext(ctx,
		`UPDATE async_jobs SET status=?, updated_at=NOW() WHERE id=?`, status, id)
	return true, nil
}

// SeedTestAccount creates (or updates) an account + one character for local
// testing. Password is stored as legacy SHA-1.
func (d *DB) SeedTestAccount(ctx context.Context, accountName, email, password, charName string) error {
	pwHash := SHA1Hex(password)
	res, err := d.SQL.ExecContext(ctx,
		`INSERT INTO accounts (name, email, password, type, premdays)
		 VALUES (?, ?, ?, 5, 999)
		 ON DUPLICATE KEY UPDATE email=VALUES(email), password=VALUES(password)`,
		accountName, email, pwHash)
	if err != nil {
		return fmt.Errorf("seed account: %w", err)
	}
	accountID, _ := res.LastInsertId()
	if accountID == 0 {
		// ON DUPLICATE path: fetch the existing id.
		if err := d.SQL.QueryRowContext(ctx,
			`SELECT id FROM accounts WHERE name=?`, accountName).Scan(&accountID); err != nil {
			return fmt.Errorf("seed account lookup: %w", err)
		}
	}
	_, err = d.SQL.ExecContext(ctx,
		`INSERT INTO players (name, account_id, level, health, healthmax, mana, manamax,
		                      looktype, posx, posy, posz, town_id, vocation, conditions)
		 VALUES (?, ?, 8, 185, 185, 90, 90, 128, 0, 0, 7, 1, 1, '')
		 ON DUPLICATE KEY UPDATE account_id=VALUES(account_id)`,
		charName, accountID)
	if err != nil {
		return fmt.Errorf("seed player: %w", err)
	}
	return nil
}
