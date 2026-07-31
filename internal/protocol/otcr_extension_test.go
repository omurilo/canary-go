package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// The creature description must end at the walkthrough byte. C++ appends the OTCR
// shader name and attached-effect list only when isOTCR (protocolgame.cpp:9659),
// and isOTCR is set by sendOTCRFeatures — the 0x43 frame that tells the client to
// enable them. This server never sends it, so no client reads those bytes and
// writing them shifts everything after: three bytes per creature, in a frame that
// carries one description per visible creature. A stock Tibia client dies parsing
// the enterWorld frame; OTClient logs "no thing at pos" and keeps going.
//
// The exact byte count is the assertion, because a length is what a desync is.
func TestCreatureDescriptionHasNoOTCRTail(t *testing.T) {
	world := game.NewWorld()
	g := &GameProtocol{
		player:  &game.Player{ID: 1, Name: "Viewer", GroupID: 1},
		deps:    &Deps{World: world, Items: world.Items},
		known:   map[uint32]bool{},
		profile: &Profile{Version: VersionCurrent},
	}

	rat := game.NewMonster(10, "Rat", nil)
	rat.MaxHealth, rat.Health = 100, 100

	w := netmsg.NewWriter()
	g.addCreature(w, rat)

	// A first-time (unknown) monster with an empty outfit and no mount:
	//   0x0061 u16 2 | removedKnownId u32 4 | id u32 4 | creatureType 1 | "Rat" 2+3
	//   health 1 | direction 1
	//   outfit: lookType u16 2 + lookTypeEx u16 2 + lookMount u16 2
	//   light level 1 | light colour 1 | speed u16 2
	//   icon count 1 | skull 1 | shield 1 | guild emblem 1 (unknown only)
	//   creatureType 1 | speech bubble 1 | mark 1 | inspection 1 | walkthrough 1
	const want = 2 + 4 + 4 + 1 + 5 + 1 + 1 + 6 + 1 + 1 + 2 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1
	got := w.Bytes()
	if len(got) != want {
		t.Fatalf("creature description = %d bytes, want %d — a difference of %d is the OTCR tail (u16 empty shader + effect count) leaking to a client that does not read it",
			len(got), want, len(got)-want)
	}
	// The description must end on the walkthrough flag: 0x01 for a solid creature.
	if got[len(got)-1] != 0x01 {
		t.Errorf("last byte = 0x%02X, want 0x01 (walkthrough: solid)", got[len(got)-1])
	}
}

// isOTCR must not be inferred from the operating system the client announced. The
// OS says what the client IS; the extensions depend on what it was TOLD, and C++
// only tells it inside sendOTCRFeatures.
func TestIsOTCRIsNotDerivedFromClientOS(t *testing.T) {
	for _, os := range []uint16{0, 2, 5, 6, clientOSOTClientLinux, 11, 12} {
		g := &GameProtocol{clientOS: os}
		if g.isOTCR() {
			t.Errorf("clientOS %d: isOTCR() = true, but this server never sends the 0x43 OTCR feature frame", os)
		}
	}
	// The OS is still parsed and kept — C++ uses it for isOTC.
	g := &GameProtocol{clientOS: clientOSOTClientLinux}
	if !g.isOTC() {
		t.Errorf("clientOS %d: isOTC() = false, want true", clientOSOTClientLinux)
	}
	if (&GameProtocol{clientOS: 2}).isOTC() {
		t.Errorf("clientOS 2 (CLIENTOS_WINDOWS): isOTC() = true, want false")
	}
}
