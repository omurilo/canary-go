package game

type OutfitEntry struct {
	LookType uint16
	Addons   uint8
}

func (p *Player) AddOutfit(lookType uint16, addons uint8) {
	for i := range p.Outfits {
		if p.Outfits[i].LookType == lookType {
			p.Outfits[i].Addons |= addons
			return
		}
	}
	p.Outfits = append(p.Outfits, OutfitEntry{LookType: lookType, Addons: addons})
}

func (p *Player) RemoveOutfit(lookType uint16) bool {
	for i, entry := range p.Outfits {
		if entry.LookType == lookType {
			p.Outfits = append(p.Outfits[:i], p.Outfits[i+1:]...)
			return true
		}
	}
	return false
}

func (p *Player) HasOutfit(lookType uint16) bool {
	for _, entry := range p.Outfits {
		if entry.LookType == lookType {
			return true
		}
	}
	return false
}

func (p *Player) GetOutfitAddons(lookType uint16) uint8 {
	for _, entry := range p.Outfits {
		if entry.LookType == lookType {
			return entry.Addons
		}
	}
	return 0
}
