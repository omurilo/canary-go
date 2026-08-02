package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// recordingSession captures the last modal window handed to SendModalWindow so
// the test can assert the engine actually dispatched it.
type recordingSession struct {
	game.Session
	last *game.ModalWindow
}

func (r *recordingSession) SendModalWindow(m *game.ModalWindow) { r.last = m }

// TestModalWindowMethods is the modal_window_helper.lua bug: the constructor
// attaches GetTypeMetatable("ModalWindow") to the userdata, but that metatable
// was never created (registerModalWindowType only set up the _ClassCtor sugar),
// so method calls failed with "attempt to index a non-table object(userdata)".
func TestModalWindowMethods(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		local mw = ModalWindow(1, "Title", "Message")
		assert(type(mw) == "userdata", "expected userdata, got " .. type(mw))
		mw:setPriority(true)
		mw:addButton(1, "Ok")
		mw:addButton(2, "Cancel")
		mw:addChoice(1, "Option")
		mw:setDefaultEnterButton(1)
		mw:setDefaultEscapeButton(2)
	`); err != nil {
		t.Fatalf("modal window methods failed: %v", err)
	}
}

// TestModalWindowFields checks the datapack's setter/getter round trip used by
// reward.lua and hireling flows before sendToPlayer.
func TestModalWindowFields(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		local mw = ModalWindow(7, "Hello", "World")
		mw:setPriority(false)
		mw:addButton(1, "Go")
		mw:setDefaultEnterButton(1)
		mw:setDefaultEscapeButton(1)
		-- sendToPlayer requires a real player session; not exercised here.
		assert(mw ~= nil)
	`); err != nil {
		t.Fatalf("modal window field setup failed: %v", err)
	}
}

// guard against accidentally nilling the userdata type metatable when the
// constructor global is rebound by the datapack's ModalWindow table.
func TestModalWindowGlobalIsTable(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	global := e.L.GetGlobal("ModalWindow")
	if global.Type() != lua.LTTable {
		t.Fatalf("ModalWindow global should be a table, got %s", global.Type())
	}
	mt := e.L.GetTypeMetatable(modalWindowTypeName)
	if mt.Type() != lua.LTTable {
		t.Fatalf("ModalWindow type metatable missing, got %s", mt.Type())
	}
}

// TestModalWindowSendToPlayer is the reward.lua bug: the datapack helper ends
// with `return modalWindow:sendToPlayer(player)` on the engine userdata, which
// had no sendToPlayer method ("attempt to call a non-function object").
func TestModalWindowSendToPlayer(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	p := &game.Player{Name: "Player" + "X"}
	session := &recordingSession{}
	p.Session = session
	e.pushPlayerUserdata(p)
	e.L.SetGlobal("p", e.L.Get(-1))
	e.L.Pop(1)

	if err := e.L.DoString(`
		local mw = ModalWindow(1, "Reward", "Choose")
		mw:addButton(1, "Select")
		mw:setPriority(true)
		local ok = mw:sendToPlayer(p)
		assert(ok == true, "expected true from sendToPlayer")
	`); err != nil {
		t.Fatalf("modalWindow:sendToPlayer failed: %v", err)
	}

	if session.last == nil {
		t.Fatal("session never received the modal window")
	}
	if session.last.Title != "Reward" || session.last.ID != 1 {
		t.Fatalf("wrong window delivered: id=%d title=%q", session.last.ID, session.last.Title)
	}
}

// TestModalWindowSendToPlayerMissingPlayer: upstream pushes nil when there is no
// valid player; the engine should too, not panic.
func TestModalWindowSendToPlayerMissingPlayer(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	if err := e.L.DoString(`
		local mw = ModalWindow(1, "T", "M")
		local ok = mw:sendToPlayer(nil)
		assert(ok == nil, "expected nil for missing player")
	`); err != nil {
		t.Fatalf("sendToPlayer with nil player failed: %v", err)
	}
}
