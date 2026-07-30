package db

import (
	"context"
	"strconv"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/kv"
)

// Unlocked familiars move from the invented `player_familiars` table into the KV
// store. C++ has no per-player familiar persistence in its core — familiars.cpp
// only parses the XML definitions — so there is no upstream layout to copy; this
// scope is canary-go's own.
const scopeFamiliars = "familiars"

// LoadPlayerFamiliars reads the unlocked familiars, keyed by looktype.
func (d *DB) LoadPlayerFamiliars(ctx context.Context, p *game.Player) error {
	p.Familiars = nil
	if d.KV == nil {
		return nil
	}

	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeFamiliars)
	keys, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		lookType, ok := parseUint16Key(key)
		if !ok {
			continue
		}
		p.Familiars = append(p.Familiars, game.Familiar{
			LookType: lookType,
			Unlocked: true,
		})
	}
	return nil
}

// SavePlayerFamiliars writes the unlocked familiars, removing those that are no
// longer unlocked.
func (d *DB) SavePlayerFamiliars(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeFamiliars)

	unlocked := make(map[uint16]struct{}, len(p.Familiars))
	for _, f := range p.Familiars {
		if f.Unlocked {
			unlocked[f.LookType] = struct{}{}
		}
	}

	existing, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	for _, key := range existing {
		lookType, ok := parseUint16Key(key)
		if !ok {
			if err := scope.Remove(ctx, key); err != nil {
				return err
			}
			continue
		}
		if _, still := unlocked[lookType]; !still {
			if err := scope.Remove(ctx, key); err != nil {
				return err
			}
		}
	}

	for lookType := range unlocked {
		key := strconv.FormatUint(uint64(lookType), 10)
		if err := scope.Set(ctx, key, kv.Bool(true)); err != nil {
			return err
		}
	}
	return nil
}

// parseUint16Key accepts a bare integer that fits in uint16.
func parseUint16Key(key string) (uint16, bool) {
	v, err := strconv.ParseUint(key, 10, 64)
	if err != nil || v > 0xFFFF {
		return 0, false
	}
	return uint16(v), true
}
