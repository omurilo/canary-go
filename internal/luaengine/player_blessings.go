package luaengine

import (
	lua "github.com/yuin/gopher-lua"
)

// playerSendblessingsdialog sends the blessing status window to the client.
// Usage: player:sendBlessingsDialog()
func playerSendblessingsdialog(L *lua.LState) int {
	p := checkPlayer(L)
	if p == nil || p.Session == nil {
		return 0
	}
	
	// Call SendBlessingsDialog via Session interface
	if session, ok := p.Session.(interface{ SendBlessingsDialog() }); ok {
		session.SendBlessingsDialog()
	}
	
	return 0
}
