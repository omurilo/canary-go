package protocol

import (
	"testing"
	"time"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/items"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

func TestPlayerPotionExhaustion(t *testing.T) {
	player := &game.Player{}

	if !player.CanDoPotionAction() {
		t.Fatalf("expected CanDoPotionAction to be true initially")
	}

	// Set 1 second delay
	player.SetNextPotionAction(1 * time.Second)

	if player.CanDoPotionAction() {
		t.Fatalf("expected CanDoPotionAction to be false immediately after setting 1s delay")
	}

	// Check standard action
	if !player.CanDoAction() {
		t.Fatalf("expected CanDoAction to be true initially")
	}

	player.SetNextAction(200 * time.Millisecond)
	if player.CanDoAction() {
		t.Fatalf("expected CanDoAction to be false immediately after setting 200ms delay")
	}
}

func TestUseItemCooldownPacket(t *testing.T) {
	w := netmsg.NewWriter()
	w.AddByte(0xA6)
	w.AddU32(1000)

	b := w.Bytes()
	if len(b) != 5 {
		t.Fatalf("expected packet length 5, got %d", len(b))
	}
	if b[0] != 0xA6 {
		t.Fatalf("expected opcode 0xA6, got 0x%02X", b[0])
	}
}

func TestIsExAction(t *testing.T) {
	gp := &GameProtocol{}

	// Mana potion (237)
	potionItem := &game.Item{ID: 237}
	if !gp.isExAction(potionItem) {
		t.Fatalf("expected mana potion (237) to be recognized as exAction")
	}

	// Health potion (266)
	healthItem := &game.Item{ID: 266}
	if !gp.isExAction(healthItem) {
		t.Fatalf("expected health potion (266) to be recognized as exAction")
	}

	// Non-potion item
	swordItem := &game.Item{ID: 2400}
	if gp.isExAction(swordItem) {
		t.Fatalf("expected sword (2400) not to be recognized as exAction")
	}

	_ = items.Catalog{}
}
