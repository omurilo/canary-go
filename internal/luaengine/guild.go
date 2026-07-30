package luaengine

import (
	"github.com/opentibiabr/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) registerGuildType() {
	mt := e.L.NewTypeMetatable("Guild")
	methods := map[string]lua.LGFunction{
		"getId":             guildGetid,
		"getName":           guildGetname,
		"getMembersOnline":  guildGetmembersonline,
		"addRank":           guildAddrank,
		"getRankById":       guildGetrankbyid,
		"getRankByLevel":    guildGetrankbylevel,
		"getMotd":           guildGetmotd,
		"setMotd":           guildSetmotd,
		"getBankBalance":    guildGetbankbalance,
		"setBankBalance":    guildSetbankbalance,
		"addMember":         guildAddmember,
		"removeMember":      guildRemovemember,
		"getMemberCount":    guildGetmembercount,
	}
	e.L.SetField(mt, "__index", e.L.SetFuncs(e.L.NewTable(), methods))

	// The global must be a callable TABLE, not a plain function: the datapack both
	// calls Guild(id) and *extends* the class (data/libs/compat/compat.lua:1496
	// `function Guild.addMember(self, player)`). A bare function global makes that
	// assignment fail with "attempt to index a non-table object (function)".
	e.setClassConstructor("Guild", func(L *lua.LState) int {
		id := uint32(L.CheckInt(1))
		guild := e.world.GetGuild(id)
		if guild == nil {
			L.Push(lua.LNil)
			return 1
		}
		ud := L.NewUserData()
		ud.Value = guild
		L.SetMetatable(ud, L.GetTypeMetatable("Guild"))
		L.Push(ud)
		return 1
	}, methods)
}

func checkGuild(L *lua.LState) *game.Guild {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*game.Guild); ok {
		return v
	}
	L.ArgError(1, "Guild expected")
	return nil
}

func guildGetid(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(g.ID))
	return 1
}

func guildGetname(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(g.Name))
	return 1
}

func guildGetmembersonline(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LNil)
		return 1
	}
	
	members := g.GetMembersOnline()
	tbl := L.NewTable()
	for i, player := range members {
		ud := L.NewUserData()
		ud.Value = player
		L.SetMetatable(ud, L.GetTypeMetatable("Player"))
		tbl.RawSetInt(i+1, ud)
	}
	L.Push(tbl)
	return 1
}

func guildAddrank(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	
	id := uint32(L.CheckInt(2))
	name := L.CheckString(3)
	level := uint8(L.CheckInt(4))
	
	g.AddRank(id, name, level)
	L.Push(lua.LBool(true))
	return 1
}

func guildGetrankbyid(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LNil)
		return 1
	}
	
	id := uint32(L.CheckInt(2))
	rank := g.GetRankByID(id)
	if rank == nil {
		L.Push(lua.LNil)
		return 1
	}
	
	tbl := L.NewTable()
	L.SetField(tbl, "id", lua.LNumber(rank.ID))
	L.SetField(tbl, "name", lua.LString(rank.Name))
	L.SetField(tbl, "level", lua.LNumber(rank.Level))
	L.Push(tbl)
	return 1
}

func guildGetrankbylevel(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LNil)
		return 1
	}
	
	level := uint8(L.CheckInt(2))
	rank := g.GetRankByLevel(level)
	if rank == nil {
		L.Push(lua.LNil)
		return 1
	}
	
	tbl := L.NewTable()
	L.SetField(tbl, "id", lua.LNumber(rank.ID))
	L.SetField(tbl, "name", lua.LString(rank.Name))
	L.SetField(tbl, "level", lua.LNumber(rank.Level))
	L.Push(tbl)
	return 1
}

func guildGetmotd(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(g.GetMOTD()))
	return 1
}

func guildSetmotd(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		return 0
	}
	motd := L.CheckString(2)
	g.SetMOTD(motd)
	return 0
}

func guildGetbankbalance(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(g.GetBankBalance()))
	return 1
}

func guildSetbankbalance(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		return 0
	}
	balance := uint64(L.CheckNumber(2))
	g.SetBankBalance(balance)
	return 0
}

func guildAddmember(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	
	g.AddMember(p)
	L.Push(lua.LBool(true))
	return 1
}

func guildRemovemember(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	
	p := checkPlayer(L)
	if p == nil {
		L.Push(lua.LBool(false))
		return 1
	}
	
	g.RemoveMember(p)
	L.Push(lua.LBool(true))
	return 1
}

func guildGetmembercount(L *lua.LState) int {
	g := checkGuild(L)
	if g == nil {
		L.Push(lua.LNumber(0))
		return 1
	}
	L.Push(lua.LNumber(g.MemberCount))
	return 1
}
