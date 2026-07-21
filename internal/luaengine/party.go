package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

const partyTypeName = "Party"

func (e *Engine) pushParty(L *lua.LState, pt *game.Party) {
	if pt == nil {
		L.Push(lua.LNil)
		return
	}
	ud := L.NewUserData()
	ud.Value = pt
	L.SetMetatable(ud, L.GetTypeMetatable(partyTypeName))
	L.Push(ud)
}

func checkParty(L *lua.LState) *game.Party {
	ud := L.CheckUserData(1)
	if pt, ok := ud.Value.(*game.Party); ok {
		return pt
	}
	L.ArgError(1, "Party expected")
	return nil
}

// checkPlayerArg resolves a Player userdata at arg n.
func checkPlayerArg(L *lua.LState, n int) *game.Player {
	if ud, ok := L.Get(n).(*lua.LUserData); ok {
		if p, ok := ud.Value.(*game.Player); ok {
			return p
		}
	}
	return nil
}

func (e *Engine) registerParty() {
	methods := map[string]lua.LGFunction{
		"getLeader": func(L *lua.LState) int {
			pt := checkParty(L)
			if pt == nil || pt.Leader() == nil {
				L.Push(lua.LNil)
				return 1
			}
			ud := L.NewUserData()
			ud.Value = pt.Leader()
			L.SetMetatable(ud, L.GetTypeMetatable("Player"))
			L.Push(ud)
			return 1
		},
		"getMembers": func(L *lua.LState) int {
			pt := checkParty(L)
			tbl := L.NewTable()
			if pt != nil {
				for i, m := range pt.Members() {
					ud := L.NewUserData()
					ud.Value = m
					L.SetMetatable(ud, L.GetTypeMetatable("Player"))
					tbl.RawSetInt(i+1, ud)
				}
			}
			L.Push(tbl)
			return 1
		},
		"getMemberCount": func(L *lua.LState) int {
			pt := checkParty(L)
			if pt == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(pt.MemberCount()))
			return 1
		},
		"getInviteeCount": func(L *lua.LState) int {
			pt := checkParty(L)
			if pt == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(len(pt.Invitees())))
			return 1
		},
		"addMember": func(L *lua.LState) int {
			pt := checkParty(L)
			p := checkPlayerArg(L, 2)
			L.Push(lua.LBool(pt != nil && p != nil && pt.Join(p)))
			return 1
		},
		"removeMember": func(L *lua.LState) int {
			pt := checkParty(L)
			p := checkPlayerArg(L, 2)
			L.Push(lua.LBool(pt != nil && p != nil && pt.Leave(p)))
			return 1
		},
		"addInvite": func(L *lua.LState) int {
			pt := checkParty(L)
			p := checkPlayerArg(L, 2)
			L.Push(lua.LBool(pt != nil && p != nil && pt.Invite(p)))
			return 1
		},
		"removeInvite": func(L *lua.LState) int {
			pt := checkParty(L)
			p := checkPlayerArg(L, 2)
			L.Push(lua.LBool(pt != nil && p != nil && pt.Revoke(p)))
			return 1
		},
		"setLeader": func(L *lua.LState) int {
			pt := checkParty(L)
			p := checkPlayerArg(L, 2)
			L.Push(lua.LBool(pt != nil && p != nil && pt.PassLeadership(p)))
			return 1
		},
		"disband": func(L *lua.LState) int {
			if pt := checkParty(L); pt != nil {
				pt.Disband()
			}
			L.Push(lua.LTrue)
			return 1
		},
		"setSharedExperience": func(L *lua.LState) int {
			pt := checkParty(L)
			if pt != nil {
				pt.SetSharedExperience(luaOptBool(L, 2))
			}
			L.Push(lua.LTrue)
			return 1
		},
		"isSharedExperienceActive": func(L *lua.LState) int {
			pt := checkParty(L)
			L.Push(lua.LBool(pt != nil && pt.IsSharedExperienceActive()))
			return 1
		},
		"isSharedExperienceEnabled": func(L *lua.LState) int {
			pt := checkParty(L)
			L.Push(lua.LBool(pt != nil && pt.IsSharedExperienceEnabled()))
			return 1
		},
		"shareExperience": func(L *lua.LState) int {
			pt := checkParty(L)
			if pt != nil {
				pt.ShareExperience(uint64(luaOptInt(L, 2)))
			}
			L.Push(lua.LTrue)
			return 1
		},
	}

	mt := e.L.NewTypeMetatable(partyTypeName)
	idx := e.L.NewTable()
	e.L.SetFuncs(idx, methods)
	e.L.SetField(mt, "__index", idx)

	e.setClassConstructor(partyTypeName, e.partyCreate, methods)
}

// partyCreate implements Party(player): creates a party led by the player, or
// returns nil when they are already in one. Mirrors luaPartyCreate.
func (e *Engine) partyCreate(L *lua.LState) int {
	p := checkPlayerArg(L, 2)
	if p == nil || p.Party != nil {
		L.Push(lua.LNil)
		return 1
	}
	pt := game.NewParty(p, e.world)
	if e.world != nil {
		e.world.UpdatePlayerShield(p)
	}
	e.pushParty(L, pt)
	return 1
}
