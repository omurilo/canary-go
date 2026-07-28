package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	lua "github.com/yuin/gopher-lua"
)

// luaNetworkMessage backs the Lua NetworkMessage class used by scripts that
// build/parse raw client packets (custom UI windows, imbuement dialogs, the NPC
// dialog list, etc.). Writing is fully wired; sendToPlayer routes the buffered
// bytes to the player's session. Reading (get*) works when the message was
// seeded from received bytes, otherwise returns zero values.
type luaNetworkMessage struct {
	w *netmsg.Writer
	r *netmsg.Reader
}

const networkMessageTypeName = "NetworkMessage"

func (e *Engine) registerNetworkMessage() {
	mt := e.L.NewTypeMetatable(networkMessageTypeName)
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), networkMessageMethods))
	e.setClassConstructor(networkMessageTypeName, networkMessageConstructor, networkMessageMethods)
}

func networkMessageConstructor(L *lua.LState) int {
	m := &luaNetworkMessage{w: netmsg.NewWriter()}
	ud := L.NewUserData()
	ud.Value = m
	L.SetMetatable(ud, L.GetTypeMetatable(networkMessageTypeName))
	L.Push(ud)
	return 1
}

func checkNetworkMessage(L *lua.LState) *luaNetworkMessage {
	ud := L.CheckUserData(1)
	if m, ok := ud.Value.(*luaNetworkMessage); ok {
		return m
	}
	L.ArgError(1, "NetworkMessage expected")
	return nil
}

var networkMessageMethods = map[string]lua.LGFunction{
	"addByte": func(L *lua.LState) int {
		if m := checkNetworkMessage(L); m != nil {
			m.w.AddByte(byte(luaOptInt(L, 2)))
		}
		return 0
	},
	"addU16": networkMessageAddU16,
	"addU32": networkMessageAddU32,
	"addU64": func(L *lua.LState) int {
		if m := checkNetworkMessage(L); m != nil {
			m.w.AddU64(uint64(L.CheckNumber(2)))
		}
		return 0
	},
	// Canary aliases add8bit/add16bit/... to the sized adders.
	"add8bit":  networkMessageAddByte8,
	"add16bit": networkMessageAddU16,
	"add32bit": networkMessageAddU32,
	"addString": func(L *lua.LState) int {
		if m := checkNetworkMessage(L); m != nil {
			m.w.AddString(L.CheckString(2))
		}
		return 0
	},
	"addPosition": func(L *lua.LState) int {
		m := checkNetworkMessage(L)
		if m == nil {
			return 0
		}
		pos := checkPosition(L, 2)
		m.w.AddPosition(netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z})
		return 0
	},
	"getByte": func(L *lua.LState) int {
		m := checkNetworkMessage(L)
		if m == nil || m.r == nil || m.r.Remaining() < 1 {
			L.Push(lua.LNumber(0))
			return 1
		}
		L.Push(lua.LNumber(m.r.GetByte()))
		return 1
	},
	"getU16": networkMessageGetU16,
	"getU32": networkMessageGetU32,
	"getString": func(L *lua.LState) int {
		m := checkNetworkMessage(L)
		if m == nil || m.r == nil {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(m.r.GetString()))
		return 1
	},
	"getUnreadBytes": func(L *lua.LState) int {
		m := checkNetworkMessage(L)
		if m == nil || m.r == nil {
			L.Push(lua.LNumber(0))
			return 1
		}
		L.Push(lua.LNumber(m.r.Remaining()))
		return 1
	},
	"skipBytes": func(L *lua.LState) int {
		if m := checkNetworkMessage(L); m != nil && m.r != nil {
			m.r.Skip(luaOptInt(L, 2))
		}
		return 0
	},
	"sendToPlayer": func(L *lua.LState) int {
		m := checkNetworkMessage(L)
		if m == nil {
			return 0
		}
		p := networkMessagePlayerArg(L, 2)
		if p != nil && p.Session != nil {
			out := netmsg.NewWriter()
			out.AddBytes(m.w.Bytes())
			if out.Len() == 0 {
				return 0
			}
			p.Session.SendToClient(out)
		}
		return 0
	},
	"delete": func(L *lua.LState) int {
		if m := checkNetworkMessage(L); m != nil {
			m.w = netmsg.NewWriter()
			m.r = nil
		}
		return 0
	},
}

func networkMessageAddByte8(L *lua.LState) int {
	if m := checkNetworkMessage(L); m != nil {
		m.w.AddByte(byte(luaOptInt(L, 2)))
	}
	return 0
}

func networkMessageAddU16(L *lua.LState) int {
	if m := checkNetworkMessage(L); m != nil {
		m.w.AddU16(uint16(luaOptInt(L, 2)))
	}
	return 0
}

func networkMessageAddU32(L *lua.LState) int {
	if m := checkNetworkMessage(L); m != nil {
		m.w.AddU32(uint32(L.CheckNumber(2)))
	}
	return 0
}

func networkMessageGetU16(L *lua.LState) int {
	m := checkNetworkMessage(L)
	if m == nil || m.r == nil || m.r.Remaining() < 2 {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(m.r.GetU16()))
	return 1
}

func networkMessageGetU32(L *lua.LState) int {
	m := checkNetworkMessage(L)
	if m == nil || m.r == nil || m.r.Remaining() < 4 {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(m.r.GetU32()))
	return 1
}

// networkMessagePlayerArg resolves a *game.Player from a userdata argument.
func networkMessagePlayerArg(L *lua.LState, n int) *game.Player {
	ud, ok := L.Get(n).(*lua.LUserData)
	if !ok {
		return nil
	}
	if p, ok := ud.Value.(*game.Player); ok {
		return p
	}
	return nil
}
