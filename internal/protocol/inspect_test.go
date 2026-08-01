package protocol

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
	"github.com/omurilo/canary-go/internal/netmsg"
)

func TestParseInspectPlayer(t *testing.T) {
	world := game.NewWorld()
	p1 := &game.Player{ID: 1001, Name: "God Tester", Level: 200}
	p2 := &game.Player{ID: 1002, Name: "Subject Player", Level: 50}

	world.AddPlayer(p1, nil)
	world.AddPlayer(p2, nil)

	catalog := &items.Catalog{}
	gp := &GameProtocol{
		player: p1,
		deps: &Deps{
			World: world,
			Items: catalog,
		},
	}

	// Test parseInspectPlayer packet
	w := netmsg.NewWriter()
	w.AddByte(1)      // action
	w.AddU32(p2.ID)   // target player ID
	buf := w.Bytes()

	r := netmsg.NewReader(buf)
	gp.parseInspectPlayer(r)

	// Test parseCyclopediaCharacterInfo packet
	w2 := netmsg.NewWriter()
	w2.AddU32(p2.ID)  // target player ID
	w2.AddByte(9)     // inspection type
	r2 := netmsg.NewReader(w2.Bytes())
	gp.parseCyclopediaCharacterInfo(r2)
}
