package game

import (
	"encoding/xml"
	"fmt"
	"os"
	"sync"
)

// Channel constants matching the C++ ChatChannel values.
const (
	ChannelGuild      = 0x00
	ChannelParty      = 0x01
	ChannelPrivate    = 0xFFFF
	ChannelLivestream = 0xFFFE
)

// ChannelEvent types (ChannelEvent_t).
const (
	ChannelEventJoin    = 0
	ChannelEventLeave   = 1
	ChannelEventInvite  = 2
	ChannelEventExclude = 3
)

// UsersMap is a map of player ID to player pointer (mirrors C++ UsersMap).
type UsersMap map[uint32]*Player

// ChatChannel represents a chat channel that players can join and talk in.
type ChatChannel struct {
	ID           uint16
	Name         string
	Users        map[uint32]*Player
	mu           sync.RWMutex
	Public       bool
	canJoinEvent int
	onJoinEvent  int
	onLeaveEvent int
	onSpeakEvent int
}

// NewChatChannel creates a new chat channel.
func NewChatChannel(id uint16, name string, public bool) *ChatChannel {
	return &ChatChannel{
		ID:           id,
		Name:         name,
		Users:        make(map[uint32]*Player),
		Public:       public,
		canJoinEvent: -1,
		onJoinEvent:  -1,
		onLeaveEvent: -1,
		onSpeakEvent: -1,
	}
}

// AddUser adds a player to the channel. Returns false if already present.
func (ch *ChatChannel) AddUser(player *Player) bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if _, ok := ch.Users[player.ID]; ok {
		return false
	}
	ch.Users[player.ID] = player
	return true
}

// RemoveUser removes a player from the channel. Returns false if not present.
func (ch *ChatChannel) RemoveUser(player *Player) bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if _, ok := ch.Users[player.ID]; !ok {
		return false
	}
	delete(ch.Users, player.ID)
	return true
}

// HasUser checks if a player is in the channel.
func (ch *ChatChannel) HasUser(player *Player) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	_, ok := ch.Users[player.ID]
	return ok
}

// IsPublic returns whether this is a public channel.
func (ch *ChatChannel) IsPublic() bool {
	return ch.Public
}

// GetOwner returns 0 for basic channels; overridden by PrivateChatChannel.
func (ch *ChatChannel) GetOwner() uint32 {
	return 0
}

// GetInvitedUsers returns nil for non-private channels.
func (ch *ChatChannel) GetInvitedUsers() map[uint32]*Player {
	return nil
}

// GetPrivateChannel returns nil, false for basic channels.
// Overridden via type-check in users of this method; see also:
// the PrivateChatChannel embed pattern.
func (ch *ChatChannel) GetPrivateChannel() (*PrivateChatChannel, bool) {
	return nil, false
}

// Talk sends a message to all users in the channel using the Session interface.
// Returns false if the sender is not in the channel.
func (ch *ChatChannel) Talk(fromPlayer *Player, talkType byte, text string) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if _, ok := ch.Users[fromPlayer.ID]; !ok {
		return false
	}
	for _, user := range ch.Users {
		if user == nil || user.ID == 0 {
			continue
		}
		if user.Session != nil {
			user.Session.SendToChannel(0, fromPlayer.Name, uint16(fromPlayer.Level), talkType, ch.ID, text)
		}
	}
	return true
}

// GetUsersSnapshot returns a copy of the users map for thread-safe iteration.
func (ch *ChatChannel) GetUsersSnapshot() map[uint32]*Player {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	snapshot := make(map[uint32]*Player, len(ch.Users))
	for k, v := range ch.Users {
		snapshot[k] = v
	}
	return snapshot
}

// PrivateChatChannel is a player-owned private channel with an invite list.
type PrivateChatChannel struct {
	ChatChannel
	owner   uint32
	Invites map[uint32]*Player
	invMu   sync.RWMutex
}

// NewPrivateChatChannel creates a new private channel owned by a player.
func NewPrivateChatChannel(id uint16, name string, ownerID uint32) *PrivateChatChannel {
	return &PrivateChatChannel{
		ChatChannel: ChatChannel{
			ID:     id,
			Name:   name,
			Users:  make(map[uint32]*Player),
			Public: false,
		},
		owner:   ownerID,
		Invites: make(map[uint32]*Player),
	}
}

// GetOwner returns the owner's player ID.
func (pcc *PrivateChatChannel) GetOwner() uint32 {
	return pcc.owner
}

