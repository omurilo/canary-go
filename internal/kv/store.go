package kv

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Store is the SQL-backed key/value store, porting KVSQL (src/kv/kv_sql.cpp)
// over the `kv_store` table (key_name / timestamp / value).
//
// Unlike C++ this has no in-memory write-back cache with eviction; every Get and
// Set hits the database. The C++ cache exists to batch a player's writes on
// logout, which is a performance concern rather than a behavioural one — the
// stored bytes and key layout are identical either way.
type Store struct {
	db *sql.DB
}

// KV is a handle onto the store, either the root or a scope of it.
type KV interface {
	Get(ctx context.Context, key string) (Value, bool, error)
	Set(ctx context.Context, key string, v Value) error
	Remove(ctx context.Context, key string) error
	// Keys lists the keys under this handle, with the scope prefix stripped.
	Keys(ctx context.Context) ([]string, error)
	Scoped(scope string) KV
}

// New returns the root store.
func New(db *sql.DB) *Store { return &Store{db: db} }

// buildKey joins a scope prefix and a key with '.', matching
// ScopedKV::buildKey (src/kv/kv.hpp:179).
func buildKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// scopedKV is a prefixed view of the root store, porting ScopedKV.
type scopedKV struct {
	root   *Store
	prefix string
}

// Scoped returns a handle whose keys are prefixed with scope.
func (s *Store) Scoped(scope string) KV {
	return &scopedKV{root: s, prefix: scope}
}

func (k *scopedKV) Scoped(scope string) KV {
	return &scopedKV{root: k.root, prefix: buildKey(k.prefix, scope)}
}

func (k *scopedKV) Get(ctx context.Context, key string) (Value, bool, error) {
	return k.root.Get(ctx, buildKey(k.prefix, key))
}

func (k *scopedKV) Set(ctx context.Context, key string, v Value) error {
	return k.root.Set(ctx, buildKey(k.prefix, key), v)
}

func (k *scopedKV) Remove(ctx context.Context, key string) error {
	return k.root.Remove(ctx, buildKey(k.prefix, key))
}

// Keys lists keys under the scope, stripping the prefix — the same contract as
// KVSQL::loadPrefix, which does replaceString(key, prefix, "").
func (k *scopedKV) Keys(ctx context.Context) ([]string, error) {
	return k.root.keysWithPrefix(ctx, k.prefix+".")
}

// Get loads one key. The second result reports presence.
func (s *Store) Get(ctx context.Context, key string) (Value, bool, error) {
	var (
		timestamp uint64
		raw       []byte
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT `timestamp`, `value` FROM `kv_store` WHERE `key_name` = ?", key,
	).Scan(&timestamp, &raw)
	if err == sql.ErrNoRows {
		return Value{}, false, nil
	}
	if err != nil {
		return Value{}, false, fmt.Errorf("kv: get %s: %w", key, err)
	}

	v, err := Unmarshal(raw, timestamp)
	if err != nil {
		return Value{}, false, fmt.Errorf("kv: decode %s: %w", key, err)
	}
	return v, true, nil
}

// Set upserts one key. A value marked Deleted removes the row instead, mirroring
// KVSQL::prepareSave.
func (s *Store) Set(ctx context.Context, key string, v Value) error {
	if v.Deleted {
		return s.Remove(ctx, key)
	}

	ts := v.Timestamp
	if ts == 0 {
		ts = uint64(time.Now().UnixMilli())
	}

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO `kv_store` (`key_name`, `timestamp`, `value`) VALUES (?, ?, ?) "+
			"ON DUPLICATE KEY UPDATE `timestamp` = VALUES(`timestamp`), `value` = VALUES(`value`)",
		key, ts, v.Marshal())
	if err != nil {
		return fmt.Errorf("kv: set %s: %w", key, err)
	}
	return nil
}

// Remove deletes one key.
func (s *Store) Remove(ctx context.Context, key string) error {
	if _, err := s.db.ExecContext(ctx,
		"DELETE FROM `kv_store` WHERE `key_name` = ?", key); err != nil {
		return fmt.Errorf("kv: remove %s: %w", key, err)
	}
	return nil
}

// Keys lists every key in the store.
func (s *Store) Keys(ctx context.Context) ([]string, error) {
	return s.keysWithPrefix(ctx, "")
}

// keysWithPrefix runs the LIKE query from KVSQL::loadPrefix and strips the prefix.
func (s *Store) keysWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT `key_name` FROM `kv_store` WHERE `key_name` LIKE ?", escapeLike(prefix)+"%")
	if err != nil {
		return nil, fmt.Errorf("kv: keys %s: %w", prefix, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("kv: keys scan: %w", err)
		}
		out = append(out, strings.TrimPrefix(key, prefix))
	}
	return out, rows.Err()
}

// escapeLike neutralizes LIKE wildcards in a prefix. C++ interpolates the prefix
// straight into the LIKE pattern, so a key containing '%' or '_' would match too
// much there; escaping is a strict improvement and cannot change results for the
// dotted keys the server actually uses.
func escapeLike(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '%', '_', '\\':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// PlayerScope returns the scope C++ uses for per-player data:
// g_kv().scoped("player")->scoped(<guid>) — Player::kv() (player.cpp:1376).
func (s *Store) PlayerScope(guid uint32) KV {
	return s.Scoped("player").Scoped(fmt.Sprintf("%d", guid))
}
