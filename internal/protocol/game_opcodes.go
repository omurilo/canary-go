package protocol

import (
	"fmt"
	"github.com/omurilo/canary-go/internal/config"
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/items"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// parseRetrieveDepotSearch handles 0x29 — depot search item request.
func (g *GameProtocol) parseRetrieveDepotSearch(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetU16() // itemId
	g.deps.Log.Debug("depot search", "player", g.player.Name)
}

// parseCyclopediaMonsterTracker handles 0x2A — cyclopedia monster tracker.
func (g *GameProtocol) parseCyclopediaMonsterTracker(r *netmsg.Reader) {
	// No-op for now.
}

// parsePartyAnalyzerAction handles 0x2B — party analyzer action.
func (g *GameProtocol) parsePartyAnalyzerAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	action := r.GetByte()
	_ = action
	g.deps.Log.Debug("party analyzer action", "player", g.player.Name, "action", action)
}

// parseLeaderFinderWindow handles 0x2C — Team Finder leader window actions.
func (g *GameProtocol) parseLeaderFinderWindow(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	action := r.GetByte()
	switch action {
	case 0: // refresh
		g.deps.World.PlayerFindTeam(g.player.ID, 0, 0, 0, false)
	case 1: // close
		g.deps.World.RemoveTeamFinder(g.player.ID)
	case 2: // manage member
		memberID := r.GetU32()
		status := r.GetByte()
		if memberID > 0 {
			g.deps.World.UpdateTeamMemberStatus(g.player.ID, memberID, status)
		}
	case 3: // create
		// Full implementation would parse the creation data from the reader.
		// For now, consume the remaining bytes as a no-op.
		_ = r.GetU16()  // slots
		_ = r.GetU16()  // minLevel
		_ = r.GetU16()  // maxLevel
		_ = r.GetByte() // vocation mask
		_ = r.GetByte() // description length
	}
}

// parseMemberFinderWindow handles 0x2D — Team Finder member window actions.
func (g *GameProtocol) parseMemberFinderWindow(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	action := r.GetByte()
	if action == 0 {
		g.deps.World.SendTeamFinderList(g.player.ID)
	} else {
		leaderID := r.GetU32()
		if action == 1 {
			g.deps.World.JoinTeamFinder(g.player.ID, leaderID)
		} else {
			g.deps.World.LeaveTeamFinder(g.player.ID, leaderID)
		}
	}
}

// parseSetClientOptions handles 0x2E — client display options.
func (g *GameProtocol) parseSetClientOptions(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	g.deps.Log.Debug("client options", "player", g.player.Name, "len", r.Remaining())
}

// parsePlayerTyping handles 0x38 — player typing status.
func (g *GameProtocol) parsePlayerTyping(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetByte() // 1 = typing, 0 = stopped
}

// parseInventoryImbuements handles 0x60 — inventory imbuement data from client.
func (g *GameProtocol) parseInventoryImbuements(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	g.deps.Log.Debug("inventory imbuements", "player", g.player.Name)
}

// parseClientCheck handles 0x63 — client version check.
func (g *GameProtocol) parseClientCheck(r *netmsg.Reader) {
	_ = r.GetU32() // client version
	_ = r.GetU32() // client type
	w := netmsg.NewWriter()
	w.AddByte(0x63)
	w.AddByte(0x00) // OK
	g.SendToClient(w)
}

// parseSetVocation handles 0x6E — Dawnport vocation selection.
func (g *GameProtocol) parseSetVocation(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	vocationID := r.GetByte()
	if vocationID == 0 && g.player.Vocation != 0 {
		return
	}
	g.deps.World.PlayerSetVocation(g.player.ID, vocationID)
}

// parseTeleport handles 0x73 — GM teleport command.
func (g *GameProtocol) parseTeleport(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	pos := r.GetPosition()
	gamePos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
	g.deps.World.PlayerTeleport(g.player.ID, gamePos)
}