// IsInvited checks if a player is invited (owner always is).
func (pcc *PrivateChatChannel) IsInvited(guid uint32) bool {
	if guid == pcc.owner {
		return true
	}
	pcc.invMu.RLock()
	defer pcc.invMu.RUnlock()
	_, ok := pcc.Invites[guid]
	return ok
}

// InvitePlayer invites another player to this private channel.
func (pcc *PrivateChatChannel) InvitePlayer(player *Player, invitePlayer *Player) {
	pcc.invMu.Lock()
	if _, exists := pcc.Invites[invitePlayer.ID]; exists {
		pcc.invMu.Unlock()
		return
	}
	pcc.Invites[invitePlayer.ID] = invitePlayer
	pcc.invMu.Unlock()

	// Notify the invited player.
	if invitePlayer.Session != nil {
		msg := fmt.Sprintf("%s invites you to %s private chat channel.", player.Name, player.GetPossessivePronoun())
		invitePlayer.Session.SendTextMessage(22, msg) // MESSAGE_PARTY_MANAGEMENT
	}

	// Notify the inviter.
	if player.Session != nil {
		msg := fmt.Sprintf("%s has been invited.", invitePlayer.Name)
		player.Session.SendTextMessage(22, msg)
	}

	// Notify all channel users about the invite.
	pcc.mu.RLock()
	for _, user := range pcc.Users {
		if user == nil || user.ID == 0 || user.Session == nil {
			continue
		}
		user.Session.SendChannelEvent(pcc.ID, invitePlayer.Name, ChannelEventInvite)
	}
	pcc.mu.RUnlock()
}

// ExcludePlayer removes a player's invite and removes them from the channel.
func (pcc *PrivateChatChannel) ExcludePlayer(player *Player, excludePlayer *Player) {
	pcc.invMu.Lock()
	delete(pcc.Invites, excludePlayer.ID)
	pcc.invMu.Unlock()

	pcc.mu.Lock()
	delete(pcc.Users, excludePlayer.ID)
	pcc.mu.Unlock()

	// Notify channel users about the exclusion.
	pcc.mu.RLock()
	for _, user := range pcc.Users {
		if user == nil || user.ID == 0 || user.Session == nil {
			continue
		}
		user.Session.SendChannelEvent(pcc.ID, excludePlayer.Name, ChannelEventExclude)
	}
	pcc.mu.RUnlock()
}

// RemoveInvite removes a player from the invite list.
func (pcc *PrivateChatChannel) RemoveInvite(guid uint32) bool {
	pcc.invMu.Lock()
	defer pcc.invMu.Unlock()
	_, ok := pcc.Invites[guid]
	delete(pcc.Invites, guid)
	return ok
}

// CloseChannel notifies all users that the private channel is closing.
func (pcc *PrivateChatChannel) CloseChannel() {
	pcc.mu.RLock()
	for _, user := range pcc.Users {
		if user == nil || user.ID == 0 || user.Session == nil {
			continue
		}
		user.Session.SendClosePrivateChannel(pcc.ID)
	}
	pcc.mu.RUnlock()
}

// GetInvitedUsers returns the invite map.
func (pcc *PrivateChatChannel) GetInvitedUsers() map[uint32]*Player {
	return pcc.Invites
}

// GetPrivateChannel returns itself, true for private channels.
func (pcc *PrivateChatChannel) GetPrivateChannel() (*PrivateChatChannel, bool) {
	return pcc, true
}

// ChatManager manages all chat channels for the world.
type ChatManager struct {
	mu              sync.RWMutex
	normalChannels  map[uint16]*ChatChannel
	privateChannels map[uint16]*PrivateChatChannel
	partyChannels   map[uint32]*ChatChannel // keyed by party leader ID
	guildChannels   map[uint32]*ChatChannel // keyed by guild ID
	dummyPrivate    *PrivateChatChannel
	nextPrivateID   uint16
}

// NewChatManager creates a new ChatManager.
func NewChatManager() *ChatManager {
	return &ChatManager{
		normalChannels:  make(map[uint16]*ChatChannel),
		privateChannels: make(map[uint16]*PrivateChatChannel),
		partyChannels:   make(map[uint32]*ChatChannel),
		guildChannels:   make(map[uint32]*ChatChannel),
		nextPrivateID:   100,
	}
}

