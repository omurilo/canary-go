package game

import (
	"fmt"
	"math/rand"

	"github.com/opentibiabr/canary-go/internal/config"
	"github.com/opentibiabr/canary-go/internal/items"
)

// This file ports the Exaltation Forge from the C++ server:
//   - src/game/game.cpp            (Game::playerForge* — RNG rolls, exhaustion)
//   - src/creatures/players/player.cpp (Player::forgeFuseItems / forgeTransferItemTier /
//                                       forgeResourceConversion — item mutation & costs)
//   - src/utils/tools.cpp          (forgeBonus — the 0..10000 bonus table)
//   - data/scripts/systems/item_tiers.lua (per-classification tier price tables)
//
// Design notes for the Go port:
//   - Slivers and cores are real inventory items (ItemForgeSliver / ItemForgeCore),
//     exactly like C++, manipulated through the existing inventory helpers.
//   - Dust is Player.ForgeDusts; the stored-dust limit is Player.ForgeDustLevel.
//   - An item's upgrade classification comes from the catalog
//     (items.ItemType.UpgradeClassification); its tier from Item.GetTier().
//   - The forged results are placed into an exaltation chest container that is
//     added to the player's inventory, mirroring ITEM_EXALTATION_CHEST.

// Forge action types (client packet 0xBF, first byte). Mirrors ForgeAction_t.
const (
	ForgeActionFusion        uint8 = 0
	ForgeActionTransfer      uint8 = 1
	ForgeActionDustToSliver  uint8 = 2
	ForgeActionSliverToCore  uint8 = 3
	ForgeActionIncreaseLimit uint8 = 4
)

// Forge item ids (src/utils/utils_definitions.hpp).
const (
	ItemForgeSliver     uint16 = 37109
	ItemForgeCore       uint16 = 37110
	ItemExaltationChest uint16 = 37561
)

// Forge configuration is read from config.lua (same keys as the C++
// g_configManager), falling back to the shipped config.lua.dist defaults. The
// only hardcoded value is ForgeDustLevelBase (75), which the C++ hardcodes too.
const ForgeDustLevelBase = 75

func ForgeMaxItemTier() int   { return int(config.Number("forgeMaxItemTier", 10)) }
func ForgeCostOneSliver() int { return int(config.Number("forgeCostOneSliver", 20)) }
func ForgeSliverAmount() int  { return int(config.Number("forgeSliverAmount", 3)) }
func ForgeCoreCost() int      { return int(config.Number("forgeCoreCost", 50)) }
func ForgeMaxDust() int       { return int(config.Number("forgeMaxDust", 225)) }
func ForgeFusionDustCost() int {
	return int(config.Number("forgeFusionDustCost", 100))
}
func ForgeConvergenceFusionCost() int {
	return int(config.Number("forgeConvergenceFusionDustCost", 130))
}
func ForgeTransferDustCost() int {
	return int(config.Number("forgeTransferDustCost", 100))
}
func ForgeConvergenceTransferCost() int {
	return int(config.Number("forgeConvergenceTransferCost", 160))
}
func ForgeBaseSuccessRate() int {
	return int(config.Number("forgeBaseSuccessRate", 50))
}
func ForgeBonusSuccessRate() int {
	return int(config.Number("forgeBonusSuccessRate", 15))
}
func ForgeTierLossReduction() int {
	return int(config.Number("forgeTierLossReduction", 50))
}

// ForgeTierPrice holds the gold/core costs to reach a given tier for a
// classification, mirroring ItemClassification::Tier.
type ForgeTierPrice struct {
	RegularPrice             uint64
	CorePrice                uint8
	ConvergenceFusionPrice   uint64
	ConvergenceTransferPrice uint64
}

