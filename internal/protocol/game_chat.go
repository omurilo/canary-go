package protocol

import (
	"strings"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// ---------------------------------------------------------------------------
// Outbound packet helpers (matching the C++ ProtocolGame send* methods).
// ---------------------------------------------------------------------------

// sendChannelsDialog sends the channel list (opcode 0xAB).
func (g *GameProtocol) sendChannelsDialog(channels []*game.ChatChannel) {
	w := netmsg.NewWriter()
	w.AddByte(0xAB)
	w.AddByte(byte(len(channels)))
	for _, ch := range channels {
		w.AddU16(ch.ID)
		w.AddString(ch.Name)
	}
	g.SendToClient(w)
}

// sendChannel opens a channel window (opcode 0xAC).
// users may be nil (public channels send 0 users, matching C++ behaviour).
func (g *GameProtocol) sendChannel(channel *game.ChatChannel, users, invited map[uint32]*game.Player) {
	w := netmsg.NewWriter()
	w.AddByte(0xAC)
	w.AddU16(channel.ID)
	w.AddString(channel.Name)

	// Protocol >= 910 sends user and invited-user lists.
	addChannelUserNames(w, users)
	addChannelUserNames(w, invited)

	g.SendToClient(w)
}

// sendToChannel sends a message to a specific channel (opcode 0xAA with channel data).
func (g *GameProtocol) sendToChannel(statementID uint32, speakerName string, speakerLevel uint16, talkType byte, channelID uint16, text string) {
	w := netmsg.NewWriter()
	w.AddByte(0xAA)
	w.AddU32(statementID)
	w.AddString(speakerName)
	if statementID > 0 {
		w.AddByte(0) // suffix (show/traded) for version >= 1281
	}
	w.AddU16(speakerLevel)
	w.AddByte(talkType)
	// Channel messages carry a channelID; say/yell/whisper carry a position.
	if talkType >= 5 { // MessageChannel and above
		w.AddU16(channelID)
	} else {
		w.AddU16(0) // position x
		w.AddU16(0) // position y
		w.AddByte(0) // position z
	}
	if text == "" {
		text = " " // empty text causes client underflow
	}
	w.AddString(text)
	g.SendToClient(w)
}

// sendOpenPrivateChannel opens a private message tab (opcode 0xAD).
func (g *GameProtocol) sendOpenPrivateChannel(receiver string) {
	w := netmsg.NewWriter()
	w.AddByte(0xAD)
	w.AddString(receiver)
	g.SendToClient(w)
}

// sendCreatePrivateChannel confirms private channel creation (opcode 0xB2).
func (g *GameProtocol) sendCreatePrivateChannel(channelID uint16, channelName string, ownerName string) {
	w := netmsg.NewWriter()
	w.AddByte(0xB2)
	w.AddU16(channelID)
	w.AddString(channelName)
	w.AddU16(1)        // number of invitees that can join
	w.AddString(ownerName)
	w.AddU16(0)        // number of invited users list
	g.SendToClient(w)
}

// sendClosePrivateChannel closes a private channel (opcode 0xB3).
func (g *GameProtocol) sendClosePrivateChannel(channelID uint16) {
	w := netmsg.NewWriter()
	w.AddByte(0xB3)
	w.AddU16(channelID)
	g.SendToClient(w)
}

// sendChannelEvent notifies channel join/leave/invite/exclude (opcode 0xF3).
func (g *GameProtocol) sendChannelEvent(channelID uint16, playerName string, event byte) {
	w := netmsg.NewWriter()
	w.AddByte(0xF3)
	w.AddU16(channelID)
	w.AddString(playerName)
	w.AddByte(event)
	g.SendToClient(w)
}

// ---------------------------------------------------------------------------
// Session interface implementations for chat.
// ---------------------------------------------------------------------------

func (g *GameProtocol) SendTextMessage(class uint8, text string) {
	w := netmsg.NewWriter()
	w.AddByte(0xB4)
	w.AddByte(class)
	w.AddString(text)
	g.SendToClient(w)
}

func (g *GameProtocol) SendChannelEvent(channelID uint16, playerName string, event byte) {
	g.sendChannelEvent(channelID, playerName, event)
}

func (g *GameProtocol) SendClosePrivateChannel(channelID uint16) {
	g.sendClosePrivateChannel(channelID)
}

func (g *GameProtocol) SendToChannel(statementID uint32, fromName string, level uint16, talkType byte, channelID uint16, text string) {
	// Channel messages use statementID=0 (matching C++ sendChannelMessage).
	g.sendToChannel(0, fromName, level, talkType, channelID, text)
}

func (g *GameProtocol) SendChannelsDialog(channels []*game.ChatChannel) {
	g.sendChannelsDialog(channels)
}

func (g *GameProtocol) SendOpenChannel(channel *game.ChatChannel) {
	// For public channels, send nil (the client knows all users).
	var users, invited map[uint32]*game.Player
	if !channel.IsPublic() {
		users = channel.GetUsersSnapshot()
		if pcc, ok := channel.GetPrivateChannel(); ok {
			invited = pcc.Invites
		}
	}
	g.sendChannel(channel, users, invited)
}

func (g *GameProtocol) SendOpenPrivateChannel(receiver string) {
	g.sendOpenPrivateChannel(receiver)
}

func (g *GameProtocol) SendCreatePrivateChannel(channelID uint16, channelName string, ownerName string) {
	g.sendCreatePrivateChannel(channelID, channelName, ownerName)
}

// ---------------------------------------------------------------------------
// Inbound packet parsers.
// ---------------------------------------------------------------------------

// parseRequestChannels handles C++ 0x97 → playerRequestChannels.
func (g *GameProtocol) parseRequestChannels(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	channels := g.deps.World.GetChannelList(g.player)
	g.sendChannelsDialog(channels)
}

// parseOpenChannel handles C++ 0x98 → playerOpenChannel.
func (g *GameProtocol) parseOpenChannel(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	channelID := r.GetU16()
	channel := g.deps.World.AddUserToChannel(g.player, channelID)
	if channel == nil {
		return
	}

	// For public channels send nil (the client knows all users).
	var users, invited map[uint32]*game.Player
	if !channel.IsPublic() {
		users = channel.GetUsersSnapshot()
		if pcc, ok := channel.GetPrivateChannel(); ok {
			invited = pcc.Invites
		}
	}
	g.sendChannel(channel, users, invited)
}

// parseOpenPrivateChannel handles C++ 0x9A → playerOpenPrivateChannel.
func (g *GameProtocol) parseOpenPrivateChannel(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	receiver := r.GetString()
	if strings.EqualFold(receiver, g.player.Name) {
		return // can't PM yourself
	}
	g.sendOpenPrivateChannel(receiver)
}

// parseCreatePrivateChannel handles C++ 0xAA → playerCreatePrivateChannel.
func (g *GameProtocol) parseCreatePrivateChannel(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	channel := g.deps.World.CreatePrivateChannel(g.player)
	if channel == nil {
		return
	}
	channel.AddUser(g.player)
	g.sendCreatePrivateChannel(channel.ID, channel.Name, g.player.Name)
}

// parseChannelInvite handles C++ 0xAB → playerChannelInvite.
func (g *GameProtocol) parseChannelInvite(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	name := r.GetString()
	g.deps.World.InviteToPrivateChannel(g.player.ID, name)
}

// parseChannelExclude handles C++ 0xAC → playerChannelExclude.
func (g *GameProtocol) parseChannelExclude(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	name := r.GetString()
	g.deps.World.ExcludeFromPrivateChannel(g.player.ID, name)
}

// addChannelUserNames serialises a list of player names (for sendChannel).
func addChannelUserNames(w *netmsg.Writer, users map[uint32]*game.Player) {
	if users == nil {
		w.AddU16(0)
		return
	}
	w.AddU16(uint16(len(users)))
	for _, p := range users {
		if p != nil {
			w.AddString(p.Name)
		}
	}
}
