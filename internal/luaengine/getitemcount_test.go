package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	lua "github.com/yuin/gopher-lua"
)

// player:getItemCount(id) was a stub returning HazardPoints, so any script
// gated on carrying an item always took the "no item" branch — the Dreamers
// carrot crossing (getItemCount(3595) > 0) never fired and just bounced the
// player with fire damage. It must count the item across the inventory tree.
func TestLuaGetItemCountCountsInventory(t *testing.T) {
	e := newTestEngine()
	defer e.Close()

	p := &game.Player{Name: "T", MaxHealth: 100, Health: 100}
	var inv [11]*game.Item
	inv[3] = &game.Item{ID: 3595, Count: 1} // a carrot in the backpack slot
	p.Inventory = inv

	ud := e.L.NewUserData()
	ud.Value = p
	e.L.SetMetatable(ud, e.L.GetTypeMetatable("Player"))
	e.L.Push(ud)
	e.L.Push(lua.LNumber(3595)) // arg 2: item id

	if n := playerGetitemcount(e.L); n != 1 {
		t.Fatalf("getItemCount returned %d Lua values, want 1", n)
	}
	if v := e.L.Get(-1); v != lua.LNumber(1) {
		t.Errorf("getItemCount(carrot) = %v, want 1 (player carries one)", v)
	}

	// An item the player does not carry reports 0.
	ud2 := e.L.NewUserData()
	ud2.Value = p
	e.L.SetMetatable(ud2, e.L.GetTypeMetatable("Player"))
	e.L.SetTop(0)
	e.L.Push(ud2)
	e.L.Push(lua.LNumber(12345))
	playerGetitemcount(e.L)
	if v := e.L.Get(-1); v != lua.LNumber(0) {
		t.Errorf("getItemCount(missing) = %v, want 0", v)
	}
}