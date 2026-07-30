package db

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/opentibiabr/canary-go/internal/game"
)

// weaponProficiencyScope is the KV scope C++ uses
// (weapon_proficiency.cpp:255: m_player.kv()->scoped("weapon-proficiency")).
const weaponProficiencyScope = "weapon-proficiency"

// LoadPlayerWeaponProficiency ports WeaponProficiency::load
// (weapon_proficiency.cpp:250). Each key under the scope is a weapon id and the
// value is the serialized WeaponProficiencyData.
//
// Key validation mirrors C++: a key that is not a plain positive integer within
// uint16 range is logged and skipped rather than aborting the load. C++ also
// rejects ids whose ItemType has proficiencyId == 0; that field is not parsed by
// the Go item catalog yet, so this only enforces the numeric range.
func (d *DB) LoadPlayerWeaponProficiency(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	p.Proficiency = make(map[uint16]game.WeaponProficiencyData)

	scope := d.KV.PlayerScope(p.DBID).Scoped(weaponProficiencyScope)
	keys, err := scope.Keys(ctx)
	if err != nil {
		return err
	}

	for _, key := range keys {
		weaponID, ok := parseWeaponKey(key)
		if !ok {
			slog.Default().Warn("skipping invalid weapon-proficiency key",
				"key", key, "player", p.Name)
			continue
		}

		value, found, err := scope.Get(ctx, key)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		p.Proficiency[weaponID] = game.WeaponProficiencyDataFromKV(value)
	}

	// Derive the aggregated bonuses from the perks just loaded. C++ gets there by
	// calling applyPerks as each perk is selected; on load it has to be rebuilt.
	p.RebuildWeaponProficiency()
	return nil
}

// SavePlayerWeaponProficiency ports WeaponProficiency::saveAll
// (weapon_proficiency.cpp:294): stored keys that are invalid or no longer tracked
// are removed, then every tracked weapon is written.
func (d *DB) SavePlayerWeaponProficiency(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	scope := d.KV.PlayerScope(p.DBID).Scoped(weaponProficiencyScope)

	keys, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		weaponID, ok := parseWeaponKey(key)
		if !ok {
			if err := scope.Remove(ctx, key); err != nil {
				return err
			}
			continue
		}
		if _, tracked := p.Proficiency[weaponID]; !tracked {
			if err := scope.Remove(ctx, key); err != nil {
				return err
			}
		}
	}

	for weaponID, data := range p.Proficiency {
		if err := scope.Set(ctx, strconv.FormatUint(uint64(weaponID), 10), data.ToKV()); err != nil {
			return err
		}
	}
	return nil
}

// parseWeaponKey accepts only a bare positive integer that fits in uint16, the
// same contract as the std::from_chars checks in C++ (ptr must reach the end, so
// "12x" and " 12" are rejected).
func parseWeaponKey(key string) (uint16, bool) {
	v, err := strconv.ParseUint(key, 10, 64)
	if err != nil || v == 0 || v > 0xFFFF {
		return 0, false
	}
	return uint16(v), true
}
