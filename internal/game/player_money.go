package game

const (
	GoldCoinID     = 2148
	PlatinumCoinID = 2152
	CrystalCoinID  = 2160
)

func coinValue(id uint16) uint64 {
	switch id {
	case GoldCoinID:
		return 1
	case PlatinumCoinID:
		return 100
	case CrystalCoinID:
		return 10000
	}
	return 0
}

// GetMoney returns the total money found in the item tree.
func GetMoney(items []*Item) uint64 {
	var total uint64
	for _, it := range items {
		if it == nil {
			continue
		}
		total += coinValue(it.ID) * uint64(it.Count)
		if len(it.Contents) > 0 {
			total += GetMoney(it.Contents)
		}
	}
	return total
}

func (p *Player) GetMoney() uint64 {
	return GetMoney(p.Inventory[:])
}

func (p *Player) RemoveMoney(amount uint64) bool {
	total := p.GetMoney()
	if total < amount {
		return false
	}

	// Delete all coins in the inventory and its containers
	p.deleteAllCoins(p.Inventory[:])

	// Add the remaining change back to the player
	p.AddMoney(total - amount)

	return true
}

func (p *Player) deleteAllCoins(items []*Item) {
	for i := range items {
		if items[i] == nil {
			continue
		}
		if items[i].ID == GoldCoinID || items[i].ID == PlatinumCoinID || items[i].ID == CrystalCoinID {
			items[i] = nil
		} else if len(items[i].Contents) > 0 {
			p.deleteAllCoins(items[i].Contents)
			compacted := items[i].Contents[:0]
			for _, child := range items[i].Contents {
				if child != nil {
					compacted = append(compacted, child)
				}
			}
			items[i].Contents = compacted
		}
	}
}

func (p *Player) AddMoney(amount uint64) {
	crystals := amount / 10000
	amount %= 10000
	platinums := amount / 100
	amount %= 100
	golds := amount

	if crystals > 0 {
		p.AddItem(CrystalCoinID, uint64(crystals))
	}
	if platinums > 0 {
		p.AddItem(PlatinumCoinID, uint64(platinums))
	}
	if golds > 0 {
		p.AddItem(GoldCoinID, uint64(golds))
	}
}

func (p *Player) AddItem(id uint16, count uint64) {
	if count == 0 {
		return
	}
	for count > 0 {
		add := uint16(100)
		if count < 100 {
			add = uint16(count)
		}
		item := &Item{ID: id, Count: add}

		added := false
		// Try to put it in an existing container first
		for i := 1; i < len(p.Inventory); i++ {
			if p.Inventory[i] != nil && p.Inventory[i].Contents != nil {
				p.Inventory[i].Contents = append(p.Inventory[i].Contents, item)
				added = true
				break
			}
		}
		// If no container, put it in an empty slot
		if !added {
			for i := 1; i < len(p.Inventory); i++ {
				if p.Inventory[i] == nil {
					p.Inventory[i] = item
					added = true
					break
				}
			}
		}
		count -= uint64(add)
	}
}
