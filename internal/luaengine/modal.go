package luaengine

import (
	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const modalWindowTypeName = "ModalWindow"

// LuaModalWindow wraps a game.ModalWindow for Lua userdata.
type LuaModalWindow struct {
	*game.ModalWindow
}

func checkModalWindow(L *lua.LState) *game.ModalWindow {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*LuaModalWindow); ok {
		return v.ModalWindow
	}
	L.ArgError(1, "ModalWindow expected")
	return nil
}

// registerModalWindowType registers the ModalWindow userdata type and its
// global constructor so Lua scripts can create and send modal windows.
func (e *Engine) registerModalWindowType() {
	methods := map[string]lua.LGFunction{
		"addButton":              e.modalWindowAddButton,
		"addChoice":              e.modalWindowAddChoice,
		"setDefaultEscapeButton": e.modalWindowSetDefaultEscapeButton,
		"setDefaultEnterButton":  e.modalWindowSetDefaultEnterButton,
		"setPriority":            e.modalWindowSetPriority,
	}
	e.setClassConstructor("ModalWindow", e.modalWindowConstructor, methods)
}

// modalWindowConstructor creates a new ModalWindow.
// Lua: ModalWindow(id, title, message)
func (e *Engine) modalWindowConstructor(L *lua.LState) int {
	id := uint32(L.CheckInt(2))
	title := L.CheckString(3)
	message := L.CheckString(4)
	mw := &LuaModalWindow{
		ModalWindow: &game.ModalWindow{
			ID:      id,
			Title:   title,
			Message: message,
		},
	}
	ud := L.NewUserData()
	ud.Value = mw
	L.SetMetatable(ud, L.GetTypeMetatable(modalWindowTypeName))
	L.Push(ud)
	return 1
}

// modalWindowAddButton adds a button to the modal window.
// Lua: modal:addButton(id, text)
func (e *Engine) modalWindowAddButton(L *lua.LState) int {
	mw := checkModalWindow(L)
	if mw == nil {
		return 0
	}
	id := uint8(L.CheckInt(2))
	text := L.CheckString(3)
	mw.Buttons = append(mw.Buttons, game.ModalButton{ID: id, Text: text})
	return 0
}

// modalWindowAddChoice adds a choice to the modal window.
// Lua: modal:addChoice(id, text)
func (e *Engine) modalWindowAddChoice(L *lua.LState) int {
	mw := checkModalWindow(L)
	if mw == nil {
		return 0
	}
	id := uint8(L.CheckInt(2))
	text := L.CheckString(3)
	mw.Choices = append(mw.Choices, game.ModalChoice{ID: id, Text: text})
	return 0
}

// modalWindowSetDefaultEscapeButton sets the default escape button.
// Lua: modal:setDefaultEscapeButton(id)
func (e *Engine) modalWindowSetDefaultEscapeButton(L *lua.LState) int {
	mw := checkModalWindow(L)
	if mw == nil {
		return 0
	}
	mw.DefaultEscapeButton = uint8(L.CheckInt(2))
	return 0
}

// modalWindowSetDefaultEnterButton sets the default enter button.
// Lua: modal:setDefaultEnterButton(id)
func (e *Engine) modalWindowSetDefaultEnterButton(L *lua.LState) int {
	mw := checkModalWindow(L)
	if mw == nil {
		return 0
	}
	mw.DefaultEnterButton = uint8(L.CheckInt(2))
	return 0
}

// modalWindowSetPriority sets the priority flag.
// Lua: modal:setPriority(bool)
func (e *Engine) modalWindowSetPriority(L *lua.LState) int {
	mw := checkModalWindow(L)
	if mw == nil {
		return 0
	}
	mw.Priority = L.CheckBool(2)
	return 0
}

// playerSendModalWindow sends a modal window to a player.
// Lua: player:sendModalWindow(modalWindow)
func playerSendModalWindow(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil {
		return 0
	}
	modal := checkModalWindow(L)
	if modal == nil {
		return 0
	}

	// Use duck-type interface assertion to avoid a circular import (luaengine
	// cannot import protocol). GameProtocol implements SendModalWindow.
	if session, ok := p.Session.(interface{ SendModalWindow(modal *game.ModalWindow) }); ok {
		session.SendModalWindow(modal)
	}
	return 0
}
