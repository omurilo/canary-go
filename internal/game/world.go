package game

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/opentibiabr/canary-go/internal/creatures"
)

// World is the authoritative in-memory game state: the map plus all online
// players. It is safe for concurrent use.
type World struct {
	mu             sync.RWMutex
	Map            *Map
	Towns          map[string]Position
	DefaultSpawn   Position
	players        map[uint32]*Player
	byName         map[string]*Player
	creatures      map[uint32]Creature
	nextCreatureID atomic.Uint32

	OnCreatureMove   func(c Creature, oldPos Position, newPos Position, oldTileIndex int)
	OnCreatureAppear func(c Creature)
	OnCreatureRemove func(c Creature)

	TypeRegistry *creatures.TypeRegistry
}

// NewWorld creates an empty world with a fresh map.
func NewWorld() *World {
	w := &World{
		Map:       NewMap(),
		Towns:     make(map[string]Position),
		players:   make(map[uint32]*Player),
		byName:    make(map[string]*Player),
		creatures: make(map[uint32]Creature),
	}
	w.nextCreatureID.Store(0x10000000) // player creature ids start high, like TFS
	return w
}

// TownTemple returns a town's temple position by (case-insensitive) name.
func (w *World) TownTemple(name string) (Position, bool) {
	p, ok := w.Towns[strings.ToLower(strings.TrimSpace(name))]
	return p, ok
}

// SetPosition moves a player to an absolute position under the world lock,
// correctly updating the tile creature tracking.
func (w *World) SetPosition(p *Player, pos Position) {
	w.mu.Lock()
	w.removeCreatureFromTile(p)
	p.Pos = pos
	w.addCreatureToTile(p)
	w.mu.Unlock()
}

// Players returns a snapshot of all online players.
func (w *World) Players() []*Player {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]*Player, 0, len(w.players))
	for _, p := range w.players {
		out = append(out, p)
	}
	return out
}

// Creatures returns a snapshot of all non-player creatures.
func (w *World) Creatures() []Creature {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]Creature, 0, len(w.creatures))
	for _, c := range w.creatures {
		out = append(out, c)
	}
	return out
}

// RLock acquires the world's read lock.
func (w *World) RLock() {
	w.mu.RLock()
}

// RUnlock releases the world's read lock.
func (w *World) RUnlock() {
	w.mu.RUnlock()
}

// AddPlayer registers a player, assigns a creature id, applies defaults and
// places it on the map. Returns false if a character with the same name is
// already online.
func (w *World) AddPlayer(p *Player, sess Session) bool {
	w.mu.Lock()
	key := strings.ToLower(p.Name)
	if _, online := w.byName[key]; online {
		w.mu.Unlock()
		return false
	}
	p.ID = w.nextCreatureID.Add(1)
	p.Session = sess
	p.ensureDefaults()
	w.players[p.ID] = p
	w.byName[key] = p
	w.addCreatureToTile(p)
	w.mu.Unlock()
	if w.OnCreatureAppear != nil {
		w.OnCreatureAppear(p)
	}
	return true
}

// RemovePlayer unregisters a player by creature id.
func (w *World) RemovePlayer(id uint32) {
	w.mu.Lock()
	if p, ok := w.players[id]; ok {
		delete(w.players, id)
		delete(w.byName, strings.ToLower(p.Name))
		w.removeCreatureFromTile(p)
		w.mu.Unlock()
		if w.OnCreatureRemove != nil {
			w.OnCreatureRemove(p)
		}
		return
	}
	w.mu.Unlock()
}

// PlayerByID returns an online player or nil.
func (w *World) PlayerByID(id uint32) *Player {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.players[id]
}

// CreatureByID returns a creature or nil.
func (w *World) CreatureByID(id uint32) Creature {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if c, ok := w.creatures[id]; ok {
		return c
	}
	if p, ok := w.players[id]; ok {
		return p
	}
	return nil
}

// AddCreature adds a non-player creature to the world.
func (w *World) AddCreature(c Creature) {
	w.mu.Lock()
	w.creatures[c.GetID()] = c
	w.addCreatureToTile(c)
	w.mu.Unlock()
	if w.OnCreatureAppear != nil {
		w.OnCreatureAppear(c)
	}
}



func (w *World) addCreatureToTile(c Creature) {
	t := w.Map.GetTile(c.GetPosition())
	if t != nil {
		t.Creatures = append(t.Creatures, c)
	}
}

func (w *World) removeCreatureFromTile(c Creature) int {
	t := w.Map.GetTile(c.GetPosition())
	if t != nil {
		for i, v := range t.Creatures {
			if v.GetID() == c.GetID() {
				t.Creatures = append(t.Creatures[:i], t.Creatures[i+1:]...)
				return i
			}
		}
	}
	return -1
}

// RemoveCreature removes a non-player creature from the world.
func (w *World) RemoveCreature(id uint32) {
	w.mu.Lock()
	c, exists := w.creatures[id]
	if exists {
		delete(w.creatures, id)
		w.removeCreatureFromTile(c)
	}
	w.mu.Unlock()
	if exists && w.OnCreatureRemove != nil {
		w.OnCreatureRemove(c)
	}
}

// OnlineCount returns the number of connected players.
func (w *World) OnlineCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.players)
}

// Spectators returns players (optionally excluding one id) whose client can see
// pos.
func (w *World) Spectators(pos Position, excludeID uint32) []*Player {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var out []*Player
	for id, p := range w.players {
		if id == excludeID {
			continue
		}
		if p.Pos.InRangeOf(pos) {
			out = append(out, p)
		}
	}
	return out
}

// CreaturesInView returns all creatures (players, monsters, NPCs) within range of pos.
func (w *World) CreaturesInView(pos Position) []Creature {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var out []Creature
	for _, p := range w.players {
		if p.Pos.InRangeOf(pos) {
			out = append(out, p)
		}
	}
	for _, c := range w.creatures {
		if c.GetPosition().InRangeOf(pos) {
			out = append(out, c)
		}
	}
	return out
}

// TryMove validates and applies a directional step for a player, returning the new position
// and whether the move succeeded.
func (w *World) TryMove(p *Player, dir Direction) (Position, bool) {
	dest := p.Pos.Offset(dir)
	if !w.Map.GetTile(dest).Walkable() {
		return p.Pos, false
	}
	w.mu.Lock()
	oldPos := p.Pos
	oldTileIndex := w.removeCreatureFromTile(p)
	p.Pos = dest
	p.Direction = dir
	w.addCreatureToTile(p)
	w.mu.Unlock()

	if w.OnCreatureMove != nil {
		w.OnCreatureMove(p, oldPos, dest, oldTileIndex)
	}

	return dest, true
}

// TryMoveCreature validates and applies a directional step for any creature, returning the new position
// and whether the move succeeded.
func (w *World) TryMoveCreature(c Creature, dir Direction) (Position, bool) {
	dest := c.GetPosition().Offset(dir)
	if !w.Map.GetTile(dest).Walkable() {
		return c.GetPosition(), false
	}
	w.mu.Lock()
	oldPos := c.GetPosition()
	oldTileIndex := w.removeCreatureFromTile(c)
	c.SetPosition(dest)
	c.SetDirection(dir)
	w.addCreatureToTile(c)
	w.mu.Unlock()
	
	if w.OnCreatureMove != nil {
		w.OnCreatureMove(c, oldPos, dest, oldTileIndex)
	}
	
	return dest, true
}
