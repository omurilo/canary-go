package db

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("db: not found")

// Account is the subset of accounts columns used for login.
type Account struct {
	ID            uint32
	Name          string
	Password      string
	Type          int
	PremDays      int
	PremDaysPurch int
	LastDay       int64
	Creation      int64
}

// Character is a listed character on an account.
type Character struct {
	Name    string
	Deleted bool
}

// LoadAccount fetches an account by email (modern) or name (old protocol).
func (d *DB) LoadAccount(ctx context.Context, identifier string) (*Account, error) {
	const q = `SELECT id, name, password, type, premdays, premdays_purchased, lastday, creation
	           FROM accounts WHERE email = ? OR name = ? LIMIT 1`
	var a Account
	err := d.SQL.QueryRowContext(ctx, q, identifier, identifier).Scan(
		&a.ID, &a.Name, &a.Password, &a.Type, &a.PremDays, &a.PremDaysPurch, &a.LastDay, &a.Creation)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ListCharacters returns non-deleted characters for an account, ordered by name.
func (d *DB) ListCharacters(ctx context.Context, accountID uint32) ([]Character, error) {
	const q = `SELECT name, deletion FROM players WHERE account_id = ? ORDER BY name ASC`
	rows, err := d.SQL.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Character
	for rows.Next() {
		var name string
		var deletion int64
		if err := rows.Scan(&name, &deletion); err != nil {
			return nil, err
		}
		if deletion != 0 {
			continue
		}
		out = append(out, Character{Name: name})
	}
	return out, rows.Err()
}
