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

	// The rest is only needed by the HTTP/JSON login the modern client uses; the
	// binary character list sends nothing but the name.
	Level      uint32
	Vocation   uint16
	Sex        uint16
	LookType   uint16
	LookHead   uint16
	LookBody   uint16
	LookLegs   uint16
	LookFeet   uint16
	LookAddons uint16
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
	const q = `SELECT name, deletion, level, vocation, sex,
	                  looktype, lookhead, lookbody, looklegs, lookfeet, lookaddons
	           FROM players WHERE account_id = ? ORDER BY name ASC`
	rows, err := d.SQL.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Character
	for rows.Next() {
		var ch Character
		var deletion int64
		if err := rows.Scan(&ch.Name, &deletion, &ch.Level, &ch.Vocation, &ch.Sex,
			&ch.LookType, &ch.LookHead, &ch.LookBody, &ch.LookLegs, &ch.LookFeet,
			&ch.LookAddons); err != nil {
			return nil, err
		}
		if deletion != 0 {
			continue
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}
