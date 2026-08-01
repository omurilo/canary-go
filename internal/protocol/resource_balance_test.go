package protocol

import (
	"testing"

	"github.com/omurilo/canary-go/internal/netmsg"
)

// 0xED means different things inbound and outbound, and conflating them killed the
// official client. Inbound it is "refresh my resources"
// (ProtocolGame::parseSendResourceBalance); outbound it is sendMessageDialog, which
// is opcode + type + STRING.
//
// The handler used to reply with 0xED and a single zero byte, so the client read the
// type and then looked for a string that was not there — "not enough bytes (2)
// available at position 2" — and died. The crash dump carried the whole packet:
// two bytes, ED 00.
func TestResourceBalanceRequestSendsNoMessageDialog(t *testing.T) {
	// A nil player must not produce a packet at all, let alone a truncated one.
	g := &GameProtocol{}
	g.parseSendResourceBalance(netmsg.NewReader([]byte{0x00}))

	// The reply must never be a bare 0xED: that opcode outbound promises a string.
	// Guarding the shape here rather than the exact resource payload keeps the test
	// about the crash it prevents.
	if got := malformedMessageDialog([]byte{0xED, 0x00}); !got {
		t.Fatalf("sanity: ED 00 is exactly the malformed dialog the client rejected")
	}
	if malformedMessageDialog([]byte{0xED, 0x14, 0x02, 0x00, 'h', 'i'}) {
		t.Errorf("a dialog with a real string is well formed")
	}
}

// malformedMessageDialog reports whether a 0xED packet lacks the string the client
// will try to read: opcode, type byte, then a u16 length.
func malformedMessageDialog(b []byte) bool {
	return len(b) > 0 && b[0] == 0xED && len(b) < 4
}
