package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// Exaltation Forge protocol. Server→client packets:
//   0x86 sendForgingData    — classification/tier price tables + config
//   0x87 sendOpenForge      — the player's fusable/transferable items
//   0x88 sendForgeHistory   — paginated action log
//   0x89 closeForgeWindow   — empty; also used as the error close
//   0x8A sendForgeResult    — outcome of a fusion/transfer
// Client→server packets: 0xBF parseForgeEnter, 0xC0 parseForgeBrowseHistory.
// Forge resources (dust/sliver/core) ride the 0xEE resource-balance packet.

// Forge resource ids for the 0xEE resource-balance packet (server_definitions.hpp).
const (
	resourceForgeDust   = 0x46
	resourceForgeSliver = 0x47
	resourceForgeCores  = 0x48
)

// parseForgeEnter handles client packet 0xBF (all forge actions).
func (g *GameProtocol) parseForgeEnter(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	actionType := r.GetByte()

	var (
		convergence    bool
		firstItem      uint16
		tier           uint8
		secondItem     uint16
		usedCore       bool
		reduceTierLoss bool
	)
	if actionType == game.ForgeActionFusion || actionType == game.ForgeActionTransfer {
		convergence = r.GetByte() == 1
		firstItem = r.GetU16()
		tier = r.GetByte()
		secondItem = r.GetU16()
	}

	switch actionType {
	case game.ForgeActionFusion:
		if !convergence {
			usedCore = r.GetByte() == 1
			reduceTierLoss = r.GetByte() == 1
		}
		res := g.player.ForgeFuseItems(g.deps.Items, firstItem, tier, secondItem, usedCore, reduceTierLoss, convergence)
		if res.Err != "" {
			g.sendForgeError(res.Err)
			return
		}
		// Refresh the bag (consumed items + new exaltation chest) first, then let
		// sendForgeResult repaint the forge window (0x8A → 0x87 → 0x86 → resources)
		// so the resource balances are the last thing the client sees.
		g.refreshForgeInventory()
		g.sendForgeResult(res)

	case game.ForgeActionTransfer:
		res := g.player.ForgeTransferItemTier(g.deps.Items, firstItem, tier, secondItem, convergence)
		if res.Err != "" {
			g.sendForgeError(res.Err)
			return
		}
		g.refreshForgeInventory()
		g.sendForgeResult(res)

	default:
		if actionType <= game.ForgeActionIncreaseLimit {
			if !g.player.ForgeResourceConversion(g.deps.Items, actionType) {
				// Mirrors Player::forgeResourceConversion sending sendForgeError on
				// insufficient resources; also stops the "infinite convert" illusion
				// once the client's stale count is corrected.
				g.sendForgeError("You do not have the required resources.")
				return
			}
			// Bag changed (slivers/cores are items); repaint it, then the forge
			// window with fresh resource balances last.
			g.refreshForgeInventory()
			g.sendOpenForge()
		}
	}
}

// parseForgeBrowseHistory handles client packet 0xC0.
func (g *GameProtocol) parseForgeBrowseHistory(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	page := r.GetByte()
	g.sendForgeHistory(page)
}

// sendForgingData sends 0x86: per-classification tier prices plus all the forge
// config values the client needs to render costs. Mirrors sendForgingData.
func (g *GameProtocol) sendForgingData() {
	if g.player == nil {
		return
	}
	classes := game.ForgeClassifications()

	// Iterate classifications in stable id order (1..4).
	ids := make([]uint8, 0, len(classes))
	for id := uint8(1); id <= 4; id++ {
		if _, ok := classes[id]; ok {
			ids = append(ids, id)
		}
	}

	w := netmsg.NewWriter()
	w.AddByte(0x86)

	corePrices := map[uint8]uint8{}
	convFusion := map[uint8]uint64{}
	convTransfer := map[uint8]uint64{}

	w.AddByte(byte(len(ids)))
	for _, id := range ids {
		tiers := classes[id]
		w.AddByte(id)
		w.AddByte(byte(len(tiers)))
		for t := uint8(1); t <= game.ForgeMaxItemTier; t++ {
			info, ok := tiers[t]
			if !ok {
				continue
			}
			w.AddByte(t - 1)
			w.AddU64(info.RegularPrice)
			corePrices[t] = info.CorePrice
			convFusion[t] = info.ConvergenceFusionPrice
			convTransfer[t] = info.ConvergenceTransferPrice
		}
	}

	// Exalted core cost per tier.
	w.AddByte(byte(len(corePrices)))
	for t := uint8(1); t <= game.ForgeMaxItemTier; t++ {
		if c, ok := corePrices[t]; ok {
			w.AddByte(t)
			w.AddByte(c)
		}
	}
	// Convergence fusion prices per tier.
	w.AddByte(byte(len(convFusion)))
	for t := uint8(1); t <= game.ForgeMaxItemTier; t++ {
		if p, ok := convFusion[t]; ok {
			w.AddByte(t - 1)
			w.AddU64(p)
		}
	}
	// Convergence transfer prices per tier.
	w.AddByte(byte(len(convTransfer)))
	for t := uint8(1); t <= game.ForgeMaxItemTier; t++ {
		if p, ok := convTransfer[t]; ok {
			w.AddByte(t)
			w.AddU64(p)
		}
	}

	w.AddByte(game.ForgeCostOneSliver)           // dust cost of 1 sliver batch input
	w.AddByte(game.ForgeSliverAmount)            // slivers produced
	w.AddByte(game.ForgeCoreCost)                // slivers per core
	w.AddByte(game.ForgeDustLevelBase)           // dustLevel - this = increase cost
	w.AddU16(g.player.GetForgeDustLevel())       // current stored dust limit
	w.AddU16(game.ForgeMaxDust)                  // max stored dust limit
	w.AddByte(game.ForgeFusionDustCost)          // normal fusion dust
	w.AddByte(game.ForgeConvergenceFusionCost)   // convergence fusion dust
	w.AddByte(game.ForgeTransferDustCost)        // normal transfer dust
	w.AddByte(game.ForgeConvergenceTransferCost) // convergence transfer dust
	w.AddByte(game.ForgeBaseSuccessRate)         // base success rate
	w.AddByte(game.ForgeBonusSuccessRate)        // core bonus success rate
	w.AddByte(game.ForgeTierLossReduction)       // tier-loss chance after reduction

	g.SendToClient(w)
	g.sendForgeResources()
}

