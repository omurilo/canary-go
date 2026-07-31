package game

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Area is a rectangular cuboid of map positions, the port of struct Area
// (src/game/zones/zone.hpp:30).
type Area struct {
	From Position
	To   Position
}

// Contains reports whether pos falls inside the area, bounds inclusive.
func (a Area) Contains(pos Position) bool {
	return pos.X >= a.From.X && pos.X <= a.To.X &&
		pos.Y >= a.From.Y && pos.Y <= a.To.Y &&
		pos.Z >= a.From.Z && pos.Z <= a.To.Z
}

// Intersects reports whether two areas overlap.
func (a Area) Intersects(b Area) bool {
	return a.From.X <= b.To.X && a.To.X >= b.From.X &&
		a.From.Y <= b.To.Y && a.To.Y >= b.From.Y &&
		a.From.Z <= b.To.Z && a.To.Z >= b.From.Z
}

// Positions enumerates every position in the area. C++ walks it with a
// PositionIterator; the only consumers are addArea/subtractArea, which need the
// whole set anyway.
func (a Area) Positions() []Position {
	if a.From.X > a.To.X || a.From.Y > a.To.Y || a.From.Z > a.To.Z {
		return nil
	}
	n := (int(a.To.X-a.From.X) + 1) * (int(a.To.Y-a.From.Y) + 1) * (int(a.To.Z-a.From.Z) + 1)
	out := make([]Position, 0, n)
	for z := a.From.Z; z <= a.To.Z; z++ {
		for y := a.From.Y; y <= a.To.Y; y++ {
			for x := a.From.X; x <= a.To.X; x++ {
				out = append(out, Position{X: x, Y: y, Z: z})
			}
		}
	}
	return out
}

func (a Area) String() string {
	return fmt.Sprintf("Area(from: %v, to: %v)", a.From, a.To)
}

// Zone is a named set of map positions, the port of class Zone
// (src/game/zones/zone.hpp). Zones come from two places: the map editor writes a
// zone id onto each tile (OTBM_TILE_ZONE) and `<map>-zones.xml` gives those ids
// names, while Lua can create anonymous ones at runtime.
//
// C++ keeps weak caches of the creatures and items inside a zone and refreshes
// them; this resolves membership from the world on demand instead. The caches are a
// performance device upstream, not a semantic one, and computing live removes the
// whole class of staleness bugs that come with invalidating them by hand.
type Zone struct {
	mu                sync.RWMutex
	name              string
	id                uint32
	monsterVariant    string
	removeDestination Position
	positions         map[Position]struct{}

	registry *ZoneRegistry
}

// Name is the zone's name, empty for one the map created but the XML never named.
func (z *Zone) Name() string {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return z.name
}

// ID is the map-editor zone id, or 0 for a zone created from Lua.
func (z *Zone) ID() uint32 {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return z.id
}

// IsStatic reports whether the zone came from the map rather than from a script.
// Zone::isStatic is `id != 0`.
func (z *Zone) IsStatic() bool { return z.ID() != 0 }

// MonsterVariant is the variant name monsters spawned in this zone take on.
func (z *Zone) MonsterVariant() string {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return z.monsterVariant
}

// SetMonsterVariant sets the variant applied to monsters inside the zone.
func (z *Zone) SetMonsterVariant(v string) {
	z.mu.Lock()
	z.monsterVariant = v
	z.mu.Unlock()
}

// SetRemoveDestination sets where RemovePlayers sends people.
func (z *Zone) SetRemoveDestination(pos Position) {
	z.mu.Lock()
	z.removeDestination = pos
	z.mu.Unlock()
}

// RemoveDestination resolves where a creature ejected from the zone should land,
// mirroring Zone::getRemoveDestination: only players are moved, an explicit
// destination wins, and otherwise it is the player's own temple.
func (z *Zone) RemoveDestination(c Creature) Position {
	p, ok := c.(*Player)
	if !ok || p == nil {
		return Position{}
	}
	z.mu.RLock()
	dest := z.removeDestination
	z.mu.RUnlock()
	if dest != (Position{}) {
		return dest
	}
	if z.registry != nil && z.registry.world != nil {
		if temple, ok := z.registry.world.TempleByTownID(p.TownID); ok {
			return temple
		}
	}
	return Position{}
}

// AddPosition adds a single position to the zone and indexes it.
func (z *Zone) AddPosition(pos Position) {
	z.mu.Lock()
	if z.positions == nil {
		z.positions = map[Position]struct{}{}
	}
	_, existed := z.positions[pos]
	z.positions[pos] = struct{}{}
	z.mu.Unlock()
	if !existed && z.registry != nil {
		z.registry.index(pos, z)
	}
}

