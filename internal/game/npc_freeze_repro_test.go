package game

import (
	"testing"

	"github.com/omurilo/canary-go/internal/creatures"
)

// TestNpcWalksWithPlayerNearby reproduces the live server scenario: a player
// stands next to an NPC that should wander. If the NPC never steps, something in
// the think chain (spectators, walk gate, tile checks) is blocking it.
func TestNpcWalksWithPlayerNearby(t *testing.T) {
	w := NewWorld()

	// Build a walkable 5x5 ground around the NPC so GetRandomStep has room.
	for x := 95; x <= 105; x++ {
		for y := 95; y <= 105; y++ {
			pos := Position{X: uint16(x), Y: uint16(y), Z: 7}
			w.Map.SetTile(pos, &Tile{Ground: &Item{ID: 1}})
		}
	}

	nt := &creatures.NpcType{
		Name:         "Walker",
		Speed:        100,
		WalkInterval: 500,
		WalkRadius:   4,
	}
	npc := NewNpc(10, "Walker", nt)
	pos := Position{X: 100, Y: 100, Z: 7}
	npc.SetPosition(pos)
	npc.MasterPos = pos
	w.AddCreature(npc)

	// A normal (non-ghost) player standing 1 tile away.
	player := &Player{ID: 1, Name: "Bob", GroupID: 1}
	player.Pos = Position{X: 101, Y: 100, Z: 7}
	player.World = w
	w.players[player.ID] = player

	e := &NpcEngine{world: w}
	start := npc.GetPosition()
	moved := false
	for i := 0; i < 20; i++ {
		e.thinkNpc(npc, 500)
		if npc.GetPosition() != start {
			moved = true
			break
		}
	}
	if !moved {
		t.Fatalf("NPC never moved after 20 ticks (10s). idle=%v spectators=%d pos=%v start=%v",
			npc.IsIdle(), len(npc.spectators), npc.GetPosition(), start)
	}
}

// TestNpcTurnsTowardPlayer covers the turn path driven by a player move.
func TestNpcTurnsTowardPlayer(t *testing.T) {
	w := NewWorld()
	for x := 95; x <= 105; x++ {
		for y := 95; y <= 105; y++ {
			w.Map.SetTile(Position{X: uint16(x), Y: uint16(y), Z: 7}, &Tile{Ground: &Item{ID: 1}})
		}
	}

	nt := &creatures.NpcType{Name: "Facer", Speed: 100}
	npc := NewNpc(10, "Facer", nt)
	pos := Position{X: 100, Y: 100, Z: 7}
	npc.SetPosition(pos)
	npc.MasterPos = pos
	w.AddCreature(npc)

	player := &Player{ID: 1, Name: "Bob", GroupID: 1}
	player.Pos = Position{X: 101, Y: 100, Z: 7}
	player.World = w
	w.players[player.ID] = player

	// Player starts a conversation (greets), then moves; NPC should face them.
	npc.SetPlayerInteraction(player.ID, 0)
	if got := npc.GetDirection(); got != DirEast {
		t.Fatalf("after greeting, npc facing = %v, want %v (east toward the player)", got, DirEast)
	}

	e := &NpcEngine{world: w}
	e.thinkNpc(npc, 500) // refresh spectators

	// Player walks south one tile: now at (101, 101), diagonally SE of NPC.
	player.Pos = Position{X: 101, Y: 101, Z: 7}
	npc.HandlePlayerMove(w, player, player.Pos)

	if got := npc.GetDirection(); got != DirSouth {
		t.Errorf("after moving SE of the NPC, facing = %v, want %v (south toward the player)", got, DirSouth)
	}
}
