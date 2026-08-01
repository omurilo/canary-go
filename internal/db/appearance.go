package db

import (
	"context"
	"strconv"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/kv"
)

// Unlocked mounts and outfits move out of the invented `player_mounts` and
// `player_outfits` tables into the KV store.
//
// The C++ core does not persist either list: Player::addOutfit (player.cpp:6723)
// only appends to an in-memory vector, and the canonical schema has no outfit or
// mount table — only the *current* look columns (looktype, lookaddons,
// lookmount*). Unlocking is datapack territory there. So there is no upstream key
// layout to copy; these scopes are canary-go's own, but keeping them in KV means
// the canonical schema stays untouched.
//
//	player.<guid>.mounts.<mountId>    → bool
//	player.<guid>.outfits.<lookType>  → addons (int)
const (
	scopeMounts  = "mounts"
	scopeOutfits = "outfits"
)

// LoadPlayerMounts reads the unlocked mounts.
func (d *DB) LoadPlayerMounts(ctx context.Context, p *game.Player) error {
	p.Mounts = make(map[uint16]bool)
	if d.KV == nil {
		return nil
	}

	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeMounts)
	keys, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if mountID, ok := parseUint16Key(key); ok {
			p.AddMount(mountID)
		}
	}
	return nil
}

// SavePlayerMounts writes the unlocked mounts, dropping any that were removed.
func (d *DB) SavePlayerMounts(ctx context.Context, p *game.Player) error {
	if d.KV == nil || p.Mounts == nil {
		return nil
	}
	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeMounts)

	existing, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	for _, key := range existing {
		mountID, ok := parseUint16Key(key)
		if !ok || !p.Mounts[mountID] {
			if err := scope.Remove(ctx, key); err != nil {
				return err
			}
		}
	}
	for mountID, unlocked := range p.Mounts {
		if !unlocked {
			continue
		}
		key := strconv.FormatUint(uint64(mountID), 10)
		if err := scope.Set(ctx, key, kv.Bool(true)); err != nil {
			return err
		}
	}
	return nil
}

// LoadPlayerOutfits reads the unlocked outfits and their addon bitmask.
func (d *DB) LoadPlayerOutfits(ctx context.Context, p *game.Player) error {
	p.Outfits = []game.OutfitEntry{}
	if d.KV == nil {
		return nil
	}

	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeOutfits)
	keys, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		lookType, ok := parseUint16Key(key)
		if !ok {
			continue
		}
		value, found, err := scope.Get(ctx, key)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		p.Outfits = append(p.Outfits, game.OutfitEntry{
			LookType: lookType,
			Addons:   uint8(value.GetInt()),
		})
	}
	return nil
}

// SavePlayerOutfits writes the unlocked outfits, dropping any that were removed.
func (d *DB) SavePlayerOutfits(ctx context.Context, p *game.Player) error {
	if d.KV == nil || p.Outfits == nil {
		return nil
	}
	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeOutfits)

	current := make(map[uint16]uint8, len(p.Outfits))
	for _, outfit := range p.Outfits {
		current[outfit.LookType] = outfit.Addons
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
		if _, still := current[lookType]; !still {
			if err := scope.Remove(ctx, key); err != nil {
				return err
			}
		}
	}
	for lookType, addons := range current {
		key := strconv.FormatUint(uint64(lookType), 10)
		if err := scope.Set(ctx, key, kv.Int(int32(addons))); err != nil {
			return err
		}
	}
	return nil
}
