package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
)

// Using a container that is already open closes it instead of opening a second
// window onto the same bag (Actions::useItem, actions.cpp:371). Go registered it
// again under a fresh cid, so the pouch lived under two ids and every refresh sent
// two identical 0x6E windows — visible in a session log as ContainerOpen cid=03 and
// cid=04 with byte-identical payloads.
func TestContainerIDIsReusedNotDuplicated(t *testing.T) {
	p := &game.Player{Name: "Owner"}
	pouch := &game.Item{ID: game.ItemGoldPouch, Contents: []*game.Item{}}

	first := p.AddContainerWithPos(pouch, game.Position{}, false)
	if first < 0 {
		t.Fatal("the first open should get a cid")
	}
	// The same container must resolve to the SAME cid, never a second one.
	if again := p.AddContainerWithPos(pouch, game.Position{}, false); again != first {
		t.Errorf("reopening gave cid %d, want the existing %d", again, first)
	}
	if got := p.GetContainerID(pouch); got != first {
		t.Errorf("GetContainerID = %d, want %d", got, first)
	}

	// A different container gets its own cid.
	other := &game.Item{ID: 1987, Contents: []*game.Item{}}
	if o := p.AddContainerWithPos(other, game.Position{}, false); o == first || o < 0 {
		t.Errorf("a different container must get its own cid, got %d", o)
	}

	// Once closed, the container is no longer registered, so a later use opens it
	// afresh rather than toggling forever.
	p.CloseContainer(uint8(first))
	if got := p.GetContainerID(pouch); got != -1 {
		t.Errorf("after closing, GetContainerID = %d, want -1", got)
	}
}
