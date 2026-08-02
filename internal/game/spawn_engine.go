package game

import (
	"log/slog"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/omurilo/canary-go/internal/creatures"
	"github.com/omurilo/canary-go/internal/game/spawns"
)

// ---- C++ spawnBlock_t equivalent ----
type spawnBlock struct {
	pos          Position
	monsterTypes map[*creatures.MonsterType]uint32 // type -> weight (for random)
	lastSpawn    time.Time
	interval     time.Duration
	direction    Direction
}

// getMonsterType picks which type this slot spawns, porting
// spawnBlock_t::getMonsterType (spawn_monster.cpp:457): a boss wins outright,
// otherwise the pick is weighted, walking the types from heaviest to lightest.
func (sb *spawnBlock) getMonsterType() *creatures.MonsterType {
	if len(sb.monsterTypes) == 0 {
		return nil
	}

	type weighted struct {
		mType  *creatures.MonsterType
		weight uint32
	}
	ordered := make([]weighted, 0, len(sb.monsterTypes))
	var totalWeight uint32
	for mType, weight := range sb.monsterTypes {
		if mType == nil {
			continue
		}
		// C++ warns when a boss shares a spawn block with others, then takes it.
		if mType.IsBoss() {
			return mType
		}
		totalWeight += weight
		ordered = append(ordered, weighted{mType, weight})
	}
	if totalWeight == 0 {
		return nil
	}
	if len(ordered) == 1 {
		return ordered[0].mType
	}

	sort.Slice(ordered, func(i, j int) bool { return ordered[i].weight > ordered[j].weight })

	randomWeight := uint32(rand.Intn(int(totalWeight)))
	for _, w := range ordered {
		if randomWeight < w.weight {
			return w.mType
		}
		randomWeight -= w.weight
	}
	return ordered[len(ordered)-1].mType
}

// SpawnData groups the parsed spawn info for one creature entry.
type SpawnData struct {
	Name      string
	Pos       Position
	Radius    int
	Interval  time.Duration
	IsNPC     bool
	Direction Direction
}

// SpawnBlock groups creatures that share the same center+radius.
type SpawnBlock struct {
	centerPos Position
	radius    int
	interval  time.Duration
	blocks    map[uint32]*spawnBlock // spawnId -> block
	spawned   map[uint32]Creature    // spawnId -> creature

	// stateMu guards blocks/spawned. C++ tracks nothing else per spawn group:
	// occupancy is spawnedMonsterMap membership, per slot.
	stateMu sync.Mutex

	// checkActive mirrors checkSpawnMonsterEvent != 0: whether this group is on
	// the maintenance sweep.
	checkActive bool

	// Reference to parent engine
	engine *SpawnEngine
}

// SpawnEngine manages all spawns in the world (C++ SpawnsMonster equivalent).
type SpawnEngine struct {
	world           *World
	Types           *creatures.TypeRegistry
	blocks          []*SpawnBlock
	nextSpawnID     uint32
	creatureToSpawn map[uint32]*spawnBlock // creatureID -> block (fast death lookup)
	mu              sync.RWMutex
}

const (
	nonBlockableInterval = 1400 * time.Millisecond // C++ NONBLOCKABLE_SPAWN_MONSTER_INTERVAL
	defaultSpawnInterval = 30 * time.Second        // C++ default
)

func NewSpawnEngine(w *World, types *creatures.TypeRegistry) *SpawnEngine {
	return &SpawnEngine{
		world:           w,
		Types:           types,
		creatureToSpawn: make(map[uint32]*spawnBlock),
	}
}

// RegisterHooks connects death tracking.
func (e *SpawnEngine) RegisterHooks() {
	if e.world == nil {
		return
	}
	e.world.OnCreatureDied = func(c Creature) {
		e.CreatureDied(c)
	}
}

// LoadSpawns parses XML spawn data into blocks matching C++ structure.
func (e *SpawnEngine) LoadSpawns(data *spawns.SpawnsData) {
	allNodes := append(data.Spawns, data.Monsters...)
	allNodes = append(allNodes, data.NPCs...)
	for _, sn := range allNodes {
		block := &SpawnBlock{
			centerPos: Position{X: uint16(sn.CenterX), Y: uint16(sn.CenterY), Z: uint8(sn.CenterZ)},
			radius:    sn.Radius,
			interval:  defaultSpawnInterval,
			blocks:    make(map[uint32]*spawnBlock),
			spawned:   make(map[uint32]Creature),
			engine:    e,
		}
		for _, mn := range sn.Monsters {
			pos := e.resolveSpawnPos(sn, mn.X, mn.Y, mn.Z)
			interval := time.Duration(mn.SpawnTime) * time.Second
			if interval <= 0 {
				interval = defaultSpawnInterval
			}
			if mn.SpawnTime > 0 {
				block.interval = interval // first valid spawntime sets block interval
			}
			dir := Direction(mn.Direction)
			block.addMonster(mn.Name, pos, dir, interval)
		}
		for _, nn := range sn.NPCs {
			pos := e.resolveSpawnPos(sn, nn.X, nn.Y, nn.Z)
			dir := Direction(nn.Direction)
			e.spawnNPC(nn.Name, pos, dir)
		}
		if len(block.blocks) > 0 {
			e.mu.Lock()
			e.blocks = append(e.blocks, block)
			e.mu.Unlock()
		}
	}
}

func (e *SpawnEngine) resolveSpawnPos(sn spawns.SpawnNode, x, y, z int) Position {
	pos := Position{
		X: uint16(sn.CenterX + x),
		Y: uint16(sn.CenterY + y),
		Z: uint8(sn.CenterZ),
	}
	if z != 0 {
		pos.Z = uint8(z)
	}
	if x > 1000 {
		pos.X = uint16(x)
	}
	if y > 1000 {
		pos.Y = uint16(y)
	}
	return pos
}