// forgeInfoMap is itemId -> tier -> count, mirroring the C++ item maps.
type forgeInfoMap map[uint16]map[uint8]uint16

func (m forgeInfoMap) add(it *game.Item, count uint16) {
	if m[it.ID] == nil {
		m[it.ID] = map[uint8]uint16{}
	}
	if count == 0 {
		count = 1
	}
	m[it.ID][it.GetTier()] += count
}

// sendOpenForge sends 0x87: the player's fusable and transferable items. Ported
// from ProtocolGame::sendOpenForge. Convergence lists are sent as empty (count
// 0) — the wire format stays valid; convergence item listing is not yet modeled.
func (g *GameProtocol) sendOpenForge() {
	if g.player == nil {
		return
	}
	cat := g.deps.Items
	maxConfigTier := uint8(game.ForgeMaxItemTier)

	fusion := forgeInfoMap{}
	donor := forgeInfoMap{}
	receive := forgeInfoMap{}

	g.player.WalkInventory(func(it *game.Item) {
		t := cat.Get(it.ID)
		if t == nil {
			return
		}
		classification := t.UpgradeClassification
		if classification == 0 {
			return
		}
		itemTier := it.GetTier()
		maxTier := classification
		if classification == 4 {
			maxTier = maxConfigTier
		}
		count := it.Count
		if t.Stackable {
			// Stackables carry no tier/classification in practice; skip.
			return
		}
		count = 1
		if itemTier < maxTier {
			fusion.add(it, count)
		}
		if classification < 4 && itemTier > maxTier {
			return
		}
		if itemTier > 1 {
			donor.add(it, count)
		}
		if itemTier == 0 {
			receive.add(it, count)
		}
	})

	// Count fusion groups with at least two copies (a fusion needs a pair).
	var fusionTotal uint16
	for _, tiers := range fusion {
		for _, c := range tiers {
			if c >= 2 {
				fusionTotal++
			}
		}
	}

	w := netmsg.NewWriter()
	w.AddByte(0x87)

	w.AddU16(fusionTotal)
	for id, tiers := range fusion {
		for tier, c := range tiers {
			if c >= 2 {
				w.AddByte(0x01)
				w.AddU16(id)
				w.AddByte(tier)
				w.AddU16(c)
			}
		}
	}

	// Convergence fusion: no groups.
	w.AddU16(0)

	// Transfer donors + matching receivers.
	w.AddByte(byte(len(donor)))
	for donorID, tiers := range donor {
		dt := cat.Get(donorID)
		w.AddU16(uint16(len(tiers)))
		for tier, c := range tiers {
			w.AddU16(donorID)
			w.AddByte(tier)
			w.AddU16(c)
		}
		// Receivers matching classification + normalized slot.
		var recvCount uint16
		for recvID := range receive {
			rt := cat.Get(recvID)
			if rt != nil && dt != nil &&
				rt.UpgradeClassification == dt.UpgradeClassification &&
				normSlot(rt.SlotPosition) == normSlot(dt.SlotPosition) {
				recvCount++
			}
		}
		w.AddU16(recvCount)
		if recvCount > 0 {
			for recvID, rtiers := range receive {
				rt := cat.Get(recvID)
				if rt == nil || dt == nil ||
					rt.UpgradeClassification != dt.UpgradeClassification ||
					normSlot(rt.SlotPosition) != normSlot(dt.SlotPosition) {
					continue
				}
				for _, c := range rtiers {
					w.AddU16(recvID)
					w.AddU16(c)
				}
			}
		}
	}

	// Convergence transfer: no groups.
	w.AddByte(0)

	w.AddU16(g.player.GetForgeDustLevel())
	g.SendToClient(w)
	g.sendForgingData()
}