// forgeClassifications maps classification id -> tier -> prices. Transcribed
// verbatim from data/scripts/systems/item_tiers.lua. Tiers are keyed as in Lua
// (1-based: tier N here is the cost to upgrade an item FROM tier N-1 to N).
var forgeClassifications = map[uint8]map[uint8]ForgeTierPrice{
	1: {
		1: {RegularPrice: 25000, CorePrice: 1},
	},
	2: {
		1: {RegularPrice: 750000, CorePrice: 1},
		2: {RegularPrice: 5000000, CorePrice: 1},
	},
	3: {
		1: {RegularPrice: 4000000, CorePrice: 1},
		2: {RegularPrice: 10000000, CorePrice: 2},
		3: {RegularPrice: 20000000, CorePrice: 3},
	},
	4: {
		1:  {RegularPrice: 8000000, CorePrice: 1, ConvergenceFusionPrice: 55000000, ConvergenceTransferPrice: 65000000},
		2:  {RegularPrice: 20000000, CorePrice: 2, ConvergenceFusionPrice: 110000000, ConvergenceTransferPrice: 165000000},
		3:  {RegularPrice: 40000000, CorePrice: 5, ConvergenceFusionPrice: 170000000, ConvergenceTransferPrice: 375000000},
		4:  {RegularPrice: 65000000, CorePrice: 10, ConvergenceFusionPrice: 300000000, ConvergenceTransferPrice: 800000000},
		5:  {RegularPrice: 100000000, CorePrice: 15, ConvergenceFusionPrice: 875000000, ConvergenceTransferPrice: 2000000000},
		6:  {RegularPrice: 250000000, CorePrice: 25, ConvergenceFusionPrice: 2350000000, ConvergenceTransferPrice: 5250000000},
		7:  {RegularPrice: 750000000, CorePrice: 35, ConvergenceFusionPrice: 6950000000, ConvergenceTransferPrice: 14500000000},
		8:  {RegularPrice: 2500000000, CorePrice: 50, ConvergenceFusionPrice: 21250000000, ConvergenceTransferPrice: 42500000000},
		9:  {RegularPrice: 8000000000, CorePrice: 60, ConvergenceFusionPrice: 50000000000, ConvergenceTransferPrice: 100000000000},
		10: {RegularPrice: 15000000000, CorePrice: 85, ConvergenceFusionPrice: 125000000000, ConvergenceTransferPrice: 300000000000},
	},
}

// ForgeClassifications returns the whole price table (used by the 0x86 forging
// data packet). Callers must not mutate the returned maps.
func ForgeClassifications() map[uint8]map[uint8]ForgeTierPrice { return forgeClassifications }

// forgeTierPrice looks up the price entry for (classification, tier). ok is
// false when the classification or tier is unknown.
func forgeTierPrice(classification, tier uint8) (ForgeTierPrice, bool) {
	tiers, ok := forgeClassifications[classification]
	if !ok {
		return ForgeTierPrice{}, false
	}
	p, ok := tiers[tier]
	return p, ok
}

// forgeBonus maps a uniform_random(0, 10000) roll to the fusion bonus case.
// Exact port of tools.cpp forgeBonus.
//
//	0: none
//	1: dust not consumed
//	2: cores not consumed
//	3: gold not consumed
//	4: second item retained, tier decreased
//	5: second item retained, tier unchanged
//	6: second item retained, tier increased
//	7: gain two tiers
func forgeBonus(number int) uint8 {
	switch {
	case number < 7400:
		return 0
	case number < 9000:
		return 1
	case number < 9500:
		return 2
	case number < 9525:
		return 3
	case number < 9550:
		return 4
	case number < 9950:
		return 5
	case number < 9975:
		return 6
	default:
		return 7
	}
}

// ForgeHistory is one row of the forge log (mirrors C++ ForgeHistory).
type ForgeHistory struct {
	ActionType     uint8
	Tier           uint8
	Success        bool
	TierLoss       bool
	Bonus          uint8
	Cost           uint64 // gold spent
	DustCost       uint64
	CoresCost      uint8
	Gained         uint64
	FirstItemName  string
	SecondItemName string
	CreatedAt      uint32 // unix seconds; stamped by the caller
	Convergence    bool
	Description    string
}