// parseStartOfflineTraining handles 0x74 — start offline training.
func (g *GameProtocol) parseStartOfflineTraining(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	skillID := r.GetByte()
	if skillID >= 0 && skillID <= 6 {
		g.player.OfflineTrainingSkill = int8(skillID)
		g.sendStatusText("Offline training started.")
	}
}

// parseContainerAction handles 0x75 — container action.
func (g *GameProtocol) parseContainerAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	action := r.GetByte()
	containerID := r.GetByte()
	_ = containerID
	g.deps.Log.Debug("container action", "action", action, "cid", containerID)
}

// parseHotkeyEquip handles 0x77 — hotkey equip request.
func (g *GameProtocol) parseHotkeyEquip(r *netmsg.Reader) {
	itemID := r.GetU16()
	_ = itemID
}

// parseLookInShop handles 0x79 — look at an item in the shop window.
func (g *GameProtocol) parseLookInShop(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	itemID := r.GetU16()
	count := r.GetByte()
	// EventCallback playerOnLookInShop(player, itemType, count). A false return
	// stops the reply, matching how EventCallback gates the C++ side.
	if g.deps.Events != nil && !g.deps.Events.ExecutePlayerOnLookInShop(g.player, itemID, uint16(count)) {
		return
	}
	npc := g.shopOwner()
	if npc == nil {
		g.sendStatusText("You are not currently trading with anyone.")
		return
	}
	for _, si := range npc.GetShopItemVector(g.player.DBID) {
		if si.ID == itemID {
			g.sendStatusText(si.Name)
			break
		}
	}
	// Npc::onPlayerCheckItem fires the npc's onCheckItem script, which is how a
	// merchant comments on what the player is inspecting. It had no caller.
	npc.OnPlayerCheckItem(g.deps.World, g.player, itemID, count)
}

// parseRequestTrade handles 0x7D — player requests trade with another player.
func (g *GameProtocol) parseRequestTrade(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	pos := r.GetPosition()
	itemID := r.GetU16()
	stackPos := r.GetByte()
	targetPlayerID := r.GetU32()
	// EventCallback playerOnTradeRequest(player, target, item) — (bool), so a false
	// return cancels the request.
	if g.deps.Events != nil {
		target, _ := g.deps.World.CreatureByID(targetPlayerID).(*game.Player)
		if !g.deps.Events.ExecutePlayerOnTradeRequest(g.player, target, nil) {
			return
		}
	}
	g.deps.World.PlayerRequestTrade(g.player.ID, targetPlayerID, game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}, itemID, stackPos)
}

// parseLookInTrade handles 0x7E — look at item in trade window.
func (g *GameProtocol) parseLookInTrade(r *netmsg.Reader) {
	counter := r.GetByte() // 0 = own, 1 = other
	index := r.GetByte()
	_ = counter
	_ = index
}

// parseAcceptTrade handles 0x7F — accept the current trade.
func (g *GameProtocol) parseAcceptTrade(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	// EventCallback playerOnTradeAccept(player, target, item, targetItem). The
	// partner and the two offered items are not tracked separately yet, so they go
	// as nil — the callback signature is still the documented one.
	if g.deps.Events != nil && !g.deps.Events.ExecutePlayerOnTradeAccept(g.player, nil, nil, nil) {
		return
	}
	g.deps.World.PlayerAcceptTrade(g.player.ID)
}

// parseCloseTrade handles 0x80 — close/cancel the current trade.
func (g *GameProtocol) parseCloseTrade(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	g.deps.World.PlayerCloseTrade(g.player.ID)
}

// parseFriendSystemAction handles 0x81 — friend (VIP) system action.
func (g *GameProtocol) parseFriendSystemAction(r *netmsg.Reader) {
	action := r.GetByte()
	_ = action
}

// parseRotateItem handles 0x85 — rotate an item on the map.
func (g *GameProtocol) parseRotateItem(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	pos := r.GetPosition()
	itemID := r.GetU16()
	stackPos := r.GetByte()
	gPos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
	// EventCallback playerOnRotateItem(player, item, position).
	if g.deps.Events != nil {
		var item *game.Item
		if tile := g.deps.World.Map.GetTile(gPos); tile != nil {
			for _, it := range tile.Items {
				if it.ID == itemID {
					item = it
					break
				}
			}
		}
		if !g.deps.Events.ExecutePlayerOnRotateItem(g.player, item, gPos) {
			return
		}
	}
	g.deps.World.PlayerRotateItem(g.player.ID, gPos, itemID, stackPos)
}

