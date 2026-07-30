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

// LoadPlayerAnimusMastery loads the animus mastery blob from the canonical
// `players`.`animus_mastery` column and decodes it into the runtime tracker.
//
// The blob is now the same PropStream format C++ uses — a bare sequence of
// length-prefixed lowercase monster names (animus_mastery.cpp serialize/
// unserialize, read at iologindata_load_player.cpp:283). Previously the column was
// an opaque passthrough that was never parsed, so the runtime masteries never
// actually persisted, and anything Go wrote was unreadable by the C++ server.
func (d *DB) LoadPlayerAnimusMastery(ctx context.Context, p *game.Player) error {
	if err := d.SQL.QueryRowContext(ctx,
		`SELECT animus_mastery FROM players WHERE id = ?`, p.DBID).Scan(&p.AnimusMastery); err != nil {
		p.AnimusMastery = nil
		return nil
	}

	p.AnimMastery = game.UnserializeAnimusMastery(p.AnimusMastery, monsterRaceLookup(p))
	return nil
}

// SavePlayerAnimusMastery re-encodes the tracker and persists it.
func (d *DB) SavePlayerAnimusMastery(ctx context.Context, p *game.Player) error {
	if p.AnimMastery != nil {
		p.AnimusMastery = p.AnimMastery.Serialize()
	}
	_, err := d.SQL.ExecContext(ctx,
		`UPDATE players SET animus_mastery = ? WHERE id = ?`, p.AnimusMastery, p.DBID)
	return err
}

// monsterRaceLookup resolves a lowercase monster name to its bestiary race id.
// Returns nil when the registry is unavailable, in which case names are kept with
// race id 0 so the blob still round-trips.
func monsterRaceLookup(p *game.Player) func(string) (uint16, bool) {
	if p == nil || p.World == nil || p.World.TypeRegistry == nil {
		return nil
	}
	registry := p.World.TypeRegistry
	return func(name string) (uint16, bool) {
		if mt, ok := registry.Monsters[name]; ok && mt != nil {
			return mt.RaceID, true
		}
		return 0, false
	}
}