// ForgeResult is what the engine returns to the protocol layer so it can emit
// the 0x8A result packet. When Err is non-empty the operation failed and no
// state was mutated; the protocol should send a forge-error dialog.
type ForgeResult struct {
	ActionType  uint8
	LeftItemID  uint16
	LeftTier    uint8
	RightItemID uint16
	RightTier   uint8
	Success     bool
	Bonus       uint8
	CoreCount   uint8
	Convergence bool
	Err         string // empty on success
}

// --- Forge resource helpers (dust amount, dust level, item-backed slivers/cores) ---

// GetForgeDusts returns the current forge-dust amount.
func (p *Player) GetForgeDusts() uint64 { return p.ForgeDusts }

// SetForgeDusts sets the forge-dust amount.
func (p *Player) SetForgeDusts(v uint64) { p.ForgeDusts = v }

// AddForgeDusts increases forge dust, clamped to the stored-dust limit.
func (p *Player) AddForgeDusts(amount uint64) {
	p.ForgeDusts += amount
	if limit := uint64(p.GetForgeDustLevel()); p.ForgeDusts > limit {
		p.ForgeDusts = limit
	}
}

// RemoveForgeDusts deducts forge dust if available.
func (p *Player) RemoveForgeDusts(amount uint64) bool {
	if p.ForgeDusts < amount {
		return false
	}
	p.ForgeDusts -= amount
	return true
}

// GetForgeDustLevel returns the stored-dust limit (default 100).
func (p *Player) GetForgeDustLevel() uint16 {
	if p.ForgeDustLevel == 0 {
		return 100
	}
	return p.ForgeDustLevel
}

// AddForgeDustLevel raises the stored-dust limit, capped at ForgeMaxDust.
func (p *Player) AddForgeDustLevel(amount uint16) {
	maxDust := uint16(ForgeMaxDust())
	p.ForgeDustLevel = p.GetForgeDustLevel() + amount
	if p.ForgeDustLevel > maxDust {
		p.ForgeDustLevel = maxDust
	}
}

// GetForgeSlivers returns how many forge slivers the player holds as items.
func (p *Player) GetForgeSlivers(cat *items.Catalog) uint32 {
	return p.GetItemTypeCount(cat, ItemForgeSliver, -1)
}

// GetForgeCores returns how many exaltation cores the player holds as items.
func (p *Player) GetForgeCores(cat *items.Catalog) uint32 {
	return p.GetItemTypeCount(cat, ItemForgeCore, -1)
}

// AddForgeHistory appends an entry to the forge log, building its human-readable
// description first (mirrors Player::registerForgeHistoryDescription).
func (p *Player) AddForgeHistory(h ForgeHistory) {
	if h.Description == "" {
		h.Description = h.describe()
	}
	p.ForgeHistory = append(p.ForgeHistory, h)
}

// describe builds the history entry text shown in the 0x88 window, condensed
// from Player::registerForgeHistoryDescription.
func (h ForgeHistory) describe() string {
	conv := ""
	if h.Convergence {
		conv = " (convergence)"
	}
	switch h.ActionType {
	case ForgeActionFusion:
		outcome := "Unsuccessful"
		if h.Success {
			outcome = "Successful"
		}
		secondFate := "consumed"
		if h.Bonus == 8 {
			secondFate = "unchanged"
		}
		gold := h.Cost
		if h.Bonus == 3 { // gold refunded by the bonus
			gold = 0
		}
		return fmt.Sprintf(
			"%s fusion%s — %s (tier %d) + %s. First item: tier +1, second item: %s. Invested: %d cores, %d dust, %d gold.",
			outcome, conv, h.FirstItemName, h.Tier, h.SecondItemName, secondFate,
			h.CoresCost, h.DustCost, gold,
		)
	case ForgeActionTransfer:
		return fmt.Sprintf(
			"Transfer%s — moved tier %d from %s to %s. Invested: %d gold.",
			conv, h.Tier, h.FirstItemName, h.SecondItemName, h.Cost,
		)
	case ForgeActionDustToSliver:
		return fmt.Sprintf("Converted %d dust into %d slivers.", h.Cost, h.Gained)
	case ForgeActionSliverToCore:
		return fmt.Sprintf("Converted %d slivers into %d exalted core.", h.Cost, h.Gained)
	case ForgeActionIncreaseLimit:
		return fmt.Sprintf("Increased the dust limit (from %d) for %d dust.", h.Gained, h.Cost)
	}
	return ""
}