// AddNormalChannel registers a normal (XML-defined) channel.
func (cm *ChatManager) AddNormalChannel(ch *ChatChannel) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.normalChannels[ch.ID] = ch
}

// GetChannelById returns a normal channel by numeric ID.
func (cm *ChatManager) GetChannelById(channelID uint16) *ChatChannel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.normalChannels[channelID]
}

// GetGuildChannelById returns a guild channel by guild ID.
func (cm *ChatManager) GetGuildChannelById(guildID uint32) *ChatChannel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.guildChannels[guildID]
}

// getChannel returns the channel the player can access by channel ID, or nil.
func (cm *ChatManager) getChannel(player *Player, channelID uint16) *ChatChannel {
	switch channelID {
	case ChannelGuild:
		guild := player.GetGuild()
		if guild == nil {
			return nil
		}
		cm.mu.RLock()
		ch := cm.guildChannels[guild.ID]
		cm.mu.RUnlock()
		return ch

	case ChannelParty:
		if player.Party == nil {
			return nil
		}
		leader := player.Party.Leader()
		if leader == nil {
			return nil
		}
		cm.mu.RLock()
		ch := cm.partyChannels[leader.ID]
		cm.mu.RUnlock()
		return ch

	default:
		cm.mu.RLock()
		ch, ok := cm.normalChannels[channelID]
		if ok && ch != nil {
			cm.mu.RUnlock()
			return ch
		}
		pcc, ok := cm.privateChannels[channelID]
		cm.mu.RUnlock()
		if ok && pcc != nil && pcc.IsInvited(player.ID) {
			return &pcc.ChatChannel
		}
		return nil
	}
}

// createChannel creates a guild, party, or private channel on demand.
func (cm *ChatManager) createChannel(player *Player, channelID uint16) *ChatChannel {
	if ch := cm.getChannel(player, channelID); ch != nil {
		return nil
	}
	switch channelID {
	case ChannelGuild:
		guild := player.GetGuild()
		if guild == nil {
			return nil
		}
		ch := NewChatChannel(channelID, guild.Name, false)
		cm.mu.Lock()
		cm.guildChannels[guild.ID] = ch
		cm.mu.Unlock()
		return ch

	case ChannelParty:
		if player.Party == nil {
			return nil
		}
		leader := player.Party.Leader()
		if leader == nil {
			return nil
		}
		ch := NewChatChannel(channelID, "Party", false)
		cm.mu.Lock()
		cm.partyChannels[leader.ID] = ch
		cm.mu.Unlock()
		return ch

	case ChannelPrivate:
		if !player.IsPremium() {
			return nil
		}
		cm.mu.Lock()
		id := cm.nextPrivateID
		cm.nextPrivateID++
		pcc := NewPrivateChatChannel(id, fmt.Sprintf("Private %d", id), player.ID)
		cm.privateChannels[id] = pcc
		cm.mu.Unlock()
		return &pcc.ChatChannel

	default:
		return nil
	}
}

// addUserToChannel looks up (or creates) a channel and adds the player.
func (cm *ChatManager) addUserToChannel(player *Player, channelID uint16) *ChatChannel {
	ch := cm.getChannel(player, channelID)
	if ch == nil {
		ch = cm.createChannel(player, channelID)
	}
	if ch == nil {
		return nil
	}
	if ch.AddUser(player) {
		return ch
	}
	return ch // already in channel
}

// removeUserFromChannel removes a player from a channel.
func (cm *ChatManager) removeUserFromChannel(player *Player, channelID uint16) bool {
	ch := cm.getChannel(player, channelID)
	if ch == nil {
		return false
	}
	if !ch.RemoveUser(player) {
		return false
	}
	if ch.GetOwner() == player.ID {
		cm.deleteChannel(player, channelID)
	}
	return true
}

// deleteChannel removes a guild, party, or private channel.
func (cm *ChatManager) deleteChannel(player *Player, channelID uint16) bool {
	switch channelID {
	case ChannelGuild:
		guild := player.GetGuild()
		if guild == nil {
			return false
		}
		cm.mu.Lock()
		delete(cm.guildChannels, guild.ID)
		cm.mu.Unlock()
		return true

	case ChannelParty:
		if player.Party == nil {
			return false
		}
		leader := player.Party.Leader()
		if leader == nil {
			return false
		}
		cm.mu.Lock()
		delete(cm.partyChannels, leader.ID)
		cm.mu.Unlock()
		return true

	default:
		cm.mu.Lock()
		pcc, ok := cm.privateChannels[channelID]
		if !ok {
			cm.mu.Unlock()
			return false
		}
		pcc.CloseChannel()
		delete(cm.privateChannels, channelID)
		cm.mu.Unlock()
		return true
	}
}

