package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/mounts"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// SendOutfitWindow sends the character customization dialog (Opcode 0xC8).
func (g *GameProtocol) SendOutfitWindow() {
	p := g.player
	if p == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0xC8) // SendOutfitWindow opcode

	// Current outfit:
	currentOutfit := p.Outfit
	if currentOutfit.LookType == 0 {
		if p.Sex == 0 {
			currentOutfit.LookType = 136 // Default female Citizen
		} else {
			currentOutfit.LookType = 128 // Default male Citizen
		}
	}
	addOutfit(w, currentOutfit)

	// Mount colors (if currentOutfit.LookMount == 0):
	if currentOutfit.LookMount == 0 {
		w.AddByte(0) // mountHead
		w.AddByte(0) // mountBody
		w.AddByte(0) // mountLegs
		w.AddByte(0) // mountFeet
	}
	w.AddU16(currentOutfit.FamiliarsType) // familiar lookType

	type outfitEntry struct {
		lookType uint16
		name     string
		addons   byte
		store    byte
	}

	var outfits []outfitEntry

	if p.GroupID >= 3 || p.AccountType >= 5 {
		outfits = append(outfits,
			outfitEntry{lookType: 75, name: "Gamemaster", addons: 0, store: 0},
			outfitEntry{lookType: 266, name: "Customer Support", addons: 0, store: 0},
			outfitEntry{lookType: 302, name: "Community Manager", addons: 0, store: 0},
		)
	}

	if p.Sex == 0 { // Female
		outfits = append(outfits,
			outfitEntry{lookType: 136, name: "Citizen", addons: 3, store: 0},
			outfitEntry{lookType: 137, name: "Hunter", addons: 3, store: 0},
			outfitEntry{lookType: 138, name: "Mage", addons: 3, store: 0},
			outfitEntry{lookType: 139, name: "Knight", addons: 3, store: 0},
			outfitEntry{lookType: 140, name: "Noblewoman", addons: 3, store: 0},
			outfitEntry{lookType: 141, name: "Summoner", addons: 3, store: 0},
			outfitEntry{lookType: 142, name: "Warrior", addons: 3, store: 0},
			outfitEntry{lookType: 147, name: "Barbarian", addons: 3, store: 0},
			outfitEntry{lookType: 148, name: "Druid", addons: 3, store: 0},
			outfitEntry{lookType: 149, name: "Wizard", addons: 3, store: 0},
			outfitEntry{lookType: 150, name: "Oriental", addons: 3, store: 0},
			outfitEntry{lookType: 155, name: "Pirate", addons: 3, store: 0},
			outfitEntry{lookType: 156, name: "Assassin", addons: 3, store: 0},
			outfitEntry{lookType: 157, name: "Beggar", addons: 3, store: 0},
			outfitEntry{lookType: 158, name: "Shaman", addons: 3, store: 0},
			outfitEntry{lookType: 252, name: "Norsewoman", addons: 3, store: 0},
			outfitEntry{lookType: 269, name: "Nightmare", addons: 3, store: 0},
			outfitEntry{lookType: 270, name: "Jester", addons: 3, store: 0},
			outfitEntry{lookType: 279, name: "Brotherhood", addons: 3, store: 0},
			outfitEntry{lookType: 288, name: "Demon Hunter", addons: 3, store: 0},
			outfitEntry{lookType: 324, name: "Yalaharian", addons: 3, store: 0},
			outfitEntry{lookType: 336, name: "Warmaster", addons: 3, store: 0},
			outfitEntry{lookType: 366, name: "Wayfarer", addons: 3, store: 0},
		)
	} else { // Male
		outfits = append(outfits,
			outfitEntry{lookType: 128, name: "Citizen", addons: 3, store: 0},
			outfitEntry{lookType: 129, name: "Hunter", addons: 3, store: 0},
			outfitEntry{lookType: 130, name: "Mage", addons: 3, store: 0},
			outfitEntry{lookType: 131, name: "Knight", addons: 3, store: 0},
			outfitEntry{lookType: 132, name: "Nobleman", addons: 3, store: 0},
			outfitEntry{lookType: 133, name: "Summoner", addons: 3, store: 0},
			outfitEntry{lookType: 134, name: "Warrior", addons: 3, store: 0},
			outfitEntry{lookType: 143, name: "Barbarian", addons: 3, store: 0},
			outfitEntry{lookType: 144, name: "Druid", addons: 3, store: 0},
			outfitEntry{lookType: 145, name: "Wizard", addons: 3, store: 0},
			outfitEntry{lookType: 146, name: "Oriental", addons: 3, store: 0},
			outfitEntry{lookType: 151, name: "Pirate", addons: 3, store: 0},
			outfitEntry{lookType: 152, name: "Assassin", addons: 3, store: 0},
			outfitEntry{lookType: 153, name: "Beggar", addons: 3, store: 0},
			outfitEntry{lookType: 154, name: "Shaman", addons: 3, store: 0},
			outfitEntry{lookType: 251, name: "Norseman", addons: 3, store: 0},
			outfitEntry{lookType: 268, name: "Nightmare", addons: 3, store: 0},
			outfitEntry{lookType: 273, name: "Jester", addons: 3, store: 0},
			outfitEntry{lookType: 278, name: "Brotherhood", addons: 3, store: 0},
			outfitEntry{lookType: 289, name: "Demon Hunter", addons: 3, store: 0},
			outfitEntry{lookType: 325, name: "Yalaharian", addons: 3, store: 0},
			outfitEntry{lookType: 335, name: "Warmaster", addons: 3, store: 0},
			outfitEntry{lookType: 367, name: "Wayfarer", addons: 3, store: 0},
		)
	}

	w.AddU16(uint16(len(outfits)))
	for _, o := range outfits {
		w.AddU16(o.lookType)
		w.AddString(o.name)
		w.AddByte(o.addons)
		w.AddByte(o.store)
		if o.store == 1 {
			w.AddU32(0) // store price
		}
	}

	// Mounts count & list
	var availableMounts []mounts.Mount
	for _, m := range mounts.All() {
		if p.GroupID >= 3 || p.AccountType >= 5 || p.HasMount(m.ID) {
			availableMounts = append(availableMounts, m)
		}
	}

	w.AddU16(uint16(len(availableMounts)))
	for _, m := range availableMounts {
		w.AddU16(m.ClientID)
		w.AddString(m.Name)
		w.AddByte(0) // store: 0
	}

	// Familiars count (uint16) - 0
	w.AddU16(0)

	// Try outfit byte
	w.AddByte(0)
	// Mounted byte
	mountedByte := byte(0)
	if currentOutfit.LookMount != 0 {
		mountedByte = 1
	}
	w.AddByte(mountedByte)
	// Random mount byte
	w.AddByte(0)

	g.SendToClient(w)
}

