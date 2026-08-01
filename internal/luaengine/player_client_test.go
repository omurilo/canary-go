package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// fakeSession reports a client family and version, the way GameProtocol does from
// what the client announced at login.
type fakeSession struct {
	game.Session
	os      uint16
	version uint16
}

func (f fakeSession) ClientOS() uint16      { return f.os }
func (f fakeSession) ClientVersion() uint16 { return f.version }

// player:getClient().os used to be hardcoded to 2, so every client looked like a
// stock Windows one and Player.isUsingOtClient() was always false — or, read the
// other way, no script could tell the families apart. That is what let the gamestore
// send OTClient-only trailing bytes to the official client and crash it with
// "Unknown Gameserver Message: 0".
func TestPlayerGetClientReportsTheAnnouncedOS(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	cases := []struct {
		name    string
		os      uint16
		version uint16
		isOTC   bool
	}{
		{"stock windows client", 2, 1525, false},
		{"otclient linux", 10, 1525, true},
		{"otclient windows", 11, 1525, true},
		{"otclient mac", 12, 1525, true},
	}
	for _, tc := range cases {
		// Set the session directly: AddPlayer refuses a duplicate name, which would
		// leave later cases with a nil session and silently test the fallback.
		p := &game.Player{Name: tc.name}
		p.Session = fakeSession{os: tc.os, version: tc.version}
		e.pushPlayerUserdata(p)
		e.L.SetGlobal("p", e.L.Get(-1))
		e.L.Pop(1)

		if err := e.L.DoString(`
			c = p:getClient()
			gotOS, gotVersion = c.os, c.version
			isOtc = c.os >= CLIENTOS_OTCLIENT_LINUX
		`); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := e.L.GetGlobal("gotOS"); got != lua.LNumber(tc.os) {
			t.Errorf("%s: os = %v, want %d", tc.name, got, tc.os)
		}
		if got := e.L.GetGlobal("gotVersion"); got != lua.LNumber(tc.version) {
			t.Errorf("%s: version = %v, want %d", tc.name, got, tc.version)
		}
		if got := e.L.GetGlobal("isOtc"); (got == lua.LTrue) != tc.isOTC {
			t.Errorf("%s: isUsingOtClient would be %v, want %v", tc.name, got, tc.isOTC)
		}
	}
}

// A session that cannot report (a test harness, a legacy profile) falls back rather
// than panicking, and the fallback is the stock client — the conservative choice,
// since it withholds the extension bytes instead of sending them blindly.
func TestPlayerGetClientFallsBack(t *testing.T) {
	e := newTestEngine()
	defer e.Close()
	p := &game.Player{Name: "NoSession"}
	e.pushPlayerUserdata(p)
	e.L.SetGlobal("p", e.L.Get(-1))
	e.L.Pop(1)
	if err := e.L.DoString(`
		local c = p:getClient()
		assert(c ~= nil, "getClient must return a table")
		assert(c.os < CLIENTOS_OTCLIENT_LINUX, "the fallback must not claim to be OTC")
		assert(c.version > 0)
	`); err != nil {
		t.Fatalf("%v", err)
	}
}
