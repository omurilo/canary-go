package luaengine

import (
	"testing"
	"time"

	"github.com/omurilo/canary-go/internal/game"
)

func TestRecursiveDeadlockAvoidanceOnMoveAndSay(t *testing.T) {
	e := newTestEngine()

	// Register NPC
	npc := game.NewNpc(e.world.GenerateCreatureID(), "testnpc", nil)
	npc.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	e.world.AddCreature(npc)

	// Create Player
	player := &game.Player{Name: "TestPlayer", GroupID: 3}
	player.SetPosition(game.Position{X: 100, Y: 101, Z: 7})
	e.world.AddPlayer(player, nil)

	// Set NPC interaction
	npc.SetPlayerInteraction(player.ID, 0)

	// Wire OnCreatureMove and OnCreatureSay like main.go
	e.world.OnCreatureMove = func(c game.Creature, oldPos game.Position, newPos game.Position, oldStackPos map[uint32]int) {
		if playerObj, ok := c.(*game.Player); ok {
			for _, cr := range e.world.Creatures() {
				if npcObj, ok := cr.(*game.Npc); ok && npcObj.IsInteractingWithPlayer(playerObj.ID) {
					dist := playerObj.GetPosition().MaxDistance(npcObj.GetPosition())
					if dist < 0 || dist > 3 {
						targetNpc, targetPlayer := npcObj, playerObj
						game.GlobalDispatcher.AddEvent(0, func() {
							e.CallNpcCloseChannel(targetNpc, targetPlayer)
						})
						npcObj.RemovePlayerInteraction(playerObj.ID)
					}
				}
			}
		}
	}

	e.world.OnCreatureSay = func(speaker game.Creature, talkType byte, text string) {
		if playerObj, ok := speaker.(*game.Player); ok {
			spectators := e.world.SpectatorCreatures(speaker.GetPosition())
			for _, spec := range spectators {
				if npcObj, ok := spec.(*game.Npc); ok {
					targetNpc, targetPlayer, tType, txt := npcObj, playerObj, talkType, text
					game.GlobalDispatcher.AddEvent(0, func() {
						e.CallNpcOnCreatureSay(targetNpc, targetPlayer, tType, txt)
					})
				}
			}
		}
	}

	// Execute Lua code inside DoString (which holds e.mu.Lock()) that teleports the player and speaks
	script := `
		local p = Player("TestPlayer")
		p:teleportTo(Position(105, 105, 7))
		p:say("hello npc", TALKTYPE_SAY)
	`

	done := make(chan struct{})
	go func() {
		err := e.DoString(script)
		if err != nil {
			t.Errorf("DoString error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success! No deadlock occurred during teleport/say inside Lua
	case <-time.After(3 * time.Second):
		t.Fatal("DEADLOCK DETECTED! DoString timed out during teleportTo/say")
	}
}
