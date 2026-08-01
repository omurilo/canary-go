package protocol

import "testing"

// OTC changed the shape it reads for 0xF2 at client version 1291
// (otclient/src/client/protocolgameparse.cpp:979). Upstream Canary sends the
// short form to everyone, which is why an OTCR login died with
//
//	parse message exception ... last opcode: 0xF2 (242) ... eof reached
//
// and took the cyclopedia town list down with it.
func TestCoinBalanceExtendedGate(t *testing.T) {
	cases := []struct {
		name     string
		os       uint16
		version  uint16
		extended bool
	}{
		{"official client, modern version", 2, 1525, false},
		{"official client at the OTC boundary version", 2, 1291, false},
		{"otclient linux, modern version", 10, 1525, true},
		{"otclient windows at the boundary", 11, 1291, true},
		{"otclient mac below the boundary", 12, 1290, false},
		{"otclient at an old version", 10, 1100, false},
	}

	for _, c := range cases {
		g := &GameProtocol{clientOS: c.os, clientVersion: c.version}
		if got := g.isOTCCoinBalanceExtended(); got != c.extended {
			t.Errorf("%s (os=%d version=%d): extended = %v, want %v",
				c.name, c.os, c.version, got, c.extended)
		}
	}
}

// The long 0xF2 must not be tied to isOTCR. That flag also gates the creature
// shader and attached-effect bytes, which the client only reads after the 0x43
// feature frame Go does not send yet — turning it on to fix the login would
// corrupt every creature description instead.
func TestCoinBalanceDoesNotDependOnOTCRFeatures(t *testing.T) {
	g := &GameProtocol{clientOS: 11, clientVersion: 1525}
	if g.isOTCR() {
		t.Fatal("isOTCR must stay false until sendOTCRFeatures (0x43) is ported")
	}
	if !g.isOTCCoinBalanceExtended() {
		t.Error("an OTC client must still get the long 0xF2 while isOTCR is false")
	}
}