// parseConfigureShowOffSocket handles 0x86 — item show-off socket configuration.
func (g *GameProtocol) parseConfigureShowOffSocket(r *netmsg.Reader) {
	_ = r.GetU16() // itemId
}

// parseTextWindow handles 0x89 — text/edit window response.
func (g *GameProtocol) parseTextWindow(r *netmsg.Reader) {
	_ = r.GetU32()    // window id
	_ = r.GetString() // new text
}

// parseHouseWindow handles 0x8A — the player closed a house access-list window
// (ProtocolGame::parseHouseWindow, protocolgame.cpp:2742).
//
// The old body read two bytes and dropped them, which is not even the right
// shape: the packet is a list id byte, then a u32 window id, then the text. So
// every subsequent field was left in the reader.
func (g *GameProtocol) parseHouseWindow(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	listID := r.GetByte()
	windowTextID := r.GetU32()
	text := r.GetString()
	g.player.UpdateHouseWindow(g.deps.World, listID, windowTextID, text)
}

// Wrap/unwrap constants (src/utils/utils_definitions.hpp).
const (
	itemFilledBathTube = 26077
	// isCaskItem ranges: a cask hides its remaining charges across the wrap.
	itemHealthCaskStart, itemHealthCaskEnd = 25879, 25883
	itemManaCaskStart, itemManaCaskEnd     = 25889, 25893
	itemSpiritCaskStart, itemSpiritCaskEnd = 25899, 25902
)

// isCaskItem ports the helper in tools.cpp:1745.
func isCaskItem(id uint16) bool {
	return (id >= itemHealthCaskStart && id <= itemHealthCaskEnd) ||
		(id >= itemManaCaskStart && id <= itemManaCaskEnd) ||
		(id >= itemSpiritCaskStart && id <= itemSpiritCaskEnd)
}

// parseWrapableItem handles 0x8B — wrap a house item into a decoration kit, or
// unwrap a kit back into the item. Ports ProtocolGame::parseWrapableItem and
// Game::playerWrapableItem / wrapItem / unwrapItem.
//
// This read the arguments and threw them away, so an item bought in the store could
// never be unwrapped: the kit just sat there. The field order is position, item id,
// stackpos — reading the id first would have given a garbage position.
func (g *GameProtocol) parseWrapableItem(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	pos := r.GetPosition()
	itemID := r.GetU16()
	stackpos := r.GetByte()
	g.wrapableAt(pos, itemID, stackpos)
}

// wrapRefuse sends the cancel message AND records why, because the cancel packet
// is not in the opcode dump: three sessions were spent guessing which of a dozen
// refusals had fired. The reason is the whole diagnostic value here.
func (g *GameProtocol) wrapRefuse(reason, msg string, pos netmsg.Position, itemID uint16, stackpos uint8) {
	if g.deps != nil && g.deps.Log != nil {
		player := ""
		if g.player != nil {
			player = g.player.Name
		}
		g.deps.Log.Warn("wrap refused", "reason", reason, "player", player,
			"pos", fmt.Sprintf("%d,%d,%d", pos.X, pos.Y, pos.Z),
			"itemId", itemID, "stackpos", stackpos)
	}
	g.sendCancelMessage(msg)
}