// RemovePosition drops a position from the zone.
func (z *Zone) RemovePosition(pos Position) {
	z.mu.Lock()
	_, existed := z.positions[pos]
	delete(z.positions, pos)
	z.mu.Unlock()
	if existed && z.registry != nil {
		z.registry.unindex(pos, z)
	}
}

// AddArea adds every position in the area (Zone::addArea).
func (z *Zone) AddArea(a Area) {
	for _, pos := range a.Positions() {
		z.AddPosition(pos)
	}
}

// SubtractArea removes every position in the area (Zone::subtractArea).
func (z *Zone) SubtractArea(a Area) {
	for _, pos := range a.Positions() {
		z.RemovePosition(pos)
	}
}

// Contains reports whether the position belongs to the zone.
func (z *Zone) Contains(pos Position) bool {
	z.mu.RLock()
	defer z.mu.RUnlock()
	_, ok := z.positions[pos]
	return ok
}

// Positions returns the zone's positions in a stable order. The underlying store
// is a set, so it is sorted rather than left to map iteration — scripts index into
// this and a shuffling order would make them non-deterministic.
func (z *Zone) Positions() []Position {
	z.mu.RLock()
	out := make([]Position, 0, len(z.positions))
	for pos := range z.positions {
		out = append(out, pos)
	}
	z.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Z != out[j].Z {
			return out[i].Z < out[j].Z
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

// Size is the number of positions in the zone.
func (z *Zone) Size() int {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return len(z.positions)
}

// Refresh re-indexes the zone's positions. In C++ it also clears the member
// caches; there are none here, so membership is already current.
func (z *Zone) Refresh() {
	if z.registry == nil {
		return
	}
	for _, pos := range z.Positions() {
		z.registry.index(pos, z)
	}
}

// Creatures returns every creature standing in the zone. World.Creatures() holds
// only the non-player creatures, so players are folded in separately — C++
// getCreatures() covers both.
func (z *Zone) Creatures() []Creature {
	if z.registry == nil || z.registry.world == nil {
		return nil
	}
	var out []Creature
	for _, c := range z.registry.world.Creatures() {
		if z.Contains(c.GetPosition()) {
			out = append(out, c)
		}
	}
	for _, p := range z.registry.world.Players() {
		if z.Contains(p.GetPosition()) {
			out = append(out, p)
		}
	}
	return out
}

// Players returns every player standing in the zone.
func (z *Zone) Players() []*Player {
	if z.registry == nil || z.registry.world == nil {
		return nil
	}
	var out []*Player
	for _, p := range z.registry.world.Players() {
		if z.Contains(p.GetPosition()) {
			out = append(out, p)
		}
	}
	return out
}

// Monsters returns every monster standing in the zone.
func (z *Zone) Monsters() []*Monster {
	var out []*Monster
	for _, c := range z.Creatures() {
		if m, ok := c.(*Monster); ok {
			out = append(out, m)
		}
	}
	return out
}

// Npcs returns every NPC standing in the zone.
func (z *Zone) Npcs() []*Npc {
	var out []*Npc
	for _, c := range z.Creatures() {
		if n, ok := c.(*Npc); ok {
			out = append(out, n)
		}
	}
	return out
}

// Items returns every item lying on the zone's tiles.
func (z *Zone) Items() []*Item {
	if z.registry == nil || z.registry.world == nil {
		return nil
	}
	var out []*Item
	for _, pos := range z.Positions() {
		if tile := z.registry.world.Map.GetTile(pos); tile != nil {
			out = append(out, tile.Items...)
		}
	}
	return out
}

// RemovePlayers teleports every player in the zone to its remove destination
// (Zone::removePlayers).
func (z *Zone) RemovePlayers() {
	if z.registry == nil || z.registry.world == nil {
		return
	}
	for _, p := range z.Players() {
		dest := z.RemoveDestination(p)
		if dest == (Position{}) {
			continue
		}
		z.registry.world.TeleportCreature(p, dest)
	}
}

// RemoveMonsters deletes every monster in the zone (Zone::removeMonsters).
func (z *Zone) RemoveMonsters() {
	if z.registry == nil || z.registry.world == nil {
		return
	}
	for _, m := range z.Monsters() {
		z.registry.world.RemoveCreature(m.GetID())
	}
}

// RemoveNpcs deletes every NPC in the zone (Zone::removeNpcs).
func (z *Zone) RemoveNpcs() {
	if z.registry == nil || z.registry.world == nil {
		return
	}
	for _, n := range z.Npcs() {
		z.registry.world.RemoveCreature(n.GetID())
	}
}

// ZoneRegistry holds every zone, indexed the three ways C++ indexes them: by name,
// by map-editor id, and by position.
type ZoneRegistry struct {
	mu     sync.RWMutex
	world  *World
	byName map[string]*Zone
	byID   map[uint32]*Zone
	byPos  map[Position][]*Zone
}

// NewZoneRegistry creates an empty registry bound to a world.
func NewZoneRegistry(w *World) *ZoneRegistry {
	return &ZoneRegistry{
		world:  w,
		byName: map[string]*Zone{},
		byID:   map[uint32]*Zone{},
		byPos:  map[Position][]*Zone{},
	}
}

// Add registers a zone, mirroring Zone::addZone including its linking rule: when a
// zone with this id already exists — which is the normal case, because the OTBM
// tiles are parsed before the XML names them — the existing one is renamed rather
// than duplicated. "default" is reserved.
func (r *ZoneRegistry) Add(name string, id uint32) (*Zone, error) {
	if strings.EqualFold(name, "default") {
		return nil, fmt.Errorf("zone name %q is reserved", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if id != 0 {
		if existing, ok := r.byID[id]; ok {
			existing.mu.Lock()
			existing.name = name
			existing.mu.Unlock()
			r.byName[name] = existing
			return existing, nil
		}
	}
	if _, ok := r.byName[name]; ok {
		return nil, fmt.Errorf("zone %q already exists", name)
	}
	z := &Zone{name: name, id: id, positions: map[Position]struct{}{}, registry: r}
	r.byName[name] = z
	if id != 0 {
		r.byID[id] = z
	}
	return z, nil
}

// ByName returns a zone by name, or nil.
func (r *ZoneRegistry) ByName(name string) *Zone {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byName[name]
}

// ByID returns the zone with this map-editor id, creating an unnamed one if it does
// not exist yet. That auto-creation is what lets the OTBM be parsed before the XML:
// tiles claim ids, and Add later attaches the names (Zone::getZone(uint32_t)).
// Id 0 has no zone.
func (r *ZoneRegistry) ByID(id uint32) *Zone {
	if id == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if z, ok := r.byID[id]; ok {
		return z
	}
	z := &Zone{id: id, positions: map[Position]struct{}{}, registry: r}
	r.byID[id] = z
	return z
}

// At returns every zone containing the position (Zone::getZones(Position)).
func (r *ZoneRegistry) At(pos Position) []*Zone {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.byPos[pos]) == 0 {
		return nil
	}
	return append([]*Zone(nil), r.byPos[pos]...)
}

// All returns every registered zone, sorted by id then name so callers get a
// stable order.
func (r *ZoneRegistry) All() []*Zone {
	r.mu.RLock()
	seen := map[*Zone]struct{}{}
	var out []*Zone
	for _, z := range r.byID {
		if _, ok := seen[z]; !ok {
			seen[z] = struct{}{}
			out = append(out, z)
		}
	}
	for _, z := range r.byName {
		if _, ok := seen[z]; !ok {
			seen[z] = struct{}{}
			out = append(out, z)
		}
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID() != out[j].ID() {
			return out[i].ID() < out[j].ID()
		}
		return out[i].Name() < out[j].Name()
	})
	return out
}

// Count is the number of distinct zones.
func (r *ZoneRegistry) Count() int { return len(r.All()) }

// RefreshAll re-indexes every zone (Zone::refreshAll).
func (r *ZoneRegistry) RefreshAll() {
	for _, z := range r.All() {
		z.Refresh()
	}
}

// Clear drops every zone (Zone::clearZones).
func (r *ZoneRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byName = map[string]*Zone{}
	r.byID = map[uint32]*Zone{}
	r.byPos = map[Position][]*Zone{}
}

func (r *ZoneRegistry) index(pos Position, z *Zone) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.byPos[pos] {
		if existing == z {
			return
		}
	}
	r.byPos[pos] = append(r.byPos[pos], z)
}

func (r *ZoneRegistry) unindex(pos Position, z *Zone) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.byPos[pos]
	for i, existing := range list {
		if existing == z {
			r.byPos[pos] = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(r.byPos[pos]) == 0 {
		delete(r.byPos, pos)
	}
}
