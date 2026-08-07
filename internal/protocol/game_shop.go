package protocol

import (
	"strings"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// Resource_t values (server_definitions.hpp).
const (
	resourceBank           = 0x00
	resourceInventoryMoney = 0x01
)

// sendResourceBalance sends a 0xEE resource update (bank / inventory money),
// which the client shop and status bar read to show the player's funds.
func (g *GameProtocol) sendResourceBalance(resource byte, value uint64) {
	w := netmsg.NewWriter()
	w.AddByte(0xEE)
	w.AddByte(resource)
	w.AddU64(value)
	g.SendToClient(w)
}

// sendShopGoods sends the 0x7B sale-item list plus the resource balances, so the
// shop window reflects the player's money and how many of each sellable item
// they own. Mirrors ProtocolGame::sendSaleItemList.
func (g *GameProtocol) sendShopGoods() {
	p := g.player
	g.sendResourceBalance(resourceBank, p.BankBalance)
	g.sendResourceBalance(resourceInventoryMoney, p.GetMoney())

	// Count, per shop item, how many the player owns (for the sell column).
	counts := map[uint16]uint32{}
	for _, it := range playerAllItems(p) {
		counts[it.ID] += uint32(itemCountOf(it))
	}

	var shopItems []game.Item
	if p.ShopOwnerID != 0 {
		if npc, ok := g.deps.World.CreatureByID(p.ShopOwnerID).(*game.Npc); ok {
			if nt := g.deps.World.TypeRegistry.Npcs[strings.ToLower(npc.Name)]; nt != nil {
				w := netmsg.NewWriter()
				w.AddByte(0x7B)
				var n uint16
				body := netmsg.NewWriter()
				seen := map[uint16]bool{}
				for _, si := range nt.ShopItems {
					if si.SellPrice == 0 || seen[si.ID] {
						continue
					}
					if c := counts[si.ID]; c > 0 {
						seen[si.ID] = true
						body.AddU16(si.ID)
						cc := c
						if cc > 0xFFFF {
							cc = 0xFFFF
						}
						body.AddU16(uint16(cc))
						n++
					}
				}
				w.AddU16(n)
				w.AddBytes(body.Bytes())
				g.SendToClient(w)
				return
			}
		}
	}
	_ = shopItems
}

// playerAllItems returns every item carried by the player: equipped items and
// the (recursive) contents of any containers they hold.
func playerAllItems(p *game.Player) []*game.Item {
	var out []*game.Item
	var walk func(items []*game.Item)
	walk = func(items []*game.Item) {
		for _, it := range items {
			if it == nil {
				continue
			}
			out = append(out, it)
			if it.Container != nil && len(it.Container.Contents) > 0 {
				walk(it.Container.Contents)
			}
		}
	}
	walk(p.Inventory[:])
	return out
}

func itemCountOf(it *game.Item) uint16 {
	if it.Count == 0 {
		return 1
	}
	return it.Count
}
