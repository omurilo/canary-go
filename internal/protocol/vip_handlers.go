package protocol

import (
	"slices"
	"strings"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// VipStatus_t values (src/creatures/creatures_definitions.hpp:783).
const (
	vipStatusOffline  = 0
	vipStatusOnline   = 1
	vipStatusPending  = 2
	vipStatusTraining = 3
)

// maxVipGroupEntries mirrors PlayerVIP::getMaxGroupEntries for a premium account
// (src/creatures/players/components/player_vip.cpp:34): 5 custom + 3 default.
const maxVipGroupEntries = 8

// SendVIPList sends the VIP groups followed by one packet per VIP entry.
//
// This whole area used to be a single invented packet on 0xD4 carrying groups and
// entries together, with u16 counts and u32 group ids. Canary 13.x splits it:
// 0xD4 = sendVIPGroups, and one 0xD2 = sendVIP per entry. 0xD5/0xD6 (previously
// used for online/offline) are not VIP opcodes at all.
func (g *GameProtocol) SendVIPList() {
	if g.player == nil {
		return
	}
	g.sendVIPGroups()
	for _, entry := range g.player.VIPList {
		g.sendVIPEntry(entry)
	}
}

// sendVIPGroups ports ProtocolGame::sendVIPGroups (protocolgame.cpp:9293):
// [0xD4][u8 groupCount] per group { u8 id, str name, u8 customizable }
// [u8 remainingGroupSlots]
func (g *GameProtocol) sendVIPGroups() {
	groups := g.player.VIPGroups

	w := netmsg.NewWriter()
	w.AddByte(0xD4)
	w.AddByte(uint8(len(groups)))
	for _, group := range groups {
		w.AddByte(uint8(group.ID))
		w.AddString(group.Name)
		w.AddByte(boolToByte(group.Customizable))
	}

	remaining := 0
	if maxVipGroupEntries > len(groups) {
		remaining = maxVipGroupEntries - len(groups)
	}
	w.AddByte(uint8(remaining))

	g.SendToClient(w)
}

// sendVIPEntry ports ProtocolGame::sendVIP (protocolgame.cpp:9260):
// [0xD2][u32 guid][str name][str description][u32 min(10,icon)][u8 notify]
// [u8 status][u8 groupCount][u8 groupIds...]
func (g *GameProtocol) sendVIPEntry(entry game.VIPEntry) {
	icon := uint32(entry.Icon)
	if icon > 10 {
		icon = 10
	}

	status := uint8(vipStatusOffline)
	if g.playerOnlineInVIP(entry.PlayerID) {
		status = vipStatusOnline
	}

	w := netmsg.NewWriter()
	w.AddByte(0xD2)
	w.AddU32(entry.PlayerID)
	w.AddString(entry.PlayerName)
	w.AddString(entry.Description)
	w.AddU32(icon)
	w.AddByte(boolByte(entry.Notify))
	w.AddByte(status)
	w.AddByte(uint8(len(entry.Groups)))
	for _, gid := range entry.Groups {
		w.AddByte(uint8(gid))
	}

	g.SendToClient(w)
}

// playerOnlineInVIP checks if a player (by DBID) is online and visible.
func (g *GameProtocol) playerOnlineInVIP(dbID uint32) bool {
	return g.deps.World.PlayerByDBID(dbID) != nil
}

// SendUpdatedVIPStatus ports ProtocolGame::sendUpdatedVIPStatus
// (protocolgame.cpp:9241): [0xD3][u32 guid][u8 status]. It replaces the previous
// SendVIPOnline/SendVIPOffline pair, which used 0xD5/0xD6 — neither of which is a
// VIP opcode in 13.x.
func (g *GameProtocol) SendUpdatedVIPStatus(playerID uint32, status uint8) {
	if g.player == nil {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(0xD3)
	w.AddU32(playerID)
	w.AddByte(status)
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

