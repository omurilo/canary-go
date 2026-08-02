package game

import "github.com/omurilo/canary-go/internal/creatures"

// The per-player shop registry, ported from src/creatures/npcs/npc.cpp.
//
// An NPC's shop is not necessarily the list on its type. A script can install a
// different list for one player — that is how a quest NPC sells something only
// to whoever finished the quest — and the NPC has to remember which players it
// has an open window with so it can close them all when it walks away or the
// player leaves.
//
// None of that existed here: the shop was read straight off the type for
// everyone, and a trade window stayed open until the client closed it.

// GetShopItemVector is Npc::getShopItemVector (npc.cpp:512): the player's own
// list if a script installed a non-empty one, otherwise the type's.
//
// The emptiness check is upstream's and is load-bearing: registering a player
// with an empty list must not hide the default shop from them.
func (n *Npc) GetShopItemVector(playerGUID uint32) []creatures.ShopItem {
	if playerGUID != 0 {
		if items, ok := n.shopPlayers[playerGUID]; ok && len(items) > 0 {
			return items
		}
	}
	if n.Type == nil {
		return nil
	}
	return n.Type.ShopItems
}

// IsShopPlayer is Npc::isShopPlayer (npc.cpp:1230): whether this player has a
// trade window open with the NPC.
func (n *Npc) IsShopPlayer(playerGUID uint32) bool {
	_, ok := n.shopPlayers[playerGUID]
	return ok
}

// AddShopPlayer is Npc::addShopPlayer (npc.cpp:1234).
//
// try_emplace, not assignment: a second call for a player who already has a
// window leaves the first list in place. Overwriting would let a re-opened
// window swap the shop out from under an in-flight purchase.
func (n *Npc) AddShopPlayer(playerGUID uint32, shopItems []creatures.ShopItem) {
	if n.shopPlayers == nil {
		n.shopPlayers = make(map[uint32][]creatures.ShopItem)
	}
	if _, exists := n.shopPlayers[playerGUID]; exists {
		return
	}
	n.shopPlayers[playerGUID] = shopItems
}

// RemoveShopPlayer is Npc::removeShopPlayer (npc.cpp:1238).
func (n *Npc) RemoveShopPlayer(playerGUID uint32) { delete(n.shopPlayers, playerGUID) }

// CloseAllShopWindows is Npc::closeAllShopWindows (npc.cpp:1242): tell every
// player with an open window that it is gone, then forget them.
//
// It runs when the NPC wanders out of interaction range and on the despawn
// path, which is the only thing that stops a player keeping a trade window open
// from the other side of town.
func (n *Npc) CloseAllShopWindows(w *World) {
	if len(n.shopPlayers) == 0 {
		return
	}
	if w != nil && w.OnCloseShopWindow != nil {
		for guid := range n.shopPlayers {
			if p := w.PlayerByDBID(guid); p != nil {
				w.OnCloseShopWindow(p)
			}
		}
	}
	n.shopPlayers = nil
}
