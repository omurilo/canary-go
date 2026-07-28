package protocol

import (
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