// --- Item lookup / classification ---

// itemClassificationOf returns the upgrade classification of an item id.
func itemClassificationOf(cat *items.Catalog, itemID uint16) uint8 {
	if cat == nil {
		return 0
	}
	if t := cat.Get(itemID); t != nil {
		return t.UpgradeClassification
	}
	return 0
}

// findForgeItem returns the first inventory item matching (itemID, tier) that is
// not the excluded instance. Mirrors Player::getForgeItemFromId's intent: find a
// concrete item of the right id and tier to consume.
func (p *Player) findForgeItem(itemID uint16, tier uint8, exclude *Item) *Item {
	var found *Item
	p.WalkInventory(func(it *Item) {
		if found != nil || it == exclude {
			return
		}
		if it.ID == itemID && it.GetTier() == tier {
			found = it
		}
	})
	return found
}

// removeItemInstance removes a single specific item instance from the inventory
// tree (equipment slot or any container). Returns false if not found.
func (p *Player) removeItemInstance(target *Item) bool {
	for slot := ConstSlotFirst; slot <= ConstSlotLast; slot++ {
		if p.Inventory[slot] == target {
			p.Inventory[slot] = nil
			return true
		}
		if it := p.Inventory[slot]; it != nil && removeInstanceFromContents(it, target) {
			return true
		}
	}
	return false
}

func removeInstanceFromContents(c, target *Item) bool {
	for i, child := range c.Contents {
		if child == target {
			c.Contents = append(c.Contents[:i], c.Contents[i+1:]...)
			return true
		}
		if child != nil && len(child.Contents) > 0 && removeInstanceFromContents(child, target) {
			return true
		}
	}
	return false
}

// addExaltationChest builds an exaltation chest containing the given forged
// items and places it in the player's inventory. Returns false if it could not
// be placed (no room). Mirrors the C++ flow of creating ITEM_EXALTATION_CHEST,
// filling it, and internalAddItem'ing it to the player.
func (p *Player) addExaltationChest(cat *items.Catalog, contents []*Item) bool {
	chest := &Item{ID: ItemExaltationChest, Count: 1, Contents: contents}
	for _, it := range contents {
		if it != nil {
			it.Parent = chest
		}
	}
	if !p.placeItem(cat, chest, ConstSlotWhereever) {
		return false
	}
	p.UpdateInventoryWeight(cat)
	return true
}

// --- Fusion ---

// ForgeFuseItems performs a fusion: it rolls success + bonus (mirroring C++
// Game::playerForgeFuseItems) and then applies the effect (Player::forgeFuseItems)
// via applyFusion. The caller (protocol) handles exhaustion checks and packet
// output.
func (p *Player) ForgeFuseItems(cat *items.Catalog, firstItemID uint16, tier uint8, secondItemID uint16, usedCore, reduceTierLoss, convergence bool) ForgeResult {
	coreCount := uint8(0)
	if usedCore {
		coreCount++
	}
	if reduceTierLoss {
		coreCount++
	}
	finalRate := ForgeBaseSuccessRate()
	if usedCore {
		finalRate += ForgeBonusSuccessRate()
	}
	success := rand.Intn(100)+1 <= finalRate
	bonus := uint8(0)
	if !convergence {
		bonus = forgeBonus(rand.Intn(10001))
	}
	return p.applyFusion(cat, firstItemID, tier, secondItemID, reduceTierLoss, convergence, success, bonus, coreCount)
}

