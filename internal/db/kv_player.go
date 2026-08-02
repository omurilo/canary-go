package db

import (
	"context"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/kv"
)

// The Lua kv binding (player:kv() → LuaKVStore) writes only to the player's
// in-memory p.KVStore map. Without a DB round-trip the hireling skills and
// outfits (and any other player-scoped kv the datapack stores) were lost on
// relog. These two functions mirror KVSQL::loadPrefix / prepareSave against the
// same `kv_store` table the subsystem loads use, so the Lua kv and the Go
// subsystems share one source of truth.

// LoadPlayerKV reads the player's kv_store entries into p.KVStore.
func (d *DB) LoadPlayerKV(ctx context.Context, p *game.Player) error {
	if p == nil || d.KV == nil {
		return nil
	}
	scope := d.KV.PlayerScope(p.DBID)
	keys, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	if p.KVStore == nil {
		p.KVStore = make(map[string]any)
	}
	for _, key := range keys {
		v, ok, err := scope.Get(ctx, key)
		if err != nil || !ok {
			continue
		}
		p.KVStore[key] = kvValueToGo(v)
	}
	return nil
}

// SavePlayerKV persists p.KVStore back to kv_store.
func (d *DB) SavePlayerKV(ctx context.Context, p *game.Player) error {
	if p == nil || d.KV == nil {
		return nil
	}
	scope := d.KV.PlayerScope(p.DBID)
	for key, val := range p.KVStore {
		if err := scope.Set(ctx, key, goValueToKV(val)); err != nil {
			return err
		}
	}
	return nil
}

func kvValueToGo(v kv.Value) any {
	switch v.Kind {
	case kv.KindString:
		return v.Str
	case kv.KindBool:
		return v.Bool
	case kv.KindInt:
		return float64(v.Int)
	default: // KindDouble and anything unexpected read back as a number.
		return v.Double
	}
}

func goValueToKV(v any) kv.Value {
	switch t := v.(type) {
	case string:
		return kv.String(t)
	case bool:
		return kv.Bool(t)
	case float64:
		return kv.Double(t)
	case int:
		return kv.Double(float64(t))
	case int32:
		return kv.Int(t)
	case int64:
		return kv.Double(float64(t))
	case uint32:
		return kv.Int(int32(t))
	case uint64:
		return kv.Double(float64(t))
	default:
		return kv.String("")
	}
}