// wrapableAt is the body of the wrap/unwrap request, split out so the auto-walk
// retry can run it again once the player has arrived.
func (g *GameProtocol) wrapableAt(pos netmsg.Position, itemID uint16, stackpos uint8) {
	if g.player == nil {
		return
	}
	item := g.resolveStowItem(pos, int(stackpos), itemID)
	if item == nil || item.ID != itemID {
		g.wrapRefuse("itemNotFound", "Sorry, not possible.", pos, itemID, stackpos)
		return
	}

	gamePos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}
	tile := g.deps.World.Map.GetTile(gamePos)
	if tile == nil {
		g.wrapRefuse("noTile", "Sorry, not possible.", pos, itemID, stackpos)
		return
	}

	// Only inside a house, and only on a protection-zone tile.
	house := g.deps.World.GetHouseByPosition(gamePos)
	if house == nil || !tile.IsProtectionZone() {
		g.wrapRefuse("notInsideHouse", "You may construct this only inside a house.", pos, itemID, stackpos)
		return
	}
	if !houseAccessIsOwner(house, g.player) {
		g.wrapRefuse("notOwner", "You are not allowed to construct this here.", pos, itemID, stackpos)
		return
	}
	// A unique id marks a map-placed fixture, which must not be pocketed.
	if item.Attr != nil && item.Attr.UniqueID != nil {
		g.wrapRefuse("hasUniqueId", "Sorry, not possible.", pos, itemID, stackpos)
		return
	}

	t := g.deps.Items.Get(item.ID)
	isKit := item.ID == game.ItemDecorationKit
	// Item::isWrapable is `wrapable && wrapableTo`, and items.cpp sets wrapableTo to
	// the decoration kit for every wrappable type.
	wrapable := t != nil && t.WrapableTo != 0
	if !wrapable && !isKit {
		g.wrapRefuse("notWrapable", "Sorry, not possible.", pos, itemID, stackpos)
		return
	}

	// An owned item belongs to whoever bought it, even inside someone else's house.
	if item.Attr != nil && item.Attr.Owner != nil && *item.Attr.Owner != 0 &&
		*item.Attr.Owner != g.player.DBID {
		g.wrapRefuse("notYours", "This item is not yours.", pos, itemID, stackpos)
		return
	}

	// onlyInvitedCanMoveHouseItems: a guest may not rearrange the furniture.
	if config.Bool("onlyInvitedCanMoveHouseItems", true) &&
		!houseAccessIsOwner(house, g.player) && !house.IsSubOwner(g.player.Name) {
		g.wrapRefuse("onlyInvitedCanMove", "You cannot use this object.", pos, itemID, stackpos)
		return
	}

	// Out of reach: walk there and run this again, which is what C++ does
	// (src/game/game.cpp:5401-5419). Refusing outright was the old behaviour and is
	// why unwrapping a kit across the room never worked — a session log shows six
	// requests for the right kit from five and six tiles away, every one rejected.
	if pos.X != 0xFFFF && chebyshev(gamePos, g.player.Pos) > 1 {
		if !g.walkToThenRetry(gamePos, func() { g.wrapableAt(pos, itemID, stackpos) }) {
			g.wrapRefuse("noPath", "There is no way.", pos, itemID, stackpos)
		}
		return
	}

	// A container has to be emptied first, or its contents vanish with it.
	if len(item.Contents) > 0 {
		g.wrapRefuse("hasContents", "Sorry, not possible.", pos, itemID, stackpos)
		return
	}

	// Wrapping only works on the floor: a kit held by the player, or a tile whose
	// top item would take an auto-carpet, is refused.
	topItem := topTopItem(tile, g.deps.Items)
	blockedUnwrap := topItem != nil && canReceiveAutoCarpet(topItem, g.deps.Items) &&
		!(t != nil && t.HasProperty(items.PropImmovableBlockSolid))
	if blockedUnwrap {
		g.wrapRefuse("notOnFloor", "You can only wrap/unwrap on the floor.", pos, itemID, stackpos)
		return
	}

	// A filled bath tub would lose its contents.
	if item.ID == itemFilledBathTube {
		g.wrapRefuse("filledBathTub", "Sorry, not possible.", pos, itemID, stackpos)
		return
	}

	if isKit {
		if !g.unwrapItem(gamePos, item, house) {
			return
		}
	} else {
		g.wrapItem(gamePos, item, t, house)
	}
	g.sendMagicEffect(gamePos, 3) // CONST_ME_POFF
}