// applyFusion mutates the inventory and charges costs for a fusion whose success
// / bonus / coreCount have already been rolled. This is the deterministic
// counterpart of Player::forgeFuseItems (only the failure tier-loss roll remains
// internal, as in C++).
func (p *Player) applyFusion(cat *items.Catalog, firstItemID uint16, tier uint8, secondItemID uint16, reduceTierLoss, convergence, success bool, bonus, coreCount uint8) ForgeResult {
	res := ForgeResult{ActionType: ForgeActionFusion, Convergence: convergence, LeftItemID: firstItemID, LeftTier: tier, RightItemID: secondItemID, RightTier: tier}
	res.Success, res.Bonus, res.CoreCount = success, bonus, coreCount

	if p.GetFreeBackpackSlots(cat) == 0 {
		res.Err = "You do not have enough room."
		return res
	}

	first := p.findForgeItem(firstItemID, tier, nil)
	if first == nil {
		res.Err = "Forge item not found."
		return res
	}
	second := p.findForgeItem(secondItemID, tier, first)
	if second == nil {
		res.Err = "Forge item not found."
		return res
	}
	classification := itemClassificationOf(cat, firstItemID)

	dustCost := uint64(ForgeFusionDustCost())
	if convergence {
		dustCost = uint64(ForgeConvergenceFusionCost())
	}

	// Pre-validate resources (read-only) before mutating anything.
	// Dust: convergence always pays; on success skipped only for bonus 1.
	if (convergence || !success || bonus != 1) && p.GetForgeDusts() < dustCost {
		res.Err = "You do not have enough dust."
		return res
	}
	// Cores: convergence needs none; on success skipped for bonus 2.
	if !convergence && (!success || bonus != 2) && coreCount != 0 &&
		p.GetForgeCores(cat) < uint32(coreCount) {
		res.Err = "You do not have enough exaltation cores."
		return res
	}
	// Gold: skipped only on success && bonus == 3.
	if convergence || !success || bonus != 3 {
		price, ok := forgeTierPrice(classification, tier+1)
		if !ok {
			res.Err = "Invalid item classification."
			return res
		}
		cost := price.RegularPrice
		if convergence {
			cost = price.ConvergenceFusionPrice
		}
		if p.GetMoney()+p.BankBalance < cost {
			res.Err = "You do not have enough money."
			return res
		}
	}

	// --- Commit: remove source items, build forged results in a chest. ---
	p.removeItemInstance(first)
	p.removeItemInstance(second)

	firstForged := &Item{ID: firstItemID, Count: 1}
	history := ForgeHistory{ActionType: ForgeActionFusion, Tier: tier, Success: success, TierLoss: reduceTierLoss, Convergence: convergence}

	if convergence {
		firstForged.SetTier(tier + 1)
		history.DustCost = dustCost
		p.RemoveForgeDusts(dustCost)
		price, _ := forgeTierPrice(classification, tier+1)
		p.RemoveMoney(price.ConvergenceFusionPrice, true)
		history.Cost = price.ConvergenceFusionPrice
		if !p.addExaltationChest(cat, []*Item{firstForged}) {
			res.Err = "You do not have enough room."
			return res
		}
	} else {
		firstForged.SetTier(tier)
		secondForged := &Item{ID: secondItemID, Count: 1}
		secondForged.SetTier(tier)
		chestContents := []*Item{firstForged, secondForged}

		if success {
			firstForged.SetTier(tier + 1)
			if bonus != 1 {
				history.DustCost = dustCost
				p.RemoveForgeDusts(dustCost)
			}
			if bonus != 2 {
				if coreCount != 0 {
					p.RemoveItemOfType(cat, ItemForgeCore, uint32(coreCount), -1, false)
				}
				history.CoresCost = coreCount
			}
			if bonus != 3 {
				if price, ok := forgeTierPrice(classification, firstForged.GetTier()); ok {
					p.RemoveMoney(price.RegularPrice, true)
					history.Cost = price.RegularPrice
				}
			}
			switch {
			case bonus == 4:
				if tier > 0 {
					secondForged.SetTier(tier - 1)
				}
			case bonus == 6:
				secondForged.SetTier(tier + 1)
			case bonus == 7 && tier+2 <= classification:
				firstForged.SetTier(tier + 2)
			}
			// The second item survives only on bonuses 4, 5, 6 (and 8, which is
			// a failure-only bonus); otherwise it is consumed.
			if bonus != 4 && bonus != 5 && bonus != 6 && bonus != 8 {
				chestContents = []*Item{firstForged}
			}
		} else {
			// Failure: roll tier loss (reduced chance when reduceTierLoss).
			lossChance := 100
			if reduceTierLoss {
				lossChance = ForgeTierLossReduction()
			}
			isTierLost := rand.Intn(100)+1 <= lossChance
			if isTierLost {
				if secondForged.GetTier() >= 1 {
					secondForged.SetTier(tier - 1)
				} else {
					chestContents = []*Item{firstForged}
				}
			}
			if isTierLost {
				bonus = 0
			} else {
				bonus = 8
			}
			history.CoresCost = coreCount
			p.RemoveForgeDusts(dustCost)
			if coreCount != 0 {
				p.RemoveItemOfType(cat, ItemForgeCore, uint32(coreCount), -1, false)
			}
			if price, ok := forgeTierPrice(classification, tier+1); ok {
				p.RemoveMoney(price.RegularPrice, true)
				history.Cost = price.RegularPrice
			}
			res.Bonus = bonus
		}
		if !p.addExaltationChest(cat, chestContents) {
			res.Err = "You do not have enough room."
			return res
		}
		res.RightTier = secondForged.GetTier()
	}

	res.RightTier = tier + 1 // packet convention: right tier is left's resulting tier
	history.FirstItemName = itemName(cat, firstItemID)
	history.SecondItemName = itemName(cat, secondItemID)
	history.Bonus = res.Bonus
	p.AddForgeHistory(history)
	return res
}

