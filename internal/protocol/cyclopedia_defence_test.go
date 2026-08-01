package protocol

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// otcReader replays the client's read sequence over a frame we produced. The
// point is not to check values — it is to check that the client consumes the
// frame exactly, with nothing left over and nothing missing. A layout drift in
// either direction is invisible to any test that only inspects fields.
type otcReader struct {
	t   *testing.T
	buf []byte
	pos int
}

func (r *otcReader) need(n int, what string) []byte {
	r.t.Helper()
	if r.pos+n > len(r.buf) {
		r.t.Fatalf("frame ran out reading %s: wanted %d bytes at pos %d of %d "+
			"— the client would report \"eof reached\" here",
			what, n, r.pos, len(r.buf))
	}
	b := r.buf[r.pos : r.pos+n]
	r.pos += n
	return b
}

func (r *otcReader) u8(what string) uint8 { return r.need(1, what)[0] }
func (r *otcReader) u16(what string) uint16 {
	b := r.need(2, what)
	return uint16(b[0]) | uint16(b[1])<<8
}
func (r *otcReader) u32(what string) uint32 {
	b := r.need(4, what)
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// A wire double is a precision byte followed by a u32 — five bytes, not eight
// (otclient/src/framework/net/inputmessage.cpp:101-106).
func (r *otcReader) double(what string) { r.need(1, what+".precision"); r.need(4, what+".value") }

func (r *otcReader) done() {
	r.t.Helper()
	if r.pos != len(r.buf) {
		r.t.Errorf("client stopped at pos %d but the frame is %d bytes: %d trailing byte(s)",
			r.pos, len(r.buf), len(r.buf)-r.pos)
	}
}

// The defence-stats frame ended seven bytes short of what the client reads —
// a spare u16 and the mitigationCombatTactics double — and OTCR reported
//
//	129 bytes, 0 unread at pos 129, last opcode 0xDA (218)
//
// This walks the frame with the client's own sequence
// (protocolgameparse.cpp:6110-6152) and fails if either side drifts again.
func TestDefenceStatsFrameMatchesClientReads(t *testing.T) {
	// clientOS 11 is CLIENTOS_OTCLIENT_WINDOWS. The official client reads a
	// shorter frame; see TestDefenceStatsOfficialClientFrameIsShorter.
	for _, os := range []uint16{10, 11, 12} {
		t.Run("otc", func(t *testing.T) { readDefenceFrame(t, os, true) })
	}
}

// The official client does not read the spare u16 or mitigationCombatTactics.
// Sending them crashed it — the first version of this fix did exactly that.
func TestDefenceStatsOfficialClientFrameIsShorter(t *testing.T) {
	otcLen := len(buildDefenceFrame(t, 11).Bytes())
	officialLen := len(buildDefenceFrame(t, 2).Bytes())

	// A wire double is 5 bytes (precision + u32), so the two fields are 2 + 5.
	if want := otcLen - 7; officialLen != want {
		t.Errorf("official frame is %d bytes, want %d (OTC %d minus the spare u16 and mitigationCombatTactics)",
			officialLen, want, otcLen)
	}
	readDefenceFrame(t, 2, false)
}

func buildDefenceFrame(t *testing.T, clientOS uint16) *netmsg.Writer {
	t.Helper()
	w := game.NewWorld()
	w.Items = items.NewCatalog(&items.ItemType{ID: 1, Name: "ground"})
	p := &game.Player{Name: "Tester", Level: 100}
	p.Skills[game.SkillShielding] = 80

	g := &GameProtocol{player: p, clientOS: clientOS, clientVersion: 1525,
		deps: &Deps{World: w, Items: w.Items}}
	frame := g.buildDefenceStats()
	if frame == nil {
		t.Fatal("no frame built")
	}
	return frame
}

func readDefenceFrame(t *testing.T, clientOS uint16, otc bool) {
	t.Helper()
	r := &otcReader{t: t, buf: buildDefenceFrame(t, clientOS).Bytes()}
	if op := r.u8("opcode"); op != 0xDA {
		t.Fatalf("opcode = 0x%02X, want 0xDA", op)
	}
	if kind := r.u8("infoType"); kind != cyclopediaCharacterInfoDefenceStats {
		t.Fatalf("infoType = %d, want %d", kind, cyclopediaCharacterInfoDefenceStats)
	}
	r.u8("errorCode")

	r.double("dodgeTotal")
	r.double("dodgeBase")
	r.double("dodgeBonus")
	r.double("unused")
	r.double("dodgeWheel")

	r.u32("magicShieldCapacity")
	r.u16("magicShieldCapacityFlat")
	r.double("magicShieldCapacityPercent")

	r.u16("reflectPhysical")
	r.u16("armor")
	// GameVocationMonk, enabled from client version 1500
	// (otclient/modules/game_features/features.lua:270-272).
	r.u16("mantra")

	r.u16("defense")
	r.u16("defenseEquipment")
	r.u8("defenseSkillType")
	r.u16("shieldingSkill")
	r.u16("defenseWheel")
	if otc {
		r.u16("spare") // parse:6135 — OTC only
	}

	r.double("mitigation")
	r.double("mitigationBase")
	r.double("mitigationEquipment")
	r.double("mitigationShield")
	r.double("mitigationWheel")
	if otc {
		r.double("mitigationCombatTactics") // parse:6142 — OTC only
	}

	n := r.u8("combatsCount")
	for i := 0; i < int(n); i++ {
		if r.u8("elementType") == 0x04 {
			r.u8("element")
			r.double("resistance")
		}
	}

	r.done()
}