// ---- SpawnBlock methods ----

func (b *SpawnBlock) addMonster(name string, pos Position, dir Direction, interval time.Duration) {
	e := b.engine
	if e == nil {
		return
	}
	var mType *creatures.MonsterType
	if e.Types != nil {
		mType = e.Types.Monsters[strings.ToLower(name)]
	}
	if mType == nil {
		slog.Debug("spawn: monster type not found", "name", name)
		return
	}
	id := e.nextID()
	b.blocks[id] = &spawnBlock{
		pos:          pos,
		monsterTypes: map[*creatures.MonsterType]uint32{mType: 1},
		lastSpawn:    time.Time{},
		interval:     interval,
		direction:    dir,
	}
}

func (e *SpawnEngine) nextID() uint32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nextSpawnID++
	return e.nextSpawnID
}

// CreatureDied handles death events (called from World.OnCreatureDied).
func (e *SpawnEngine) CreatureDied(c Creature) {
	e.mu.Lock()
	sb, ok := e.creatureToSpawn[c.GetID()]
	if ok {
		sb.lastSpawn = time.Now()
		delete(e.creatureToSpawn, c.GetID())
	}
	e.mu.Unlock()
	if ok {
		// Find parent block and remove from spawned map
		for _, block := range e.blocks {
			block.stateMu.Lock()
			for id, cr := range block.spawned {
				if cr.GetID() == c.GetID() {
					delete(block.spawned, id)
					break
				}
			}
			block.stateMu.Unlock()
		}
	}
}

// Start begins spawn checking.
func (e *SpawnEngine) Start() {
	GlobalDispatcher.AddEvent(1*time.Second, e.checkSpawns)
}

// checkSpawns runs every 1s, matching C++ SpawnMonster::checkSpawnMonster.
func (e *SpawnEngine) checkSpawns() {
	e.checkSpawnsOnce(time.Now())
	GlobalDispatcher.AddEvent(1*time.Second, e.checkSpawns)
}

// checkSpawnsOnce is one pass, split out so it can be driven with an explicit
// clock instead of only from the dispatcher.
func (e *SpawnEngine) checkSpawnsOnce(now time.Time) {
	for _, block := range e.blocks {
		block.CheckSpawnMonster(e.world, now)
	}
}

func (e *SpawnEngine) findPlayerNear(pos Position) bool {
	for _, p := range e.world.Players() {
		if p.HasFlag(0) { // PlayerFlag_IgnoredByMonsters stub
			continue
		}
		if p.Pos.Z != pos.Z {
			continue
		}
		dx := int(p.Pos.X) - int(pos.X)
		dy := int(p.Pos.Y) - int(pos.Y)
		if dx*dx+dy*dy <= 100 { // ~10 tiles radius
			return true
		}
	}
	return false
}

func (e *SpawnEngine) spawnCreatureInBlock(block *SpawnBlock, id uint32, sb *spawnBlock, mType *creatures.MonsterType, now time.Time) Creature {
	pos := sb.pos
	if block.radius > 0 {
		pos = e.randomSpawnPos(block.centerPos, block.radius)
	}
	if mType == nil {
		return nil
	}

	nid := e.world.nextCreatureID.Add(1)
	monster := NewMonster(nid, mType.Name, mType)
	monster.SetPosition(pos)
	monster.SetDirection(sb.direction)

	e.world.AddCreature(monster)

	sb.lastSpawn = now
	e.mu.Lock()
	e.creatureToSpawn[monster.GetID()] = sb
	e.mu.Unlock()

	return monster
}

// spawnNPC creates and places an NPC in the world immediately (C++ SpawnNpc startup behavior).
func (e *SpawnEngine) spawnNPC(name string, pos Position, dir Direction) {
	nType := e.Types.Npcs[strings.ToLower(name)]
	if nType == nil {
		slog.Debug("spawn: npc type not found", "name", name)
		return
	}
	nid := e.world.nextCreatureID.Add(1)
	npc := NewNpc(nid, name, nType)
	npc.SetPosition(pos)
	npc.SetDirection(dir)
	// MasterPos anchors the walk radius; Npc::isInSpawnRange measures against it.
	npc.MasterPos = pos
	e.world.AddCreature(npc)
	slog.Debug("spawned npc", "name", name, "pos", pos)
}

func (e *SpawnEngine) randomSpawnPos(center Position, radius int) Position {
	if radius <= 0 {
		return center
	}
	dx := rand.Intn(radius*2+1) - radius
	dy := rand.Intn(radius*2+1) - radius
	return Position{
		X: uint16(int(center.X) + dx),
		Y: uint16(int(center.Y) + dy),
		Z: center.Z,
	}
}

// Reload clears all spawns and reloads from parsed data.
func (e *SpawnEngine) Reload(data *spawns.SpawnsData) {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Remove all spawned creatures
	for _, block := range e.blocks {
		for _, cr := range block.spawned {
			e.world.RemoveCreature(cr.GetID())
		}
	}
	e.blocks = nil
	e.creatureToSpawn = make(map[uint32]*spawnBlock)
	e.nextSpawnID = 0
	e.LoadSpawns(data)
	slog.Info("spawns reloaded", "count", len(e.blocks))
}

// Stats returns spawn system statistics.
func (e *SpawnEngine) Stats() (total, alive int) {
	for _, block := range e.blocks {
		total += len(block.blocks)
		alive += len(block.spawned)
	}
	return
}