// wrapItem turns furniture into a decoration kit, remembering what it was.
func (g *GameProtocol) wrapItem(pos game.Position, item *game.Item, t *items.ItemType, house *game.House) {
	original := item.ID
	// Game::wrapItem (game.cpp:5463) drops the bed from the house's list before
	// transforming it. Without this the house keeps counting a bed that is now a
	// decoration kit in someone's backpack, and eventually refuses to accept a
	// real one because it believes it is at the cap.
	if house != nil && t != nil && t.Type == items.ItemTypeBed {
		house.RemoveBed(pos)
	}
	name := "item"
	if t != nil && t.Name != "" {
		name = t.Name
	}
	// A cask hides its remaining charges in the DATE slot so unwrapping restores
	// them; without this the cask comes back empty.
	var hiddenCharges uint64
	if isCaskItem(original) {
		hiddenCharges = uint64(item.Count)
	}
	amount := item.Count
	if amount == 0 {
		amount = 1
	}
	var storeStamp *int64
	if item.Attr != nil {
		storeStamp = item.Attr.StoreTimestamp
	}

	g.deps.World.TransformItem(pos, item, game.ItemDecorationKit)
	if item.Attr == nil {
		item.Attr = &game.ItemAttributes{}
	}
	item.SetCustomAttribute("unWrapId", int64(original))
	desc := fmt.Sprintf("You bought this item in the Store.\nUnwrap it in your own house to create a <%s>.", name)
	item.Attr.Description = &desc
	item.Attr.Amount = &amount
	// The store stamp survives the round trip, so a store item stays marked as one.
	item.Attr.StoreTimestamp = storeStamp
	if hiddenCharges > 0 {
		item.Attr.WrittenDate = &hiddenCharges // ItemAttribute_t::DATE
	}
}

// unwrapItem turns a kit back into what it was wrapped from. It reports whether
// the unwrap happened, so the caller can skip the effect when it did not.
func (g *GameProtocol) unwrapItem(pos game.Position, item *game.Item, house *game.House) bool {
	raw, ok := item.GetCustomAttribute("unWrapId")
	unwrapID := customAttrUint16(raw)
	if !ok || unwrapID == 0 {
		// Outside wrapableAt, so this refusal carried no reason either: a kit whose
		// unWrapId the store never wrote looks identical, from the log, to a request
		// that was dropped.
		g.wrapRefuse("noUnwrapId", "Sorry, not possible.",
			netmsg.Position{X: pos.X, Y: pos.Y, Z: pos.Z}, item.ID, 0)
		return false
	}

	// The bed cap is checked BEFORE the transform (game.cpp:5493): a refused
	// unwrap has to leave the kit intact, and a transform is not undoable.
	newType := g.deps.Items.Get(unwrapID)
	isBed := newType != nil && newType.Type == items.ItemTypeBed
	if isBed && house != nil && house.MaxBeds > -1 && int32(house.BedCount()) >= house.MaxBeds {
		g.player.SendCancelMessage("You reached the maximum beds in this house")
		return false
	}
	if g.deps != nil && g.deps.Log != nil {
		g.deps.Log.Warn("unwrap proceeding", "item", item.ID, "into", unwrapID,
			"pos", fmt.Sprintf("%d,%d,%d", pos.X, pos.Y, pos.Z))
	}

	amount := uint16(1)
	var hiddenCharges uint64
	var storeStamp *int64
	if item.Attr != nil {
		if item.Attr.Amount != nil && *item.Attr.Amount > 0 {
			amount = *item.Attr.Amount
		}
		if item.Attr.WrittenDate != nil {
			hiddenCharges = *item.Attr.WrittenDate
		}
		storeStamp = item.Attr.StoreTimestamp
	}

	g.deps.World.TransformItem(pos, item, unwrapID)
	item.Count = amount
	// A cask comes back with the charges it went in with.
	if hiddenCharges > 0 && isCaskItem(unwrapID) {
		item.Count = uint16(hiddenCharges)
	}
	item.RemoveCustomAttribute("unWrapId")
	if item.Attr != nil {
		item.Attr.Description = nil
		item.Attr.WrittenDate = nil
		item.Attr.StoreTimestamp = storeStamp
	}
	// game.cpp:5507 — the house takes the new bed onto its list, which is what
	// getBedCount reports and what the houses table stores.
	if isBed && house != nil {
		house.AddBed(pos)
	}
	return true
}