// removeUserFromAllChannels removes a player from every channel.
func (cm *ChatManager) removeUserFromAllChannels(player *Player) {
	cm.mu.RLock()
	for _, ch := range cm.normalChannels {
		ch.RemoveUser(player)
	}
	for _, ch := range cm.partyChannels {
		ch.RemoveUser(player)
	}
	for _, ch := range cm.guildChannels {
		ch.RemoveUser(player)
	}
	for _, pcc := range cm.privateChannels {
		pcc.RemoveInvite(player.ID)
		pcc.RemoveUser(player)
		if pcc.GetOwner() == player.ID {
			pcc.CloseChannel()
		}
	}
	cm.mu.RUnlock()

	// Remove owned private channels.
	cm.mu.Lock()
	for id, pcc := range cm.privateChannels {
		if pcc.GetOwner() == player.ID {
			delete(cm.privateChannels, id)
		}
	}
	cm.mu.Unlock()
}

// getChannelList builds the list of channels visible to the player.
func (cm *ChatManager) getChannelList(player *Player) []*ChatChannel {
	var list []*ChatChannel

	if player.GetGuild() != nil {
		ch := cm.getChannel(player, ChannelGuild)
		if ch == nil {
			ch = cm.createChannel(player, ChannelGuild)
		}
		if ch != nil {
			list = append(list, ch)
		}
	}

	if player.Party != nil {
		ch := cm.getChannel(player, ChannelParty)
		if ch == nil {
			ch = cm.createChannel(player, ChannelParty)
		}
		if ch != nil {
			list = append(list, ch)
		}
	}

	cm.mu.RLock()
	for _, ch := range cm.normalChannels {
		list = append(list, ch)
	}
	cm.mu.RUnlock()

	hasPrivate := false
	cm.mu.RLock()
	for _, pcc := range cm.privateChannels {
		if pcc.IsInvited(player.ID) {
			list = append(list, &pcc.ChatChannel)
		}
		if pcc.GetOwner() == player.ID {
			hasPrivate = true
		}
	}
	cm.mu.RUnlock()

	if !hasPrivate && player.IsPremium() {
		cm.mu.Lock()
		if cm.dummyPrivate == nil {
			cm.dummyPrivate = NewPrivateChatChannel(ChannelPrivate, "Private Chat", 0)
		}
		cm.mu.Unlock()
		list = append(list, &cm.dummyPrivate.ChatChannel)
	}

	return list
}

// talkToChannel sends a message to a channel on behalf of a player.
func (cm *ChatManager) talkToChannel(player *Player, talkType byte, text string, channelID uint16) bool {
	ch := cm.getChannel(player, channelID)
	if ch == nil {
		return false
	}

	if channelID == ChannelGuild {
		rank := player.GetGuildRankLevel()
		if rank > 1 {
			talkType = 8 // TALKTYPE_CHANNEL_O
		} else if talkType != 7 {
			talkType = 7 // TALKTYPE_CHANNEL_Y
		}
	} else if talkType != 7 && (channelID == ChannelPrivate || channelID == ChannelParty) {
		talkType = 7
	}

	return ch.Talk(player, talkType, text)
}

// getPrivateChannel returns the private channel owned by the player, or nil.
func (cm *ChatManager) getPrivateChannel(player *Player) *PrivateChatChannel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, pcc := range cm.privateChannels {
		if pcc.GetOwner() == player.ID {
			return pcc
		}
	}
	return nil
}

// -- World integration methods --

// GetChannelList returns the list of channels visible to the player.
func (w *World) GetChannelList(player *Player) []*ChatChannel {
	if w.ChatManager == nil {
		return nil
	}
	return w.ChatManager.getChannelList(player)
}

// GetChannel returns a channel the player can access.
func (w *World) GetChannel(player *Player, channelID uint16) *ChatChannel {
	if w.ChatManager == nil {
		return nil
	}
	return w.ChatManager.getChannel(player, channelID)
}

// AddUserToChannel adds a player to the channel, creating it if needed.
func (w *World) AddUserToChannel(player *Player, channelID uint16) *ChatChannel {
	if w.ChatManager == nil {
		return nil
	}
	return w.ChatManager.addUserToChannel(player, channelID)
}