// normSlot collapses the two-handed / hand slot variants so that a donor and a
// receiver in "the same hand" match, mirroring the SLOTP_TWO_HAND→SLOTP_HAND
// fold in C++ sendOpenForge.
func normSlot(s string) string {
	switch s {
	case "two-handed", "one-handed", "left", "right", "hand", "shield", "weapon":
		return "hand"
	}
	return s
}

// sendForgeResult sends 0x8A. Ported from ProtocolGame::sendForgeResult.
func (g *GameProtocol) sendForgeResult(res game.ForgeResult) {
	if g.player == nil {
		return
	}
	leftID, leftTier := res.LeftItemID, res.LeftTier
	rightID, rightTier := res.RightItemID, res.RightTier
	success := res.Success

	if res.Convergence && res.ActionType == game.ForgeActionFusion {
		success = true
		leftID, rightID = rightID, leftID
	}

	w := netmsg.NewWriter()
	w.AddByte(0x8A)
	w.AddByte(res.ActionType)
	w.AddByte(boolByte(res.Convergence))
	w.AddByte(boolByte(success))
	w.AddU16(leftID)
	w.AddByte(leftTier)
	w.AddU16(rightID)
	w.AddByte(rightTier)

	if res.ActionType == game.ForgeActionTransfer {
		w.AddByte(0x00) // bonus none for transfer
	} else {
		w.AddByte(res.Bonus)
		switch {
		case res.Bonus == 2:
			w.AddByte(res.CoreCount)
		case res.Bonus >= 4 && res.Bonus <= 8:
			w.AddU16(leftID)
			w.AddByte(leftTier)
		}
	}

	g.SendToClient(w)
	g.sendOpenForge()
}

// sendForgeHistory sends 0x88 (9 entries per page, newest first). Ported from
// ProtocolGame::sendForgeHistory.
func (g *GameProtocol) sendForgeHistory(page uint8) {
	if g.player == nil {
		return
	}
	const perPage = 9
	history := g.player.ForgeHistory
	total := len(history)

	currentPage := 1
	lastPage := 1
	var pageEntries []game.ForgeHistory
	if total > 0 {
		lastPage = (total-1)/perPage + 1
		requested := int(page) + 1
		if requested > lastPage {
			requested = lastPage
		}
		currentPage = requested
		first := total - (currentPage-1)*perPage
		last := 0
		if total > currentPage*perPage {
			last = total - currentPage*perPage
		}
		for e := first; e > last; e-- {
			pageEntries = append(pageEntries, history[e-1])
		}
	}

	w := netmsg.NewWriter()
	w.AddByte(0x88)
	w.AddU16(uint16(currentPage - 1))
	w.AddU16(uint16(lastPage))
	w.AddByte(byte(len(pageEntries)))
	for _, h := range pageEntries {
		w.AddU32(h.CreatedAt)
		w.AddByte(h.ActionType)
		w.AddString(h.Description)
		w.AddByte(boolByte(h.Bonus >= 1 && h.Bonus < 8))
	}
	g.SendToClient(w)
}

// closeForgeWindow sends 0x89 (empty).
func (g *GameProtocol) closeForgeWindow() {
	w := netmsg.NewWriter()
	w.AddByte(0x89)
	g.SendToClient(w)
}

// sendForgeError shows the failure text and closes the forge window, mirroring
// ProtocolGame::sendForgeError.
func (g *GameProtocol) sendForgeError(msg string) {
	if g.player != nil {
		g.player.SendTextMessage(0x14, msg) // MESSAGE_LOOK / small status
	}
	g.closeForgeWindow()
}

// sendForgeResources pushes the dust/sliver/core balances (0xEE) plus money.
func (g *GameProtocol) sendForgeResources() {
	if g.player == nil {
		return
	}
	g.sendResourceBalance(resourceForgeDust, g.player.GetForgeDusts())
	g.sendResourceBalance(resourceForgeSliver, uint64(g.player.GetForgeSlivers(g.deps.Items)))
	g.sendResourceBalance(resourceForgeCores, uint64(g.player.GetForgeCores(g.deps.Items)))
	g.sendResourceBalance(0x00, g.player.BankBalance)
	g.sendResourceBalance(0x01, uint64(g.player.GetMoney()))
}

// refreshForgeInventory re-sends inventory, open containers and stats after a
// forge operation mutated the player's items. It intentionally does NOT send the
// forge resource balances — the caller sends the forge window packets (which
// carry the fresh balances) afterwards so those are the client's last update.
func (g *GameProtocol) refreshForgeInventory() {
	p := g.player
	if p == nil {
		return
	}
	p.UpdateInventoryWeight(g.deps.Items)
	for slot := game.ConstSlotFirst; slot <= game.ConstSlotLast; slot++ {
		if item := p.Inventory[slot]; item != nil {
			g.sendInventoryItem(uint8(slot), item)
		} else {
			g.sendInventoryEmpty(uint8(slot))
		}
	}
	for _, c := range g.rangeContainers() {
		g.refreshContainerIfOpen(c)
	}
	g.sendStats()
}

// SendOpenForge is the Session entry point used by player:openForge().
func (g *GameProtocol) SendOpenForge() {
	g.sendOpenForge()
}
