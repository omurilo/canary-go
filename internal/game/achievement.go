package game

import (
	"sort"
	"sync"
	"time"
)

// Achievement represents a single achievement definition. Achievements are
// registered by Lua scripts via Game.registerAchievement and stored in a
// global registry. Mirrors C++ src/creatures/players/components/player_achievement.cpp.
type Achievement struct {
	ID          uint16
	Name        string
	Description string
	Secret      bool
	Points      uint8
}

// AchievementRegistry holds all known achievements. Populated by Lua scripts
// calling Game.registerAchievement; queries come from the player API and the
// cyclopedia protocol packets.
type AchievementRegistry struct {
	mu     sync.RWMutex
	byID   map[uint16]*Achievement
	byName map[string]*Achievement
	nextID uint16
}

// NewAchievementRegistry creates an empty registry.
func NewAchievementRegistry() *AchievementRegistry {
	return &AchievementRegistry{
		byID:   make(map[uint16]*Achievement),
		byName: make(map[string]*Achievement),
		nextID: 1,
	}
}

// Register adds an achievement. Returns the auto-assigned ID.
func (r *AchievementRegistry) Register(name, description string, secret bool, points uint8) uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Deduplicate by name
	if existing, ok := r.byName[name]; ok {
		return existing.ID
	}

	id := r.nextID
	r.nextID++
	a := &Achievement{
		ID:          id,
		Name:        name,
		Description: description,
		Secret:      secret,
		Points:      points,
	}
	r.byID[id] = a
	r.byName[name] = a
	return id
}

// GetByID returns the achievement with the given ID, or nil.
func (r *AchievementRegistry) GetByID(id uint16) *Achievement {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byID[id]
}

// GetByName returns the achievement with the given name, or nil.
func (r *AchievementRegistry) GetByName(name string) *Achievement {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byName[name]
}

// PublicAchievements returns all non-secret achievements sorted by ID.
func (r *AchievementRegistry) PublicAchievements() []*Achievement {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*Achievement
	for _, a := range r.byID {
		if !a.Secret {
			list = append(list, a)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// SecretAchievements returns all secret achievements sorted by ID.
func (r *AchievementRegistry) SecretAchievements() []*Achievement {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*Achievement
	for _, a := range r.byID {
		if a.Secret {
			list = append(list, a)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// AllAchievements returns every achievement sorted by ID.
func (r *AchievementRegistry) AllAchievements() []*Achievement {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*Achievement, 0, len(r.byID))
	for _, a := range r.byID {
		list = append(list, a)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// Count returns the total number of registered achievements.
func (r *AchievementRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

// PlayerAchievement tracks a single unlocked achievement with its unlock time.
type PlayerAchievement struct {
	Achievement *Achievement
	UnlockedAt  time.Time
}

// --- Player achievement methods ---

// AddAchievement unlocks an achievement for the player. Returns true if newly
// unlocked (was not already unlocked). Mirrors PlayerAchievement::add.
func (p *Player) AddAchievement(reg *AchievementRegistry, id uint16, ts int64) bool {
	if p.Achievements == nil {
		p.Achievements = make(map[uint16]int64)
	}

	if _, ok := p.Achievements[id]; ok {
		return false // already unlocked
	}

	if reg != nil && reg.GetByID(id) == nil {
		return false // unknown achievement
	}

	if ts == 0 {
		ts = time.Now().Unix()
	}
	p.Achievements[id] = ts
	return true
}

// AddAchievementByName unlocks an achievement by name. Returns true if newly
// unlocked. Mirrors PlayerAchievement::add(name).
func (p *Player) AddAchievementByName(reg *AchievementRegistry, name string) bool {
	if reg == nil {
		return false
	}
	a := reg.GetByName(name)
	if a == nil {
		return false
	}
	return p.AddAchievement(reg, a.ID, 0)
}

// HasAchievement returns true if the player has unlocked the given achievement.
func (p *Player) HasAchievement(id uint16) bool {
	if p.Achievements == nil {
		return false
	}
	_, ok := p.Achievements[id]
	return ok
}

// HasAchievementByName returns true if the player has unlocked the named achievement.
func (p *Player) HasAchievementByName(reg *AchievementRegistry, name string) bool {
	if reg == nil {
		return false
	}
	a := reg.GetByName(name)
	if a == nil {
		return false
	}
	return p.HasAchievement(a.ID)
}

// AchievementPoints returns the total achievement points earned.
func (p *Player) AchievementPoints(reg *AchievementRegistry) uint32 {
	if reg == nil || p.Achievements == nil {
		return 0
	}
	var total uint32
	for id := range p.Achievements {
		if a := reg.GetByID(id); a != nil {
			total += uint32(a.Points)
		}
	}
	return total
}

// AchievementCount returns the number of unlocked achievements.
func (p *Player) AchievementCount() int {
	if p.Achievements == nil {
		return 0
	}
	return len(p.Achievements)
}

// UnlockedAchievements returns the player's unlocked achievements with timestamps,
// sorted by unlock time. Mirrors PlayerAchievement::getUnlockedAchievements.
func (p *Player) UnlockedAchievements(reg *AchievementRegistry) []PlayerAchievement {
	if reg == nil || p.Achievements == nil {
		return nil
	}
	var list []PlayerAchievement
	for id, ts := range p.Achievements {
		if a := reg.GetByID(id); a != nil {
			list = append(list, PlayerAchievement{
				Achievement: a,
				UnlockedAt:  time.Unix(ts, 0),
			})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UnlockedAt.Before(list[j].UnlockedAt)
	})
	return list
}

// SecretCount returns the number of secret achievements the player has unlocked.
func (p *Player) SecretCount(reg *AchievementRegistry) int {
	if reg == nil || p.Achievements == nil {
		return 0
	}
	count := 0
	for id := range p.Achievements {
		if a := reg.GetByID(id); a != nil && a.Secret {
			count++
		}
	}
	return count
}
