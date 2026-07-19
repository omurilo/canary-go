package game

import (
	"time"

	"github.com/opentibiabr/canary-go/internal/game/spawns"
)

// Spawn represents a single creature spawn point.
type Spawn struct {
	Name      string
	Pos       Position
	Radius    int
	SpawnTime time.Duration
	IsNPC     bool

	creatureID uint32
	lastDeath  time.Time
}

// SpawnEngine manages all spawns in the world.
type SpawnEngine struct {
	world  *World
	spawns []*Spawn
}

// NewSpawnEngine creates a new SpawnEngine.
func NewSpawnEngine(w *World) *SpawnEngine {
	return &SpawnEngine{world: w}
}

// LoadSpawns loads spawns from parsed XML data.
func (e *SpawnEngine) LoadSpawns(data *spawns.SpawnsData) {
	for _, sn := range data.Spawns {
		for _, mn := range sn.Monsters {
			pos := Position{
				X: uint16(sn.CenterX + mn.X),
				Y: uint16(sn.CenterY + mn.Y),
				Z: uint8(sn.CenterZ),
			}
			// Fallback for absolute Z
			if mn.Z != 0 {
				pos.Z = uint8(mn.Z)
			}
			// Some maps use absolute X/Y if they are very large
			if mn.X > 1000 {
				pos.X = uint16(mn.X)
			}
			if mn.Y > 1000 {
				pos.Y = uint16(mn.Y)
			}
			e.AddSpawn(mn.Name, pos, sn.Radius, time.Duration(mn.SpawnTime)*time.Second, false)
		}
		for _, nn := range sn.NPCs {
			pos := Position{
				X: uint16(sn.CenterX + nn.X),
				Y: uint16(sn.CenterY + nn.Y),
				Z: uint8(sn.CenterZ),
			}
			if nn.Z != 0 {
				pos.Z = uint8(nn.Z)
			}
			if nn.X > 1000 {
				pos.X = uint16(nn.X)
			}
			if nn.Y > 1000 {
				pos.Y = uint16(nn.Y)
			}
			e.AddSpawn(nn.Name, pos, sn.Radius, time.Duration(nn.SpawnTime)*time.Second, true)
		}
	}
}

func (e *SpawnEngine) AddSpawn(name string, pos Position, radius int, spawnTime time.Duration, isNpc bool) {
	e.spawns = append(e.spawns, &Spawn{
		Name:      name,
		Pos:       pos,
		Radius:    radius,
		SpawnTime: spawnTime,
		IsNPC:     isNpc,
	})
}

// Start begins the spawn checking loop.
func (e *SpawnEngine) Start() {
	GlobalDispatcher.AddEvent(1*time.Second, e.checkSpawns)
}

func (e *SpawnEngine) checkSpawns() {
	now := time.Now()
	for _, s := range e.spawns {
		// If the creature is missing or dead
		if s.creatureID == 0 || e.world.CreatureByID(s.creatureID) == nil {
			if s.lastDeath.IsZero() || now.Sub(s.lastDeath) >= s.SpawnTime {
				e.spawnCreature(s)
			}
		}
	}
	GlobalDispatcher.AddEvent(5*time.Second, e.checkSpawns)
}

func (e *SpawnEngine) spawnCreature(s *Spawn) {
	var c Creature
	id := e.world.nextCreatureID.Add(1)

	if s.IsNPC {
		npc := NewNpc(id, s.Name)
		npc.SetPosition(s.Pos)
		c = npc
	} else {
		monster := NewMonster(id, s.Name, 100)
		monster.SetPosition(s.Pos)
		c = monster
	}

	e.world.AddCreature(c)
	s.creatureID = c.GetID()
}
