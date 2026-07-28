package game

import (
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/creatures"
	"github.com/opentibiabr/canary-go/internal/game/spawns"
)

// RespawnType controls how the spawn timer works after death.
type RespawnType uint8

const (
	RespawnNormal   RespawnType = 0 // full spawnTime wait after death
	RespawnPeriodic RespawnType = 1 // respawn at fixed intervals regardless
)

// Spawn represents a single creature spawn point.
type Spawn struct {
	Name      string
	Pos       Position
	Radius    int
	SpawnTime time.Duration
	IsNPC     bool
	Type      RespawnType

	creatureID uint32
	lastDeath  time.Time
	nextSpawn  time.Time
}

// SpawnEngine manages all spawns in the world.
type SpawnEngine struct {
	world           *World
	spawns          []*Spawn
	Types           *creatures.TypeRegistry
	creatureToSpawn map[uint32]*Spawn // creatureID -> spawn tracking
}

// NewSpawnEngine creates a new SpawnEngine.
func NewSpawnEngine(w *World, types *creatures.TypeRegistry) *SpawnEngine {
	return &SpawnEngine{
		world:           w,
		Types:           types,
		creatureToSpawn: make(map[uint32]*Spawn),
	}
}

// LoadSpawns loads spawns from parsed XML data.
func (e *SpawnEngine) LoadSpawns(data *spawns.SpawnsData) {
	allNodes := append(data.Spawns, data.Monsters...)
	allNodes = append(allNodes, data.NPCs...)
	for _, sn := range allNodes {
		for _, mn := range sn.Monsters {
			pos := e.resolveSpawnPos(sn, mn.X, mn.Y, mn.Z)
			e.AddSpawn(mn.Name, pos, sn.Radius, time.Duration(mn.SpawnTime)*time.Second, false)
		}
		for _, nn := range sn.NPCs {
			pos := e.resolveSpawnPos(sn, nn.X, nn.Y, nn.Z)
			e.AddSpawn(nn.Name, pos, sn.Radius, time.Duration(nn.SpawnTime)*time.Second, true)
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

func (e *SpawnEngine) AddSpawn(name string, pos Position, radius int, spawnTime time.Duration, isNpc bool) {
	e.spawns = append(e.spawns, &Spawn{
		Name:      name,
		Pos:       pos,
		Radius:    radius,
		SpawnTime: spawnTime,
		IsNPC:     isNpc,
		Type:      RespawnNormal,
	})
}

// CreatureDied is called by the combat engine when a creature dies.
// It resets the spawn tracking so the respawn timer starts.
func (e *SpawnEngine) CreatureDied(c Creature) {
	if s, ok := e.creatureToSpawn[c.GetID()]; ok {
		s.creatureID = 0
		s.lastDeath = time.Now()
		s.nextSpawn = s.lastDeath.Add(s.SpawnTime)
		delete(e.creatureToSpawn, c.GetID())
	}
}

// RegisterHooks connects the SpawnEngine to World death events.
func (e *SpawnEngine) RegisterHooks() {
	if e.world == nil {
		return
	}
	e.world.OnCreatureDied = func(c Creature) {
		e.CreatureDied(c)
	}
}

// Start begins the spawn checking loop.
func (e *SpawnEngine) Start() {
	GlobalDispatcher.AddEvent(1*time.Second, e.checkSpawns)
}

func (e *SpawnEngine) checkSpawns() {
	now := time.Now()
	for _, s := range e.spawns {
		if s.creatureID != 0 && e.world.CreatureByID(s.creatureID) != nil {
			continue // still alive
		}
		if !s.lastDeath.IsZero() && now.Before(s.nextSpawn) {
			continue // not time yet
		}
		if s.creatureID != 0 {
			// creature died but we missed the death event; recover
			s.creatureID = 0
			s.lastDeath = now
			s.nextSpawn = now.Add(s.SpawnTime)
			continue
		}
		e.spawnCreature(s)
	}
	GlobalDispatcher.AddEvent(1*time.Second, e.checkSpawns)
}

func (e *SpawnEngine) spawnCreature(s *Spawn) {
	pos := e.randomSpawnPos(s)

	if s.IsNPC {
		npc := e.spawnNPC(s, pos)
		if npc == nil {
			return
		}
		s.creatureID = npc.GetID()
		e.creatureToSpawn[npc.GetID()] = s
	} else {
		monster := e.spawnMonster(s, pos)
		if monster == nil {
			return
		}
		s.creatureID = monster.GetID()
		e.creatureToSpawn[monster.GetID()] = s
	}
}

func (e *SpawnEngine) randomSpawnPos(s *Spawn) Position {
	if s.Radius <= 0 {
		return s.Pos
	}
	// Random offset within radius, matching C++ Monster::randomWalk
	dx := rand.Intn(s.Radius*2+1) - s.Radius
	dy := rand.Intn(s.Radius*2+1) - s.Radius
	pos := Position{
		X: uint16(int(s.Pos.X) + dx),
		Y: uint16(int(s.Pos.Y) + dy),
		Z: s.Pos.Z,
	}
	// Fallback to center if outside map bounds
	if e.world.Map != nil && e.world.Map.GetTile(pos) == nil {
		return s.Pos
	}
	return pos
}

func (e *SpawnEngine) spawnNPC(s *Spawn, pos Position) *Npc {
	var nType *creatures.NpcType
	if e.Types != nil {
		nType = e.Types.Npcs[strings.ToLower(s.Name)]
	}
	if nType == nil {
		slog.Debug("spawned npc type not found in registry; skipping spawn", "name", s.Name)
		return nil
	}
	id := e.world.nextCreatureID.Add(1)
	npc := NewNpc(id, s.Name, nType)
	npc.SetPosition(pos)
	e.world.AddCreature(npc)
	slog.Debug("spawned npc", "name", s.Name, "pos", pos)
	return npc
}

func (e *SpawnEngine) spawnMonster(s *Spawn, pos Position) *Monster {
	var mType *creatures.MonsterType
	if e.Types != nil {
		mType = e.Types.Monsters[strings.ToLower(s.Name)]
	}
	if mType == nil {
		slog.Debug("spawned monster type not found in registry; skipping spawn", "name", s.Name)
		return nil
	}
	id := e.world.nextCreatureID.Add(1)
	monster := NewMonster(id, s.Name, mType)
	monster.SetPosition(pos)
	e.world.AddCreature(monster)
	slog.Debug("spawned monster", "name", s.Name, "pos", pos)
	return monster
}

// SpawnCount returns the number of managed spawn points.
func (e *SpawnEngine) SpawnCount() int { return len(e.spawns) }

// ActiveSpawnCount returns how many spawns are currently alive.
func (e *SpawnEngine) ActiveSpawnCount() int {
	count := 0
	for _, s := range e.spawns {
		if s.creatureID != 0 && e.world.CreatureByID(s.creatureID) != nil {
			count++
		}
	}
	return count
}
