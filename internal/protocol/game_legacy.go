package protocol

import "github.com/opentibiabr/canary-go/internal/netmsg"

// addLegacyItemCount appends the count byte for a stackable item in legacy
// protocol encoding. The 0xFF marker byte is inserted for 10x+ legacy clients.
func (g *GameProtocol) addLegacyItemCount(w *netmsg.Writer, count uint16) {
	if g.profile.Version >= 1000 {
		w.AddByte(0xFF) // legacy marker
	}
	if count > 1 || (count == 1 && g.profile.Version < 1000) {
		w.AddByte(byte(count))
	}
}

// legacySpeakClass maps modern talk type values to the 8.60 wire format.
func legacySpeakClass(talkType byte) byte {
	switch talkType {
	case 0x01: // say
		return 0x01
	case 0x02: // whisper
		return 0x02
	case 0x03: // yell
		return 0x03
	case 0x04: // private to player (online)
		return 0x04
	case 0x05: // channel (RA/Support)
		return 0x05
	case 0x06: // channel OOC
		return 0x06
	case 0x07: // channel
		return 0x07
	case 0x08: // private NPC
		return 0x0A
	case 0x09: // private to player (offline)
		return 0x14
	case 0x0B: // broadcast
		return 0x0B
	default:
		return talkType
	}
}

// legacyMessageClass maps modern message type values to the 8.60 wire format.
func legacyMessageClass(msgType byte) byte {
	switch msgType {
	case 0x11: // status default
		return 0x11
	case 0x12: // status warning
		return 0x12
	case 0x13: // market
		return 0x13
	case 0x14: // heal
		return 0x14
	case 0x15: // advance
		return 0x15
	case 0x16: // damage received
		return 0x16
	case 0x17: // damage dealt
		return 0x17
	case 0x18: // event default
		return 0x18
	case 0x19: // event guild
		return 0x0E
	case 0x1A: // event party
		return 0x0D
	case 0x1C: // log
		return 0x1C
	case 0x20: // hotkey use
		return 0x20
	default:
		return msgType
	}
}