// --- Transfer ---

// ForgeTransferItemTier moves a tier from a donor item to a receiver item.
// Mirrors Player::forgeTransferItemTier.
func (p *Player) ForgeTransferItemTier(cat *items.Catalog, donorItemID uint16, tier uint8, receiveItemID uint16, convergence bool) ForgeResult {
	res := ForgeResult{ActionType: ForgeActionTransfer, Convergence: convergence, LeftItemID: donorItemID, LeftTier: tier, RightItemID: receiveItemID, Success: true}
	if convergence {
		res.RightTier = tier
	} else {
		res.RightTier = tier - 1
	}

	if p.GetFreeBackpackSlots(cat) == 0 {
		res.Err = "You do not have enough room."
		return res
	}

	donor := p.findForgeItem(donorItemID, tier, nil)
	if donor == nil {
		res.Err = "Forge item not found."
		return res
	}
	receive := p.findForgeItem(receiveItemID, 0, donor)
	if receive == nil {
		res.Err = "Forge item not found."
		return res
	}

	dustCost := uint64(ForgeTransferDustCost())
	if convergence {
		dustCost = uint64(ForgeConvergenceTransferCost())
	}
	if p.GetForgeDusts() < dustCost {
		res.Err = "You do not have enough dust."
		return res
	}

	classification := itemClassificationOf(cat, donorItemID)
	toTier := tier - 1
	if convergence {
		toTier = tier
	}
	price, ok := forgeTierPrice(classification, toTier)
	if !ok {
		res.Err = "Invalid item classification."
		return res
	}
	cost := price.RegularPrice
	if convergence {
		cost = price.ConvergenceTransferPrice
	}
	coresAmount := price.CorePrice

	if p.GetForgeCores(cat) < uint32(coresAmount) {
		res.Err = "You do not have enough exaltation cores."
		return res
	}
	if p.GetMoney()+p.BankBalance < cost {
		res.Err = "You do not have enough money."
		return res
	}

	// Commit.
	p.removeItemInstance(donor)
	p.removeItemInstance(receive)

	newReceive := &Item{ID: receiveItemID, Count: 1}
	if convergence {
		newReceive.SetTier(tier)
	} else {
		newReceive.SetTier(tier - 1)
	}
	if !p.addExaltationChest(cat, []*Item{newReceive}) {
		res.Err = "You do not have enough room."
		return res
	}

	p.RemoveForgeDusts(dustCost)
	if coresAmount != 0 {
		p.RemoveItemOfType(cat, ItemForgeCore, uint32(coresAmount), -1, false)
	}
	p.RemoveMoney(cost, true)

	p.AddForgeHistory(ForgeHistory{
		ActionType:     ForgeActionTransfer,
		Tier:           tier,
		Success:        true,
		Cost:           cost,
		FirstItemName:  itemName(cat, donorItemID),
		SecondItemName: itemName(cat, receiveItemID),
		Convergence:    convergence,
	})
	return res
}

