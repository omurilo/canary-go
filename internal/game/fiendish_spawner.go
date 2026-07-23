package game

import (
	"math/rand"
	"sync"
	"time"
)

type FiendishManager struct {
	mu               sync.RWMutex
	fiendishMonsters map[uint32]*Monster
	limit            int
}

func NewFiendishManager(limit int) *FiendishManager {
	if limit <= 0 {
		limit = 3
	}
	return &FiendishManager{
		fiendishMonsters: make(map[uint32]*Monster),
		limit:            limit,
	}
}

// MakeFiendishMonster selects a random eligible monster from the world and promotes it to Fiendish state.
func (fm *FiendishManager) MakeFiendishMonster(world *World) *Monster {
	if world == nil {
		return nil
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Clean up dead/removed fiendish monsters
	for id, m := range fm.fiendishMonsters {
		if m == nil || m.Health == 0 {
			delete(fm.fiendishMonsters, id)
		}
	}

	if len(fm.fiendishMonsters) >= fm.limit {
		return nil
	}

	// Gather eligible map monsters
	world.mu.RLock()
	candidates := make([]*Monster, 0)
	for _, c := range world.creatures {
		if m, ok := c.(*Monster); ok && m != nil && m.Health > 0 && m.CanBeForgeMonster() {
			candidates = append(candidates, m)
		}
	}
	world.mu.RUnlock()

	if len(candidates) == 0 {
		return nil
	}

	// Pick random candidate
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	selected := candidates[r.Intn(len(candidates))]

	selected.ForgeClassification = ForgeClassifications_Fiendish
	selected.ConfigureForgeSystem(15)
	selected.TimeToChangeFiendish = time.Now().Add(1 * time.Hour).Unix()

	fm.fiendishMonsters[selected.ID] = selected
	return selected
}

// GetFiendishMonsters returns all currently active fiendish monsters.
func (fm *FiendishManager) GetFiendishMonsters() []*Monster {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	list := make([]*Monster, 0, len(fm.fiendishMonsters))
	for _, m := range fm.fiendishMonsters {
		if m != nil && m.Health > 0 {
			list = append(list, m)
		}
	}
	return list
}

// FindFiendishMonster returns a single active fiendish monster or nil if none.
func (fm *FiendishManager) FindFiendishMonster() *Monster {
	monsters := fm.GetFiendishMonsters()
	if len(monsters) == 0 {
		return nil
	}
	return monsters[0]
}
