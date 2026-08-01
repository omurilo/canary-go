package protocol

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

func TestNpcCloseChannelPacket(t *testing.T) {
	world := game.NewWorld()
	player := &game.Player{ID: 1, Name: "TestPlayer"}
	world.AddPlayer(player, nil)

	npc := game.NewNpc(2, "TestNpc", nil)
	npc.SetPlayerInteraction(player.ID, 1)
	world.AddCreature(npc)

	if !npc.IsInteractingWithPlayer(player.ID) {
		t.Fatal("expected player to be interacting with NPC")
	}

	deps := &Deps{World: world}
	gp := &GameProtocol{deps: deps, player: player}

	writer := netmsg.NewWriter()
	writer.AddU16(0x00) // channelId
	reader := netmsg.NewReader(writer.Bytes())

	gp.parseCloseChannel(reader)

	if npc.IsInteractingWithPlayer(player.ID) {
		t.Error("expected interaction to be removed after parseCloseChannel")
	}
}

func TestPositionMaxDistanceForNpc(t *testing.T) {
	p1 := game.Position{X: 100, Y: 100, Z: 7}
	p2 := game.Position{X: 103, Y: 103, Z: 7}
	p3 := game.Position{X: 104, Y: 100, Z: 7}
	p4 := game.Position{X: 100, Y: 100, Z: 8}

	if dist := p1.MaxDistance(p2); dist != 3 {
		t.Errorf("expected max distance 3, got %d", dist)
	}
	if dist := p1.MaxDistance(p3); dist != 4 {
		t.Errorf("expected max distance 4, got %d", dist)
	}
	if dist := p1.MaxDistance(p4); dist != -1 {
		t.Errorf("expected max distance -1 for different floor, got %d", dist)
	}
}
