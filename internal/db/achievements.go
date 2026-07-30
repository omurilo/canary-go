package db

import (
	"context"
	"log/slog"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/kv"
)

// Achievements and titles live in the KV store, not in dedicated tables. The key
// layout is taken from PlayerAchievement / PlayerTitle:
//
//	player.<guid>.achievements.points            → int
//	player.<guid>.achievements.unlocked.<name>   → unlock timestamp
//	player.<guid>.titles.current-title           → title id
//	player.<guid>.titles.unlocked.<name>         → unlock timestamp
//
// See player_achievement.cpp:81,87,136 and player_title.cpp:56,89,94,176.
// The previous Go implementation invented `player_achievements` and
// `player_titles` tables created by runtime DDL that was never called, so
// nothing persisted at all.
//
// C++ keys these by achievement/title NAME rather than id, so the registry is
// needed to translate. Timestamps go through IntType, which is int32 on both
// sides, so both servers share the same year-2038 ceiling.
const (
	scopeAchievements = "achievements"
	scopeTitles       = "titles"
	scopeUnlocked     = "unlocked"

	keyPoints = "points"
)

// LoadPlayerAchievements reads the unlocked achievements, porting
// PlayerAchievement::loadUnlockedAchievements (player_achievement.cpp:105).
func (d *DB) LoadPlayerAchievements(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	p.Achievements = make(map[uint16]int64)

	registry := achievementRegistry(p)
	if registry == nil {
		// Without the registry the stored names cannot be resolved back to ids.
		// Leave the map empty rather than guessing.
		return nil
	}

	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeAchievements).Scoped(scopeUnlocked)
	names, err := scope.Keys(ctx)
	if err != nil {
		return err
	}

	for _, name := range names {
		achievement := registry.GetByName(name)
		if achievement == nil {
			slog.Default().Warn("unknown achievement in KV; skipping",
				"name", name, "player", p.Name)
			continue
		}
		value, found, err := scope.Get(ctx, name)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		p.Achievements[achievement.ID] = int64(value.GetInt())
	}
	return nil
}

// SavePlayerAchievements writes the unlocked achievements and the points total.
func (d *DB) SavePlayerAchievements(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	registry := achievementRegistry(p)
	if registry == nil {
		return nil
	}

	base := d.KV.PlayerScope(p.DBID).Scoped(scopeAchievements)
	scope := base.Scoped(scopeUnlocked)

	var points int32
	for id, ts := range p.Achievements {
		achievement := registry.GetByID(id)
		if achievement == nil {
			slog.Default().Warn("unknown achievement id; not persisted",
				"id", id, "player", p.Name)
			continue
		}
		if err := scope.Set(ctx, achievement.Name, kv.Int(int32(ts))); err != nil {
			return err
		}
		points += int32(achievement.Points)
	}

	return base.Set(ctx, keyPoints, kv.Int(points))
}

// LoadPlayerTitles reads the unlocked titles, porting
// PlayerTitle::loadUnlockedTitles (player_title.cpp:170).
func (d *DB) LoadPlayerTitles(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	p.TitleStrings = nil

	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeTitles).Scoped(scopeUnlocked)
	names, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	p.TitleStrings = append(p.TitleStrings, names...)
	return nil
}

// SavePlayerTitles writes the unlocked titles, dropping any that were revoked and
// preserving the original unlock timestamp of the ones that remain.
func (d *DB) SavePlayerTitles(ctx context.Context, p *game.Player) error {
	if d.KV == nil {
		return nil
	}
	scope := d.KV.PlayerScope(p.DBID).Scoped(scopeTitles).Scoped(scopeUnlocked)

	existing, err := scope.Keys(ctx)
	if err != nil {
		return err
	}
	current := make(map[string]struct{}, len(p.TitleStrings))
	for _, title := range p.TitleStrings {
		current[title] = struct{}{}
	}

	for _, name := range existing {
		if _, ok := current[name]; !ok {
			if err := scope.Remove(ctx, name); err != nil {
				return err
			}
		}
	}

	now := int32(time.Now().Unix())
	for title := range current {
		_, found, err := scope.Get(ctx, title)
		if err != nil {
			return err
		}
		if found {
			continue
		}
		if err := scope.Set(ctx, title, kv.Int(now)); err != nil {
			return err
		}
	}
	return nil
}

// achievementRegistry reaches the registry through the player's world.
func achievementRegistry(p *game.Player) *game.AchievementRegistry {
	if p == nil || p.World == nil {
		return nil
	}
	return p.World.Achievements
}
