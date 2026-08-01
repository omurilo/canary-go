package combat

import (
	"sync"
	"time"
)

// SpellCooldown represents the cooldown info for spells
type SpellCooldown struct {
	Id         uint16
	Expiration int64
}

// CooldownManager manages spell cooldowns for a creature
type CooldownManager struct {
	mu          sync.RWMutex
	Spells      map[uint16]int64
	GroupSpells map[uint32]int64
	GlobalCD    int64
}

// NewCooldownManager creates a new CooldownManager
func NewCooldownManager() *CooldownManager {
	return &CooldownManager{
		Spells:      make(map[uint16]int64),
		GroupSpells: make(map[uint32]int64),
	}
}

// GetTimeNow returns the current time in milliseconds
func GetTimeNow() int64 {
	return time.Now().UnixMilli()
}

// AddCooldown adds a spell cooldown
func (c *CooldownManager) AddCooldown(spellId uint16, cooldownMs uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Spells[spellId] = GetTimeNow() + int64(cooldownMs)
}

// AddGroupCooldown adds a group cooldown
func (c *CooldownManager) AddGroupCooldown(groupId uint32, cooldownMs uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.GroupSpells[groupId] = GetTimeNow() + int64(cooldownMs)
}

// HasCooldown checks if the spell is on cooldown
func (c *CooldownManager) HasCooldown(spellId uint16) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if expiration, ok := c.Spells[spellId]; ok {
		return GetTimeNow() < expiration
	}
	return false
}

// HasGroupCooldown checks if the group is on cooldown
func (c *CooldownManager) HasGroupCooldown(groupId uint32) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if expiration, ok := c.GroupSpells[groupId]; ok {
		return GetTimeNow() < expiration
	}
	return false
}

// CleanExpired removes expired cooldowns to free memory
func (c *CooldownManager) CleanExpired() {
	now := GetTimeNow()
	c.mu.Lock()
	defer c.mu.Unlock()

	for id, expiration := range c.Spells {
		if now >= expiration {
			delete(c.Spells, id)
		}
	}

	for id, expiration := range c.GroupSpells {
		if now >= expiration {
			delete(c.GroupSpells, id)
		}
	}
}
