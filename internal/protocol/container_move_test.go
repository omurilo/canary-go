package protocol

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
)

func TestIsChildOf(t *testing.T) {
	bp1 := &game.Item{ID: 1988}
	bp2 := &game.Item{ID: 1988}
	sword := &game.Item{ID: 2400}

	bp1.Contents = []*game.Item{bp2, sword}

	if !isChildOf(bp1, bp1) {
		t.Fatalf("expected isChildOf(bp1, bp1) to be true")
	}

	if !isChildOf(bp1, bp2) {
		t.Fatalf("expected isChildOf(bp1, bp2) to be true")
	}

	if isChildOf(bp2, bp1) {
		t.Fatalf("expected isChildOf(bp2, bp1) to be false")
	}
}

func TestContainerDestinationResolution(t *testing.T) {
	cat := &items.Catalog{}
	bp := &game.Item{ID: 1988}
	childBp := &game.Item{ID: 1988}
	potion := &game.Item{ID: 237}

	bp.Contents = []*game.Item{childBp}

	player := &game.Player{}
	player.Inventory[game.ConstSlotBackpack] = bp

	gp := &GameProtocol{
		player: player,
		deps:   &Deps{Items: cat},
	}

	_ = gp
	_ = potion
}
