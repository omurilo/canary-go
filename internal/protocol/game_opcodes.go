package protocol

import (
	"fmt"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
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
		_ = r.GetU16() // slots
		_ = r.GetU16() // minLevel
		_ = r.GetU16() // maxLevel
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
	if nType := g.shopOwnerType(); nType != nil {
		for _, si := range nType.ShopItems {
			if si.ID == itemID {
				g.sendStatusText(si.Name)
				break
			}
		}
	} else {
		g.sendStatusText("You are not currently trading with anyone.")
	}
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
	_ = r.GetU32() // window id
	_ = r.GetString() // new text
}

// parseHouseWindow handles 0x8A — house window action.
func (g *GameProtocol) parseHouseWindow(r *netmsg.Reader) {
	houseID := r.GetByte()
	action := r.GetByte()
	_ = houseID
	_ = action
}

// parseWrapableItem handles 0x8B — wrap an item.
func (g *GameProtocol) parseWrapableItem(r *netmsg.Reader) {
	itemID := r.GetU16()
	pos := r.GetPosition()
	_ = itemID
	_ = pos
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
	_ = r.GetU16() // monster race id
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
func (g *GameProtocol) parseQuestLine(r *netmsg.Reader) { if g.player == nil { return }; qid := r.GetU16(); g.deps.Lua.Call("onPlayerQuestLine", g.player.Name, fmt.Sprint(qid)) }
func (g *GameProtocol) parseRewardChestCollect(r *netmsg.Reader) { if g.player == nil { return } }

func (g *GameProtocol) parseCharacterTradeConfig(r *netmsg.Reader) { if g.player == nil { return }; _ = r.GetByte() }
func (g *GameProtocol) parseExivaRestrictions(r *netmsg.Reader) { if g.player == nil { return }; _ = r.GetByte(); _ = r.GetString() }
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
func (g *GameProtocol) parseClientDetails(r *netmsg.Reader) { if g.player == nil { return }; _ = r.GetU16(); _ = r.GetU16() }
func (g *GameProtocol) parseBossDifficultySelection(r *netmsg.Reader) { if g.player == nil { return }; _ = r.GetU16(); _ = r.GetByte() }
func (g *GameProtocol) parseAimAtTarget(r *netmsg.Reader) { if g.player == nil { return }; _ = r.GetU32() }
func (g *GameProtocol) parseGetTransactionDetails(r *netmsg.Reader) { if g.player == nil { return }; _ = r.GetU32() }
func (g *GameProtocol) parseCyclopediaMapAction(r *netmsg.Reader) { if g.player == nil { return }; _ = r.GetByte() }
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