// RemoveUserFromChannel removes a player from a channel.
func (w *World) RemoveUserFromChannel(player *Player, channelID uint16) bool {
	if w.ChatManager == nil {
		return false
	}
	return w.ChatManager.removeUserFromChannel(player, channelID)
}

// RemoveUserFromAllChannels removes a player from all channels.
func (w *World) RemoveUserFromAllChannels(player *Player) {
	if w.ChatManager == nil {
		return
	}
	w.ChatManager.removeUserFromAllChannels(player)
}

// TalkToChannel sends a message to a channel on behalf of a player.
func (w *World) TalkToChannel(player *Player, channelID uint16, talkType byte, text string) bool {
	if w.ChatManager == nil {
		return false
	}
	return w.ChatManager.talkToChannel(player, talkType, text, channelID)
}

// CreatePrivateChannel creates a new private channel for a player.
func (w *World) CreatePrivateChannel(player *Player) *ChatChannel {
	if w.ChatManager == nil {
		return nil
	}
	return w.ChatManager.createChannel(player, ChannelPrivate)
}

// InviteToPrivateChannel invites a target player to the sender's private channel.
func (w *World) InviteToPrivateChannel(playerID uint32, targetName string) {
	if w.ChatManager == nil {
		return
	}
	w.mu.RLock()
	p := w.players[playerID]
	w.mu.RUnlock()
	if p == nil {
		return
	}
	channel := w.ChatManager.getPrivateChannel(p)
	if channel == nil {
		return
	}
	target := w.PlayerByName(targetName)
	if target == nil || target.ID == playerID {
		return
	}
	channel.InvitePlayer(p, target)
}

// ExcludeFromPrivateChannel excludes a target from the sender's private channel.
func (w *World) ExcludeFromPrivateChannel(playerID uint32, targetName string) {
	if w.ChatManager == nil {
		return
	}
	w.mu.RLock()
	p := w.players[playerID]
	w.mu.RUnlock()
	if p == nil {
		return
	}
	channel := w.ChatManager.getPrivateChannel(p)
	if channel == nil {
		return
	}
	target := w.PlayerByName(targetName)
	if target == nil || target.ID == playerID {
		return
	}
	channel.ExcludePlayer(p, target)
}

// RemoveUserFromAllChannelsByID removes a player (looked up by ID) from all channels.
// Called from World.RemovePlayer before the player is unregistered, so we look up
// the player manually without holding the World lock (which RemovePlayer already holds).
func (w *World) RemoveUserFromAllChannelsByID(id uint32) {
	w.mu.RLock()
	p := w.players[id]
	w.mu.RUnlock()
	if p == nil || w.ChatManager == nil {
		return
	}
	w.ChatManager.removeUserFromAllChannels(p)
}

// SendPrivateMessage sends a private message from sender to receiver.
func (w *World) SendPrivateMessage(sender, receiver *Player, talkType byte, text string) {
	if receiver == nil || receiver.Session == nil {
		return
	}
	receiver.Session.SendToChannel(0, sender.Name, uint16(sender.Level), talkType, 0, text)
}

// ---------------------------------------------------------------------------
// XML channel loading.
// ---------------------------------------------------------------------------

// chatChannelXML is the XML structure for chatchannels.xml.
type chatChannelXML struct {
	XMLName  xml.Name           `xml:"channels"`
	Channels []chatChannelEntry `xml:"channel"`
}

type chatChannelEntry struct {
	ID     uint16 `xml:"id,attr"`
	Name   string `xml:"name,attr"`
	Public bool   `xml:"public,attr"`
	Script string `xml:"script,attr"`
}

// LoadChatChannels loads channel definitions from an XML file.
func (w *World) LoadChatChannels(path string) error {
	if w.ChatManager == nil {
		w.ChatManager = NewChatManager()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read chatchannels.xml: %w", err)
	}

	var xmlChannels chatChannelXML
	if err := xml.Unmarshal(data, &xmlChannels); err != nil {
		return fmt.Errorf("parse chatchannels.xml: %w", err)
	}

	for _, entry := range xmlChannels.Channels {
		ch := NewChatChannel(entry.ID, entry.Name, entry.Public)
		ch.canJoinEvent = -1
		ch.onJoinEvent = -1
		ch.onLeaveEvent = -1
		ch.onSpeakEvent = -1
		w.ChatManager.AddNormalChannel(ch)
	}

	return nil
}