// topTopItem returns the tile's topmost always-on-top item, Tile::getTopTopItem.
func topTopItem(tile *game.Tile, cat *items.Catalog) *game.Item {
	var top *game.Item
	for _, it := range tile.Items {
		if t := cat.Get(it.ID); t != nil && t.AlwaysOnTop() {
			top = it
		}
	}
	return top
}

// canReceiveAutoCarpet ports Item::canReceiveAutoCarpet (item.hpp:565).
func canReceiveAutoCarpet(it *game.Item, cat *items.Catalog) bool {
	t := cat.Get(it.ID)
	return t != nil && t.BlockSolid && t.AlwaysOnTop() && !t.HasHeight
}

// customAttrUint16 narrows a custom attribute to the u16 an item id needs. The store
// writes the id as an integer, so anything else means a malformed kit.
func customAttrUint16(v any) uint16 {
	switch n := v.(type) {
	case int64:
		if n > 0 && n <= 0xFFFF {
			return uint16(n)
		}
	case float64:
		if n > 0 && n <= 0xFFFF {
			return uint16(n)
		}
	}
	return 0
}

// parseLookInBattleList handles 0x8D — look at a creature in the battle list.
func (g *GameProtocol) parseLookInBattleList(r *netmsg.Reader) {
	// EventCallback playerOnLookInBattleList(player, creature, distance) — (void).
	defer func() {
		if g.deps.Events == nil || g.player == nil {
			return
		}
		if target := g.deps.World.CreatureByID(g.player.TargetID); target != nil {
			g.deps.Events.ExecutePlayerOnLookInBattleList(g.player, target,
				chebyshev(g.player.Pos, target.GetPosition()))
		}
	}()
	creatureID := r.GetU32()
	_ = creatureID
}

// parseJoinAggression handles 0x8E — join aggression against a target.
func (g *GameProtocol) parseJoinAggression(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	targetID := r.GetU32()
	g.deps.World.PlayerJoinAggression(g.player.ID, targetID)
}

// parseOpenDepotSearch handles 0x92 — open depot search.
func (g *GameProtocol) parseOpenDepotSearch(r *netmsg.Reader) {
	g.sendStatusText("Use depot search from the depot.")
}

// parseCloseDepotSearch handles 0x93 — close depot search.
func (g *GameProtocol) parseCloseDepotSearch(r *netmsg.Reader) {
}

// parseDepotSearchItemRequest handles 0x94 — depot search item request.
func (g *GameProtocol) parseDepotSearchItemRequest(r *netmsg.Reader) {
	_ = r.GetU16() // itemId
}

// parseOpenParentContainer handles 0x95 — open parent container (alias for container up).
func (g *GameProtocol) parseOpenParentContainer(r *netmsg.Reader) {
	containerID := r.GetByte()
	_ = containerID
	g.parseContainerUp(r)
}

// parseEditGuildMessage handles 0x9C — edit guild message.
func (g *GameProtocol) parseEditGuildMessage(r *netmsg.Reader) {
	_ = r.GetString() // new message
}

// parseGetTextForReport handles 0x9D — get recent chat for rule violation report.
func (g *GameProtocol) parseGetTextForReport(r *netmsg.Reader) {
	w := netmsg.NewWriter()
	w.AddByte(0x9D)
	w.AddU16(0) // empty
	g.SendToClient(w)
}

// parseCloseNpcChannel handles 0x9E — close NPC trade channel.
func (g *GameProtocol) parseCloseNpcChannel(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	g.deps.World.PlayerCloseNpcChannel(g.player.ID)
}

// parseSetMonsterPodium handles 0x9F — set monster podium decoration.
func (g *GameProtocol) parseSetMonsterPodium(r *netmsg.Reader) {
	_ = r.GetU16()  // monster race id
	_ = r.GetByte() // look type
	_ = r.GetByte() // state
}

