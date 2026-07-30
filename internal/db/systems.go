package db

import (
	"context"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/kv"
)

// Hazard points and concoctions used to be persisted by ALTERing the canonical
// `players` table at runtime to add `hazard_points` and `concoctions`, neither of
// which exists in the C++ schema. Mutating the shared table is worse than an
// extra table: MyAAC and the login-server read the same database, and the
// canonical schema is the contract between them.
//
// Both now live in the KV store, which is where C++ keeps player state that has
// no column. C++ has no hazard or concoction persistence in its core at all, so
// there is no upstream key layout to copy — these scopes are canary-go's own, but
// they are namespaced per player exactly like the ported ones.
const (
	scopeHazard      = "hazard"
	scopeConcoctions = "concoctions"

	keyHazardPoints = "points"
)

// LoadPlayerHazard reads the hazard points.
func (d *DB) LoadPlayerHazard(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	value, found, err := d.KV.PlayerScope(p.DBID).Scoped(scopeHazard).Get(ctx, keyHazardPoints)
	if err != nil {
		return err
	}
	if found {
		if points := value.GetInt(); points > 0 {
			p.HazardPoints = uint32(points)
		}
	}
	return nil
}

// SavePlayerHazard writes the hazard points.
func (d *DB) SavePlayerHazard(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	return d.KV.PlayerScope(p.DBID).Scoped(scopeHazard).
		Set(ctx, keyHazardPoints, kv.Int(int32(p.HazardPoints)))
}

// LoadPlayerConcoctions reads the active concoctions as name → expiry.
func (d *DB) LoadPlayerConcoctions(ctx context.Context, p *game.Player) error {
	p.Concoctions = make(map[string]int64)
	if d.KV == nil {
		return nil
	}

	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeConcoctions)
	names, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	for _, name := range names {
		value, found, err := scope.Get(ctx, name)
		if err != nil {
			return err
		}
		if found {
			p.Concoctions[name] = int64(value.GetInt())
		}
	}
	return nil
}

// SavePlayerConcoctions writes the active concoctions, removing those that are no
// longer held.
func (d *DB) SavePlayerConcoctions(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeConcoctions)

	existing, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	for _, name := range existing {
		if _, ok := p.Concoctions[name]; !ok {
			if err := scope.Remove(ctx, name); err != nil {
				return err
			}
		}
	}
	for name, expiry := range p.Concoctions {
		if err := scope.Set(ctx, name, kv.Int(int32(expiry))); err != nil {
			return err
		}
	}
	return nil
}

// LoadPlayerAnimusMastery loads the animus mastery blob. This one IS a canonical
// column (`players`.`animus_mastery`), so it stays in SQL.
//
// The encoding still differs from C++, which writes it with PropStream
// (iologindata_load_player.cpp:283) while Go uses its own layout — the column is
// shared but its contents are not yet interchangeable.
func (d *DB) LoadPlayerAnimusMastery(ctx context.Context, p *game.Player) error {
	err := d.SQL.QueryRowContext(ctx, `SELECT animus_mastery FROM players WHERE id = ?`, p.DBID).Scan(&p.AnimusMastery)
	if err != nil {
		p.AnimusMastery = nil
	}
	return nil
}

// SavePlayerAnimusMastery persists the animus mastery blob.
func (d *DB) SavePlayerAnimusMastery(ctx context.Context, p *game.Player) error {
	_, err := d.SQL.ExecContext(ctx, `UPDATE players SET animus_mastery = ? WHERE id = ?`, p.AnimusMastery, p.DBID)
	return err
}
