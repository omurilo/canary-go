package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// The client drains every opcode of ONE message and then hangs up
// unconditionally (otclient/modules/gamelib/protocollogin.lua:156-186):
//
//	function ProtocolLogin:onRecv(msg)
//	    while not msg:eof() do ... end
//	    self:disconnect()
//	end
//
// MOTD, session key and character list therefore have to share a single
// message, which is what getCharacterList does — one OutputMessage, one send()
// (protocollogin.cpp:60-148). Go sent three, so with a MOTD configured the
// client read the MOTD, disconnected, and never saw the list: login "worked"
// and the character list was empty.
func TestLoginReplyIsOneMessage(t *testing.T) {
	p := &LoginProtocol{deps: &Deps{Cfg: &config.Config{
		ServerName: "Canary-Go", IP: "127.0.0.1", GamePort: 7172, MOTD: "Welcome",
	}}}
	acc := &db.Account{ID: 1, PremDays: 0}
	chars := []db.Character{{Name: "Gm Test"}, {Name: "Second"}}

	w := netmsg.NewWriter()
	p.writeMOTD(w)
	p.writeSessionKey(w, "acc", "pw")
	p.writeCharacterList(w, acc, chars)

	r := netmsg.NewReader(w.Bytes())

	if op := r.GetByte(); op != opLoginMOTD {
		t.Fatalf("first opcode = 0x%02X, want MOTD 0x%02X", op, opLoginMOTD)
	}
	if motd := r.GetString(); motd != "1\nWelcome" {
		t.Errorf("motd = %q", motd)
	}

	if op := r.GetByte(); op != opLoginSessionKey {
		t.Fatalf("second opcode = 0x%02X, want session key 0x%02X — the three parts "+
			"must share one message", op, opLoginSessionKey)
	}
	if key := r.GetString(); key != "acc\npw" {
		t.Errorf("session key = %q", key)
	}

	if op := r.GetByte(); op != opLoginCharList {
		t.Fatalf("third opcode = 0x%02X, want char list 0x%02X", op, opLoginCharList)
	}

	// World list, then the characters (protocollogin.cpp:125-141).
	if n := r.GetByte(); n != 1 {
		t.Errorf("worlds = %d, want 1", n)
	}
	r.GetByte() // world id
	if name := r.GetString(); name != "Canary-Go" {
		t.Errorf("world name = %q", name)
	}
	if ip := r.GetString(); ip != "127.0.0.1" {
		t.Errorf("world ip = %q", ip)
	}
	if port := r.GetU16(); port != 7172 {
		t.Errorf("world port = %d, want 7172", port)
	}
	r.GetByte() // preview

	n := r.GetByte()
	if int(n) != len(chars) {
		t.Fatalf("characters = %d, want %d", n, len(chars))
	}
	for i := 0; i < int(n); i++ {
		r.GetByte() // world id
		if got := r.GetString(); got != chars[i].Name {
			t.Errorf("character %d = %q, want %q", i, got, chars[i].Name)
		}
	}

	r.GetByte() // premium days
	r.GetByte() // is premium
	r.GetU32()  // premium last day

	if r.Remaining() != 0 {
		t.Errorf("%d trailing byte(s) after the character list", r.Remaining())
	}
}

// With no MOTD configured the message starts at the session key, and the list
// still has to be in it.
func TestLoginReplyWithoutMOTD(t *testing.T) {
	p := &LoginProtocol{deps: &Deps{Cfg: &config.Config{
		ServerName: "Canary-Go", IP: "127.0.0.1", GamePort: 7172,
	}}}

	w := netmsg.NewWriter()
	p.writeMOTD(w)
	p.writeSessionKey(w, "acc", "pw")
	p.writeCharacterList(w, &db.Account{ID: 1}, []db.Character{{Name: "Only"}})

	r := netmsg.NewReader(w.Bytes())
	if op := r.GetByte(); op != opLoginSessionKey {
		t.Fatalf("first opcode = 0x%02X, want session key 0x%02X", op, opLoginSessionKey)
	}
}