// parseFollow handles 0xA2 — follow a creature.
func (g *GameProtocol) parseFollow(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	targetID := r.GetU32()
	g.deps.World.PlayerFollow(g.player.ID, targetID)
}

func (g *GameProtocol) parseQuestLog(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	// EventCallback playerOnRequestQuestLog(player).
	if g.deps.Events != nil && !g.deps.Events.ExecutePlayerOnRequestQuestLog(g.player) {
		return
	}
	g.deps.Lua.Call("onPlayerQuestLog", g.player.Name)
}
func (g *GameProtocol) parseQuestLine(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	qid := r.GetU16()
	g.deps.Lua.Call("onPlayerQuestLine", g.player.Name, fmt.Sprint(qid))
}
func (g *GameProtocol) parseRewardChestCollect(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
}

func (g *GameProtocol) parseCharacterTradeConfig(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetByte()
}
func (g *GameProtocol) parseExivaRestrictions(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetByte()
	_ = r.GetString()
}
func (g *GameProtocol) parseBrowseField(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	pos := r.GetPosition()
	gPos := game.Position{X: pos.X, Y: pos.Y, Z: pos.Z}

	// Same floor check
	if g.player.Pos.Z != gPos.Z {
		msg := "First go upstairs"
		if g.player.Pos.Z < gPos.Z {
			msg = "First go downstairs"
		}
		g.sendStatusText(msg)
		return
	}

	// Gods/GMs can browse from any distance
	if g.player.AccountType < 5 {
		dx := int(g.player.Pos.X) - int(gPos.X)
		if dx < 0 {
			dx = -dx
		}
		dy := int(g.player.Pos.Y) - int(gPos.Y)
		if dy < 0 {
			dy = -dy
		}
		if dx > 1 || dy > 1 {
			g.sendStatusText("You are too far.")
			return
		}
	}

	g.sendBrowseField(gPos)
}
func (g *GameProtocol) parseClientDetails(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetU16()
	_ = r.GetU16()
}
func (g *GameProtocol) parseBossDifficultySelection(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetU16()
	_ = r.GetByte()
}
func (g *GameProtocol) parseAimAtTarget(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetU32()
}
func (g *GameProtocol) parseGetTransactionDetails(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetU32()
}
func (g *GameProtocol) parseCyclopediaMapAction(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	_ = r.GetByte()
}
func (g *GameProtocol) parseBugReport(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	category := r.GetByte()
	_ = r.GetByte()
	message := r.GetString()
	_ = r.GetString()
	// EventCallback playerOnReportBug(player, message, position, category) — (void).
	if g.deps.Events != nil {
		g.deps.Events.ExecutePlayerOnReportBug(g.player, message, g.player.Pos, category)
	}
}

// Removed as dead duplicates (each was defined but unreachable, and the opcode
// it would have served is already handled by a dispatched handler):
//
//	parseMode                  → 0xA0 is parseFightModes
//	parsePingBack              → 0x1D is dispatched as inPing
//	parseReceivePing           → 0x1E is dispatched as inPong
//	parseRequestOutfit         → 0xD2 calls SendOutfitWindow directly
//	parseBlessingWindowRequest → 0xCF calls SendBlessingsDialog directly
//	parseWheelOfDestiny        → 0x61/0x62 are parseOpenWheel/parseSaveWheel
//	parseQuestTracker          → no counterpart in the C++ dispatcher
//	parseTrackAnalysis         → no counterpart in the C++ dispatcher

// houseAccessIsOwner reports HOUSE_OWNER access, the top of
// House::getHouseAccessLevel (src/map/house/house.cpp). Ownership is not the only
// way in: a staff member carrying PlayerFlags_t::CanEditHouses counts as the owner
// of EVERY house, which is what makes /owner and in-place decoration work for a
// gamemaster standing in someone else's building.
func houseAccessIsOwner(house *game.House, p *game.Player) bool {
	if house == nil || p == nil {
		return false
	}
	return house.GetHouseAccessLevel(p) == game.HouseOwner
}
