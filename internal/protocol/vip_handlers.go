package protocol

import (
	"slices"
	"strings"

	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// SendVIPList sends the VIP list (Opcode 0xD4).
func (g *GameProtocol) SendVIPList() {
	if g.player == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0xD4)

	w.AddU16(uint16(len(g.player.VIPGroups)))
	for _, group := range g.player.VIPGroups {
		w.AddU32(group.ID)
		w.AddString(group.Name)
		w.AddByte(boolToByte(group.Customizable))
	}

	w.AddU16(uint16(len(g.player.VIPList)))
	for _, entry := range g.player.VIPList {
		w.AddU32(entry.PlayerID)
		w.AddString(entry.PlayerName)
		w.AddByte(entry.Icon)
		w.AddByte(boolByte(entry.Notify))

		online := g.playerOnlineInVIP(entry.PlayerID)
		w.AddByte(boolByte(online))

		w.AddU16(uint16(len(entry.Groups)))
		for _, gid := range entry.Groups {
			w.AddU32(gid)
		}
	}

	g.SendToClient(w)
}

// playerOnlineInVIP checks if a player (by DBID) is online and visible.
func (g *GameProtocol) playerOnlineInVIP(dbID uint32) bool {
	return g.deps.World.PlayerByDBID(dbID) != nil
}

// SendVIPOnline sends a notification that a VIP player came online (Opcode 0xD5).
func (g *GameProtocol) SendVIPOnline(playerID uint32) {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xD5)
	w.AddU32(playerID)
	g.SendToClient(w)
}

// SendVIPOffline sends a notification that a VIP player went offline (Opcode 0xD6).
func (g *GameProtocol) SendVIPOffline(playerID uint32) {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xD6)
	w.AddU32(playerID)
	g.SendToClient(w)
}

// parseVIPAdd handles client request to add a VIP entry (Opcode 0xDC).
func (g *GameProtocol) parseVIPAdd(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	playerName := strings.TrimSpace(r.GetString())
	if playerName == "" {
		return
	}

	// Look up online player by name
	target := g.deps.World.PlayerByName(playerName)
	if target == nil || target.DBID == 0 {
		// Player might exist offline; we require them to be online to add
		g.player.SendTextMessage(0x14, "A player with this name does not exist.")
		return
	}

	// Check if already in VIP list
	for _, e := range g.player.VIPList {
		if e.PlayerID == target.DBID {
			g.player.SendTextMessage(0x14, "This player is already on your VIP list.")
			return
		}
	}

	g.player.VIPList = append(g.player.VIPList, game.VIPEntry{
		PlayerID:   target.DBID,
		PlayerName: target.Name,
		Icon:       0,
		Notify:     true,
	})

	g.player.SendTextMessage(0x13, target.Name+" has been added to your VIP list.")
	g.SendVIPList()
}

// parseVIPRemove handles client request to remove a VIP entry (Opcode 0xDD).
func (g *GameProtocol) parseVIPRemove(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	playerID := r.GetU32()

	for i, e := range g.player.VIPList {
		if e.PlayerID == playerID {
			g.player.VIPList = append(g.player.VIPList[:i], g.player.VIPList[i+1:]...)
			g.player.SendTextMessage(0x13, e.PlayerName+" has been removed from your VIP list.")
			g.SendVIPList()
			return
		}
	}
}

// parseVIPEdit handles client request to edit a VIP entry (icon, notify, groups) (Opcode 0xDE).
func (g *GameProtocol) parseVIPEdit(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	guid := r.GetU32()
	description := r.GetString()
	icon := r.GetU32()
	if icon > 10 {
		icon = 10
	}
	notify := r.GetByte() != 0
	groupsAmount := r.GetByte()

	var groupsID []uint32
	for range groupsAmount {
		gid := r.GetU32()
		groupsID = append(groupsID, gid)
	}

	// Find and update the VIP entry
	for i := range g.player.VIPList {
		if g.player.VIPList[i].PlayerID == guid {
			g.player.VIPList[i].Description = description
			g.player.VIPList[i].Icon = uint8(icon)
			g.player.VIPList[i].Notify = notify
			g.player.VIPList[i].Groups = groupsID
			break
		}
	}

	g.SendVIPList()
}

// parseVipGroupActions handles VIP group management (add/edit/remove) (Opcode 0xDF).
func (g *GameProtocol) parseVipGroupActions(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	action := r.GetByte()

	switch action {
	case 0x01: // Add group
		groupName := r.GetString()
		// Check for duplicate name
		for _, grp := range g.player.VIPGroups {
			if grp.Name == groupName {
				g.player.SendTextMessage(0x14, "A group with this name already exists. Please choose another name.")
				return
			}
		}
		// Find free ID (1-8)
		var freeID uint32 = 1
	loop:
		for freeID <= 8 {
			for _, grp := range g.player.VIPGroups {
				if grp.ID == freeID {
					freeID++
					continue loop
				}
			}
			break
		}
		if freeID > 8 {
			g.player.SendTextMessage(0x14, "No free VIP group ID available.")
			return
		}
		g.player.VIPGroups = append(g.player.VIPGroups, game.VIPGroup{
			ID:           freeID,
			Name:         groupName,
			Customizable: true,
		})

	case 0x02: // Edit group
		groupID := r.GetU32()
		newName := r.GetString()
		for i := range g.player.VIPGroups {
			if g.player.VIPGroups[i].ID == groupID {
				g.player.VIPGroups[i].Name = newName
				break
			}
		}

	case 0x03: // Remove group
		groupID := r.GetU32()
		groupIdx := -1
		for i, grp := range g.player.VIPGroups {
			if grp.ID == groupID {
				groupIdx = i
				break
			}
		}
		if groupIdx >= 0 {
			g.player.VIPGroups = append(g.player.VIPGroups[:groupIdx], g.player.VIPGroups[groupIdx+1:]...)
		}

		// Also remove this group from all VIP entries
		for i := range g.player.VIPList {
			entry := &g.player.VIPList[i]
			entry.Groups = slices.DeleteFunc(entry.Groups, func(gid uint32) bool {
				return gid == groupID
			})
		}
	}

	g.SendVIPList()
}