// parseSetOutfit handles Opcode 0xD3 when the player selects outfit/colors/addons.
func (g *GameProtocol) parseSetOutfit(r *netmsg.Reader) {
	p := g.player
	if p == nil {
		return
	}

	outfitType := r.GetByte()
	lookType := r.GetU16()
	lookHead := uint8(minInt(132, int(r.GetByte())))
	lookBody := uint8(minInt(132, int(r.GetByte())))
	lookLegs := uint8(minInt(132, int(r.GetByte())))
	lookFeet := uint8(minInt(132, int(r.GetByte())))
	lookAddons := r.GetByte()

	if outfitType == 0 {
		lookMount := r.GetU16()
		if r.Remaining() >= 4 {
			_ = r.GetByte() // mount head
			_ = r.GetByte() // mount body
			_ = r.GetByte() // mount legs
			_ = r.GetByte() // mount feet
		}
		if r.Remaining() >= 1 {
			_ = r.GetByte() // set mount
		}
		if r.Remaining() >= 2 {
			_ = r.GetU16() // familiar
		}
		if r.Remaining() >= 1 {
			_ = r.GetByte() // mount randomized
		}

		p.Outfit.LookType = lookType
		p.Outfit.Head = lookHead
		p.Outfit.Body = lookBody
		p.Outfit.Legs = lookLegs
		p.Outfit.Feet = lookFeet
		p.Outfit.Addons = lookAddons
		p.Outfit.LookMount = lookMount
		if lookMount != 0 {
			p.LastMount = lookMount
		}
	}

	g.broadcastOutfit(p)
}

// parseToggleMount handles Opcode 0xD4 (Ctrl+R / Mount toggle).
func (g *GameProtocol) parseToggleMount(r *netmsg.Reader) {
	p := g.player
	if p == nil {
		return
	}

	mount := r.GetByte() != 0
	if mount {
		if p.Outfit.LookMount == 0 {
			if p.LastMount != 0 {
				p.Outfit.LookMount = p.LastMount
			} else if len(p.Mounts) > 0 {
				// Pick the first available mount if they have no LastMount
				for mID, has := range p.Mounts {
					if has {
						p.Outfit.LookMount = mID
						break
					}
				}
			} else {
				// Player has no mounts, send cancel message
				g.sendStatusText("You don't have any mounts.")
				return
			}
		}
	} else {
		if p.Outfit.LookMount != 0 {
			p.LastMount = p.Outfit.LookMount
			p.Outfit.LookMount = 0
		}
	}

	g.broadcastOutfit(p)
}

// SendCreatureOutfit sends opcode 0x8E (opCreatureOutfit) for a given creature.
func (g *GameProtocol) SendCreatureOutfit(c game.Creature, o game.Outfit) {
	// canSee-gated in C++ (protocolgame.cpp sendCreatureOutfit).
	if c == nil || !g.canSee(c.GetPosition()) {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0x8E) // opCreatureOutfit
	w.AddU32(c.GetID())
	addOutfit(w, o)
	g.SendToClient(w)
}

// broadcastOutfit sends the updated outfit of player p to all nearby spectators and p.
func (g *GameProtocol) broadcastOutfit(p *game.Player) {
	for _, s := range g.deps.World.Spectators(p.GetPosition(), 0) {
		if gp, ok := s.Session.(*GameProtocol); ok {
			gp.SendCreatureOutfit(p, p.Outfit)
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