// --- Resource conversion ---

// ForgeResourceConversion handles dust->slivers, slivers->core, and raising the
// stored-dust limit. Mirrors Player::forgeResourceConversion. Returns false when
// the player lacks the resources.
func (p *Player) ForgeResourceConversion(cat *items.Catalog, actionType uint8) bool {
	switch actionType {
	case ForgeActionDustToSliver:
		sliverAmount := uint32(ForgeSliverAmount())
		cost := uint64(ForgeCostOneSliver()) * uint64(sliverAmount)
		if p.GetForgeDusts() < cost {
			return false
		}
		if _, ok := p.InternalAddItem(cat, ItemForgeSliver, sliverAmount, -1, ConstSlotWhereever); !ok {
			return false
		}
		p.RemoveForgeDusts(cost)
		p.AddForgeHistory(ForgeHistory{ActionType: actionType, Success: true, Cost: cost, Gained: uint64(sliverAmount)})
		return true

	case ForgeActionSliverToCore:
		coreCost := uint32(ForgeCoreCost())
		if p.GetForgeSlivers(cat) < coreCost {
			return false
		}
		if !p.RemoveItemOfType(cat, ItemForgeSliver, coreCost, -1, false) {
			return false
		}
		if _, ok := p.InternalAddItem(cat, ItemForgeCore, 1, -1, ConstSlotWhereever); !ok {
			return false
		}
		p.AddForgeHistory(ForgeHistory{ActionType: actionType, Success: true, Cost: uint64(coreCost), Gained: 1})
		return true

	case ForgeActionIncreaseLimit:
		level := p.GetForgeDustLevel()
		if level >= uint16(ForgeMaxDust()) {
			return false
		}
		upgradeCost := uint64(level) - ForgeDustLevelBase
		if p.GetForgeDusts() < upgradeCost {
			return false
		}
		p.RemoveForgeDusts(upgradeCost)
		p.AddForgeDustLevel(1)
		p.AddForgeHistory(ForgeHistory{ActionType: actionType, Success: true, Cost: upgradeCost, Gained: uint64(level)})
		return true
	}
	return false
}

// itemName returns an item's display name from the catalog, or "" if unknown.
func itemName(cat *items.Catalog, id uint16) string {
	if cat == nil {
		return ""
	}
	if t := cat.Get(id); t != nil {
		return t.Name
	}
	return ""
}
