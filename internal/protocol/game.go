package protocol

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/network"
	"github.com/opentibiabr/canary-go/internal/tibcrypto"
	"github.com/opentibiabr/canary-go/internal/transport"
)

// Outbound game opcodes.
const (
	opChallenge      = 0x1F
	opError          = 0x14
	opSelfAppear     = 0x17
	opAllowBugReport = 0x1A
	opTibiaTime      = 0xEF
	opPending        = 0x0A
	opEnterWorld     = 0x0F
	opFullMap        = 0x64
	opMapNorth       = 0x65
	opMapEast        = 0x66
	opMapSouth       = 0x67
	opMapWest        = 0x68
	opUpdateTile     = 0x69
	opTileTransform  = 0x6B
	opCreatureMove   = 0x6D
	opInventoryItem  = 0x78
	opInventoryEmpty = 0x79
	opMagicEffect    = 0x83
	opWorldLight     = 0x82
	opCreatureHealth = 0x8C
	opCreatureLight  = 0x8D
	opBasicData      = 0x9F
	opPlayerStats    = 0xA0
	opPlayerSkills   = 0xA1
	opTextMessage    = 0xB4
	opCreatureSay    = 0xAA
	opPing           = 0x1D // server→client keep-alive ping (client replies 0x1E)
	opPingBack       = 0x1E // server→client reply to a client ping
	opCancelTarget   = 0xA3
	creatureTurnMark = 0x0063
)

// Inbound game opcodes.
const (
	inLogout         = 0x14
	inPing           = 0x1D // client keep-alive ping → reply with opPingBack
	inPong           = 0x1E // client reply to our opPing → refresh liveness
	inAutoWalk       = 0x64
	inStopAutoWalk   = 0x69
	inWalkNorth      = 0x65
	inWalkEast       = 0x66
	inWalkSouth      = 0x67
	inWalkWest       = 0x68
	inWalkNE         = 0x6A
	inWalkSE         = 0x6B
	inWalkSW         = 0x6C
	inWalkNW         = 0x6D
	inTurnNorth      = 0x6F
	inTurnEast       = 0x70
	inTurnSouth      = 0x71
	inTurnWest       = 0x72
	inSay            = 0x96
	inExtendedOpcode = 0x32
	inUseItem          = 0x82
	inUseItemWith      = 0x83
	inUseWithCreature  = 0x84
	inCloseContainer = 0x87
	inContainerUp    = 0x88
	inLookAt         = 0x8C
	inThrowItem      = 0x78
	inAttack         = 0xA1
	inFightModes     = 0xA0
	inBuyItem        = 0x7A
	inSellItem       = 0x7B
	inCloseShop      = 0x7C
	// 0x97 is playerRequestChannels in C++ (protocolgame.cpp:1842), NOT close
	// channel. Closing a channel is 0x99.
	inRequestChannels = 0x97
	inCloseChannel    = 0x99
	inNpcGreet        = 0xEE
	inHighscore       = 0xB1
	// Inbound party opcodes (0xA3..0xA8). NOTE: 0xA3 collides with the OUTBOUND
	// opCancelTarget const — these are a separate inbound namespace.
	inInviteToParty        = 0xA3
	inJoinParty            = 0xA4
	inRevokePartyInvite    = 0xA5
	inPassPartyLeadership  = 0xA6
	inLeaveParty           = 0xA7
	inEnableSharedPartyExp = 0xA8
	// Inbound market opcodes (0xF3..0xF8).
	inMarketLeave    = 0xF3
	inMarketBrowse   = 0xF5
	inMarketCreate   = 0xF6
	inMarketCancel   = 0xF7
	inMarketAccept   = 0xF8

	// Batch 1: additional inbound opcodes.
	inRetrieveDepotSearch      = 0x29
	inCyclopediaMonsterTracker = 0x2A
	inPartyAnalyzerAction      = 0x2B
	inLeaderFinderWindow       = 0x2C
	inMemberFinderWindow       = 0x2D
	inSetClientOptions         = 0x2E
	inPlayerTyping             = 0x38
	inInventoryImbuements      = 0x60
	inClientCheck              = 0x63
	inSetVocation              = 0x6E
	inTeleport                 = 0x73
	inStartOfflineTraining     = 0x74
	inContainerAction          = 0x75
	inHotkeyEquip              = 0x77
	inLookInShop               = 0x79
	inRequestTrade             = 0x7D
	inLookInTrade              = 0x7E
	inAcceptTrade              = 0x7F
	inCloseTrade               = 0x80
	inFriendSystemAction       = 0x81
	inRotateItem               = 0x85
	inConfigureShowOffSocket   = 0x86
	inTextWindow               = 0x89
	inHouseWindow              = 0x8A
	inWrapableItem             = 0x8B
	inLookInBattleList         = 0x8D
	inJoinAggression           = 0x8E
	inOpenDepotSearch          = 0x92
	inCloseDepotSearch         = 0x93
	inDepotSearchItemRequest   = 0x94
	inOpenParentContainer      = 0x95
	inEditGuildMessage         = 0x9C
	inGetTextForReport         = 0x9D
	inCloseNpcChannel          = 0x9E
	inSetMonsterPodium         = 0x9F
	inFollow                   = 0xA2

	// Batch 2: opcodes whose handlers already existed but were never dispatched.
	// Values taken from the ProtocolGame::parsePacketFromDispatcher switch
	// (src/server/network/protocol/protocolgame.cpp:1640-2068).
	inCharacterTradeConfig    = 0x76
	inOpenChannel             = 0x98
	inOpenPrivateChannel      = 0x9A
	inCreatePrivateChannel    = 0xAA
	inChannelInvite           = 0xAB
	inChannelExclude          = 0xAC
	inClientDetails           = 0xC1
	inBossDifficultySelection = 0xC2
	inAimAtTarget             = 0xC8
	inGetTransactionDetails   = 0xC9
	inExivaRestrictions       = 0xCA
	inCyclopediaMapAction     = 0xDB
	inVIPEdit                 = 0xDE
	inVipGroupActions         = 0xDF
	inBugReport               = 0xE6
	inSendResourceBalance     = 0xED
	inQuestLog                = 0xF0
	inQuestLine               = 0xF1
	inModalWindowAnswer       = 0xF9
	inRewardChestCollect      = 0xFF
)

// worldTypeNoPvp mirrors WORLD_TYPE_NO_PVP (src/game/game.hpp).
const worldTypeNoPvp = 1

// GameProtocol is one game-server session.
type GameProtocol struct {
	deps *Deps
	conn *network.Connection
	// profile is the protocol profile this session uses. It is set by the factory
	// based on the listening port and determines wire-level behaviour differences
	// (challenge format, login layout, feature flags).
	profile *Profile

	challengeTS   uint32
	challengeRand uint8
	loggedIn      bool

	player  *game.Player
	knownMu sync.RWMutex
	known   map[uint32]bool

	statementID uint32

	pingStop chan struct{} // closed once to stop the keep-alive ping loop
	pingOnce sync.Once

	actionMu sync.Mutex    // serializes player movement (walk/turn/auto-walk step)
	walkGen  atomic.Uint64 // bumping cancels the in-flight auto-walk path
}

func (g *GameProtocol) isKnown(id uint32) bool {
	g.knownMu.RLock()
	defer g.knownMu.RUnlock()
	return g.known[id]
}

func (g *GameProtocol) setKnown(id uint32, known bool) {
	g.knownMu.Lock()
	defer g.knownMu.Unlock()
	if g.known == nil {
		g.known = make(map[uint32]bool)
	}
	if known {
		g.known[id] = true
	} else {
		delete(g.known, id)
	}
}

// openContainerByCID returns the container open under a client cid, preserving
// the (item, ok) shape callers expect. The open-container state is the single
// source of truth on game.Player (see Player.openContainers).
func (g *GameProtocol) openContainerByCID(cid uint8) (*game.Item, int, bool) {
	if g.player == nil {
		return nil, 0, false
	}
	c := g.player.GetContainerByID(cid)
	if c == nil {
		return nil, 0, false
	}
	offset := int(g.player.GetContainerIndex(cid))
	return c, offset, true
}

// rangeContainers returns a snapshot of the open containers as cid->item for
// iteration.
func (g *GameProtocol) rangeContainers() map[uint8]*game.Item {
	out := map[uint8]*game.Item{}
	if g.player == nil {
		return out
	}
	for cid, oc := range g.player.OpenContainersSnapshot() {
		if oc.Container != nil {
			out[cid] = oc.Container
		}
	}
	return out
}

// NewGameFactory returns a factory building GameProtocol instances for the
// given protocol profile. Pass nil for the default current (1525) profile.
func NewGameFactory(deps *Deps, profile *Profile) network.ProtocolFactory {
	return func() network.Protocol {
		p := profile
		if p == nil {
			p = currentProfile
		}
		return &GameProtocol{
			deps:     deps,
			profile:  p,
			known:   make(map[uint32]bool),
			pingStop: make(chan struct{}),
		}
	}
}

// NewLegacy1100Factory returns a factory for the legacy 11.00 game protocol.
func NewLegacy1100Factory(deps *Deps) network.ProtocolFactory {
	return NewGameFactory(deps, tibia1100Profile)
}

// NewLegacy860Factory returns a factory for the legacy 8.60 game protocol.
func NewLegacy860Factory(deps *Deps) network.ProtocolFactory {
	return NewGameFactory(deps, cipsoft860Profile)
}

// pingInterval mirrors Player::sendPing: the reference server pings the client
// every 5s; the modern client drops the connection if these stop arriving.
const pingInterval = 5 * time.Second

// startPingLoop sends a keep-alive ping (0x1D) every pingInterval until the
// connection closes.
func (g *GameProtocol) startPingLoop() {
	go func() {
		t := time.NewTicker(pingInterval)
		defer t.Stop()
		for {
			select {
			case <-g.pingStop:
				return
			case <-t.C:
				w := netmsg.NewWriter()
				w.AddByte(opPing)
				g.SendToClient(w)
			}
		}
	}()
}

// stopPingLoop halts the keep-alive ping loop (idempotent).
func (g *GameProtocol) stopPingLoop() {
	g.pingOnce.Do(func() { close(g.pingStop) })
}

// Player implements game.Session.
// isOldProtocol returns true for legacy protocol profiles (11.00 or 8.60).
func (g *GameProtocol) isOldProtocol() bool {
	return g.profile != nil && g.profile.Version != VersionCurrent
}

// isCipsoft860 returns true for the 8.60 protocol profile.
func (g *GameProtocol) isCipsoft860() bool {
	return g.profile != nil && g.profile.Version == VersionCipsoft860
}

func (g *GameProtocol) Player() *game.Player { return g.player }

// SendToClient implements game.Session.
func (g *GameProtocol) SendToClient(w *netmsg.Writer) {
	if g.conn != nil {
		b := w.Bytes()
		if len(b) == 0 {
			g.deps.Log.Warn("empty packet discarded")
			return
		}
		_ = g.conn.Send(w)
	}
}

// Disconnect implements game.Session.
func (g *GameProtocol) Disconnect() {
	if g.conn != nil {
		g.conn.Close()
	}
}

// OnConnect sends the login challenge immediately.
func (g *GameProtocol) OnConnect(c *network.Connection) {
	g.conn = c

	// Apply transport profile based on protocol version.
	switch g.profile.Version {
	case VersionTibia1100:
		c.Codec().ApplyProfile(transport.ProfileLegacyLogin)
	case VersionCipsoft860:
		c.Codec().ApplyProfile(transport.ProfileLegacyClassic)
	default:
		c.Codec().ApplyProfile(transport.ProfileCurrentModern)
	}

	// Deterministic-ish challenge derived from the clock; values are echoed back.
	now := uint32(time.Now().Unix())
	g.challengeTS = now
	g.challengeRand = uint8(now & 0xFF)

	if g.profile.HasChallengeResponse {
		g.sendModernChallenge()
	} else {
		g.sendLegacyChallenge()
	}
}

// sendModernChallenge sends the 1525 challenge:
// [adler32(payload)][0x01][0x1F][ts u32][rand u8][0x71]
func (g *GameProtocol) sendModernChallenge() {
	payload := netmsg.NewWriter()
	payload.AddByte(0x01)
	payload.AddByte(opChallenge)
	payload.AddU32(g.challengeTS)
	payload.AddByte(g.challengeRand)
	payload.AddByte(0x71)
	pb := payload.Bytes()

	w := netmsg.NewWriter()
	w.AddU32(tibcrypto.Adler32(pb))
	w.AddBytes(pb)
	_ = g.conn.Send(w)
}

// sendLegacyChallenge sends the 860/1100 challenge:
// [adler32(1+6)][0x1F][ts u32][rand u8] — no 0x01 marker, no 0x71 terminator.
func (g *GameProtocol) sendLegacyChallenge() {
	payload := netmsg.NewWriter()
	payload.AddByte(opChallenge)
	payload.AddU32(g.challengeTS)
	payload.AddByte(g.challengeRand)
	pb := payload.Bytes()

	w := netmsg.NewWriter()
	w.AddU32(tibcrypto.Adler32(pb))
	w.AddBytes(pb)
	_ = g.conn.Send(w)
}

// OnDisconnect saves and removes the player.
func (g *GameProtocol) OnDisconnect(c *network.Connection) {
	g.stopPingLoop()
	if g.player == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Update last logout timestamp and execute creature event logout callbacks
	g.player.LastLogout = uint64(time.Now().Unix())
	g.deps.Lua.ExecuteCreatureOnLogout(g.player)

	if err := g.deps.DB.SavePlayer(ctx, g.player); err != nil {
		c.Logger().Warn("save on disconnect failed", "err", err)
	}
	g.deps.World.RemovePlayer(g.player.ID)
		if err := g.deps.DB.RemovePlayerOnline(ctx, g.player.DBID); err != nil {
			c.Logger().Debug("remove player_online", "err", err)
		}
	g.broadcastRemove(g.player)
	c.Logger().Info("player logged out", "name", g.player.Name, "online", g.deps.World.OnlineCount())
	g.player = nil
}

// firstHeaderBytes returns the number of leading bytes to strip from the first
// game packet before the client metadata. Differs per protocol profile.
func (g *GameProtocol) firstHeaderBytes() int {
	switch g.profile.Version {
	case VersionCipsoft860:
		return 4 // CHECKSUM_LENGTH (no extra marker byte)
	case VersionTibia1100:
		return 5 // CHECKSUM_LENGTH + 1
	default:
		return 6 // CHECKSUM_LENGTH + 2 (modern)
	}
}

// OnFirstPacket parses the game login request. The wire layout varies by
// protocol profile (1525 current, 11.00 legacy, 8.60 legacy).
func (g *GameProtocol) OnFirstPacket(c *network.Connection, body []byte) {
	g.conn = c
	skip := g.firstHeaderBytes()
	if len(body) < skip {
		c.Logger().Debug("game: short first packet", "len", len(body))
		return
	}
	r := netmsg.NewReader(body[skip:])

	_ = r.GetU16() // OS
	_ = r.GetU16() // protocol version

	var clientVersion uint32
	var contentRevision uint16

	switch g.profile.Version {
	case VersionCipsoft860:
		// 8.60: asset signatures before RSA
		_ = r.GetU32() // dat signature
		_ = r.GetU32() // spr signature
		_ = r.GetU32() // pic signature
	case VersionTibia1100:
		// 11.00: client version u32 + content revision u16
		clientVersion = r.GetU32()
		contentRevision = r.GetU16()
		_ = contentRevision
	default:
		// Current (1525): full metadata set
		clientVersion = r.GetU32()
		_ = r.GetString() // client version string
		_ = r.GetString() // asset hash
		_ = r.GetByte()   // preview state
	}

	if r.Remaining() < tibcrypto.BlockSize {
		c.Logger().Debug("game: short packet, no RSA block")
		return
	}
	block := r.GetBytes(tibcrypto.BlockSize)
	if err := g.deps.RSA.Decrypt(block); err != nil || block[0] != 0 {
		c.Logger().Debug("game: rsa decrypt failed")
		return
	}

	var key tibcrypto.XTEAKey
	var account, password, charName string

	if g.profile.PasswordLogin {
		// 8.60 password-based login: [XTEA key][account_num][char_name][password]
		br := netmsg.NewReader(block[1:])
		key = tibcrypto.KeyFromBytes(br.GetBytes(16))
		accountNum := br.GetU32()
		charName = br.GetString()
		password = br.GetString()
		account = fmt.Sprintf("%d", accountNum)
	} else {
		// Session-key auth (1525 & 1100): [XTEA key][gm][session_key][char_name][echo_ts][echo_rand]
		br := netmsg.NewReader(block[1:])
		key = tibcrypto.KeyFromBytes(br.GetBytes(16))
		_ = br.GetByte() // is gamemaster
		sessionKey := br.GetString()
		charName = br.GetString()

		if g.profile.HasChallengeResponse {
			echoTS := br.GetU32()
			echoRand := br.GetByte()
			if echoTS != g.challengeTS || echoRand != g.challengeRand {
				c.Logger().Debug("game: challenge mismatch")
			}
		}

		account, password = splitSessionKey(sessionKey)
	}

	// Enable encryption with the correct profile.
	if g.profile.Transport == transport.ProfileCurrentModern {
		g.conn.Codec().EnableModernGame(key)
	} else {
		g.conn.Codec().EnableLegacyGame(key)
	}

	if clientVersion != 0 && clientVersion != uint32(g.profile.Version) {
		msg := fmt.Sprintf("Wrong client version. Please use a %s client.", g.profile.Label)
		g.disconnect(msg)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	acc, err := g.deps.DB.LoadAccount(ctx, account)
	if err != nil || !db.VerifyPassword(password, acc.Password) {
		g.disconnect("Account name or password is not correct.")
		return
	}
	player, err := g.deps.DB.LoadPlayer(ctx, charName)
	if err != nil || player.AccountID != acc.ID {
		g.disconnect("Character not found on this account.")
		return
	}

	// Resolve the player's temple (respawn point) from the OTBM towns. The SQL
	// `towns` table only holds placeholder data, so trusting it sends dead
	// players to a void tile ("limbo"); the OTBM town data has the real temple
	// positions. Fall back to the default spawn when the town id is unknown or
	// its tile is not walkable.
	temple := g.deps.World.DefaultSpawn
	if t, ok := g.deps.World.TempleByTownID(player.TownID); ok && g.deps.World.Map.GetTile(t).Walkable(g.deps.Items) {
		temple = t
	}
	player.LoginPosition = temple

	// Relocate to the temple if the stored tile has no ground (e.g. a fresh
	// character, or a stored position outside the loaded map).
	if !g.deps.World.Map.GetTile(player.Pos).Walkable(g.deps.Items) {
		player.Pos = temple
	}

	if !g.deps.World.AddPlayer(player, g) {
		g.disconnect("You are already logged in.")
		return
	}
	if err := g.deps.DB.AddPlayerOnline(ctx, player.DBID); err != nil {
		c.Logger().Debug("add player_online", "err", err)
	}
	g.player = player
	g.enterWorld()

	// Run creature event login callbacks (offline training, stamina, first items, daily rewards, etc.)
	g.deps.Lua.ExecuteCreatureOnLogin(player)
	player.LastLogin = uint64(time.Now().Unix())

	g.deps.Lua.Call("onPlayerLogin", player.Name)
	c.Logger().Info("player entered world", "name", player.Name,
		"pos", player.Pos, "online", g.deps.World.OnlineCount())
}

func splitSessionKey(sk string) (account, password string) {
	if i := strings.IndexByte(sk, '\n'); i >= 0 {
		return sk[:i], sk[i+1:]
	}
	return sk, ""
}

func (g *GameProtocol) disconnect(msg string) {
	w := netmsg.NewWriter()
	w.AddByte(opError)
	w.AddString(msg)
	w.AddByte(0x00) // reason flag (OTClient parseLoginError expects this for v>=1523)
	_ = g.conn.Send(w)
	g.conn.Close()
}

func (g *GameProtocol) sendCoinBalance() {
	if g.player == nil {
		return
	}

	// 0xF2: Coin Balance Updating (show spinner)
	w1 := netmsg.NewWriter()
	w1.AddByte(0xF2)
	w1.AddByte(0x01)
	g.SendToClient(w1)

	// 0xDF: Coin Balance (hide spinner and update UI)
	w2 := netmsg.NewWriter()
	w2.AddByte(0xDF)
	w2.AddByte(0x01)

	// Total must be >= Transferable, otherwise the client disables the Sell UI.
	total := g.player.CoinBalance + g.player.CoinTransferable
	w2.AddU32(total)
	w2.AddU32(g.player.CoinTransferable)
	w2.AddU32(total) // reserved auction coins

	g.SendToClient(w2)
}

func (g *GameProtocol) sendResourceBalances() {
	if g.player == nil {
		return
	}
	// RESOURCE_BANK (0x00) - send as u64
	w := netmsg.NewWriter()
	w.AddByte(0xEE)
	w.AddByte(0x00)
	w.AddU64(g.player.BankBalance)
	g.SendToClient(w)
}


// dispatchStore forwards a store packet's payload (bytes after the opcode) to
// the gamestore Lua module.
func (g *GameProtocol) dispatchDailyReward(op byte, r *netmsg.Reader) {
	if g.player == nil || g.deps == nil || g.deps.Lua == nil {
		return
	}
	data := r.GetBytes(r.Remaining())
	g.deps.Lua.DispatchDailyRewardPacket(g.player, op, data)
}

func (g *GameProtocol) dispatchStore(op byte, r *netmsg.Reader) {
	if g.player == nil || g.deps == nil || g.deps.Lua == nil {
		return
	}
	data := r.GetBytes(r.Remaining())
	g.deps.Log.Info("store opcode <- client", "op", fmt.Sprintf("0x%02X", op),
		"payloadLen", len(data), "hex", fmt.Sprintf("%x", data))
	g.deps.Lua.DispatchStorePacket(g.player, op, data)
}

// enterWorld sends the full login sequence as a single message.
func (g *GameProtocol) enterWorld() {
	p := g.player
	
	w := netmsg.NewWriter()

	// 0x17 self appear.
	w.AddByte(opSelfAppear)
	w.AddU32(p.ID)
	w.AddU16(ServerBeat)
	w.AddDouble(float64(p.Speed), 3) // speedA
	w.AddDouble(0, 3)                // speedB
	w.AddDouble(0, 3)                // speedC
	w.AddByte(0)                     // can change pvp framing
	w.AddByte(0)                     // expert mode
	w.AddString("")                  // store images url
	w.AddU16(25)                     // store coin package size
	w.AddByte(0)                     // exiva button

	// 0x1A allow bug report.
	w.AddByte(opAllowBugReport)
	w.AddByte(0)

	// 0xEF tibia time.
	now := time.Now()
	w.AddByte(opTibiaTime)
	w.AddByte(uint8(now.Hour()))
	w.AddByte(uint8(now.Minute()))

	// 0x0A pending state, 0x0F enter world.
	w.AddByte(opPending)
	w.AddByte(opEnterWorld)

	// 0x64 full map description.
	w.AddByte(opFullMap)
	w.AddPosition(netmsg.Position{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z})
	g.addMapDescription(w, int(p.Pos.X)-viewportX, int(p.Pos.Y)-viewportY, p.Pos.Z, mapWidth, mapHeight)

	// 0x83 magic effect (login teleport), modern layout: create-effect (3),
	// u16 type, source byte, end-loop (0).
	w.AddByte(opMagicEffect)
	w.AddPosition(netmsg.Position{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z})
	w.AddByte(magicEffectsCreate)
	w.AddU16(constMETeleport)
	w.AddByte(sourceEffectOwn)
	w.AddByte(magicEffectsEndLoop)

	// Inventory: send items or empty.
	for slot := byte(1); slot <= 10; slot++ {
		if item := p.Inventory[slot]; item != nil {
			w.AddByte(opInventoryItem)
			w.AddByte(slot)
			g.addItem(w, item)
		} else {
			w.AddByte(opInventoryEmpty)
			w.AddByte(slot)
		}
	}
	// Slot 11 (CONST_SLOT_STORE_INBOX): always present so the client shows the
	// store inbox and can open it to retrieve in-game store purchases.
	if p.StoreInbox == nil {
		p.StoreInbox = &game.Item{ID: 23396} // ITEM_STORE_INBOX
	}
	w.AddByte(opInventoryItem)
	w.AddByte(11)
	g.addItem(w, p.StoreInbox)

	g.addStats(w)
	g.addSkills(w)

	// 0x82 world light (level + color).
	w.AddByte(opWorldLight)
	w.AddByte(0xFF) // full daylight level
	w.AddByte(0x00) // light color (default)

	// 0x8D creature light.
	w.AddByte(opCreatureLight)
	w.AddU32(p.ID)
	w.AddByte(0) // light level
	w.AddByte(0) // light color

	g.addBasicData(w)
	g.SendToClient(w)
	// Cyclopedia houses info (enables the house auction UI in the client).
	g.sendHousesInfo()
	// Enable stash/market special containers menu so "Stow" appears.
	g.sendSpecialContainersAvailable()

	// Force close all containers on the client to clear any ghost containers.
	// The client caches open containers locally, but if the server doesn't have them
	// saved (e.g. lost state), the client shows an empty "Container" ghost window.
	for i := uint8(0); i < 16; i++ {
		wClose := netmsg.NewWriter()
		wClose.AddByte(0x6F) // opContainerClose
		wClose.AddByte(i)
		g.SendToClient(wClose)
	}

	// Restore any containers that were left open by the client in its local config.
	g.restoreOpenContainers()

	// Send initial condition/protection zone icons
	g.SendIcons()

	// Store / Tibia Coins balance (shown on the store button).
	g.sendCoinBalance()
	g.sendResourceBalances()

	// Send Prey data & prices
	g.SendPreyPrices()
	g.SendAllPreyData()

	// Boss Cyclopedia fight cooldowns (empty when the player has none).
	g.SendBosstiaryCooldownTimer()

	// Keep-alive pings start once the player is in the world.
	g.startPingLoop()

	// Notify other spectators that we appeared.
	g.broadcastAppear(p)
}

func (g *GameProtocol) sendStats() {
	w := netmsg.NewWriter()
	g.addStats(w)
	g.conn.Send(w)
}

func (g *GameProtocol) addStats(w *netmsg.Writer) {
	p := g.player
	w.AddByte(opPlayerStats)
	w.AddU32(p.Health)
	w.AddU32(p.GetMaxHealth())

	totalCap := p.GetCapacity()
	usedCap := g.getPlayerTotalWeight()
	freeCap := uint32(0)
	if totalCap > usedCap {
		freeCap = totalCap - usedCap
	}
	w.AddU32(freeCap)
	
	w.AddU64(p.Experience)
	w.AddU16(p.Level)
	w.AddU16(p.GetLevelPercent()) // level percent (0-10000)
	w.AddU16(100) // base xp gain
	w.AddU16(0)   // grinding xp boost
	w.AddU16(0)   // xp boost percent
	w.AddU16(0)   // stamina multiplier
	w.AddU32(p.Mana)
	w.AddU32(p.GetMaxMana())
	w.AddByte(p.Soul)
	w.AddU16(2520) // stamina minutes
	w.AddU16(p.Speed)
	// Food/regeneration time in seconds (RegenTicks is milliseconds). Shown as
	// the "food" timer in the client's character status.
	regenSecs := uint32(0)
	if p.RegenTicks > 0 {
		regenSecs = uint32(p.RegenTicks) / 1000
	}
	if regenSecs > 0xFFFF {
		regenSecs = 0xFFFF
	}
	w.AddU16(uint16(regenSecs)) // regeneration seconds

	offlineTrainingMinutes := uint16(0)
	if p.OfflineTrainingTime > 0 {
		offlineTrainingMinutes = uint16(p.OfflineTrainingTime / 60000)
	}
	w.AddU16(offlineTrainingMinutes) // offline training minutes
	w.AddU16(0)                      // xp boost time
	w.AddByte(1) // can buy xp boost
	w.AddU32(0)  // mana shield
	w.AddU32(0)  // max mana shield
}

func (g *GameProtocol) addSkills(w *netmsg.Writer) {
	p := g.player
	w.AddByte(opPlayerSkills)
	// magic level.
	w.AddU16(p.MagLevel) // magic level
	w.AddU16(p.MagLevel) // base magic level
	w.AddU16(0)          // loyalty magic level
	w.AddU16(p.GetMagLevelPercent()) // magic level percent * 100
	// combat skills fist..fishing.
	for i := game.SkillFist; i < game.SkillCount; i++ {
		w.AddU16(p.Skills[i]) // level
		w.AddU16(p.Skills[i]) // base
		w.AddU16(0)           // loyalty
		w.AddU16(p.GetSkillPercent(i)) // percent * 100
	}

	// The rest mirrors AddPlayerSkills for the modern (1525) profile. Our
	// simplified server has no equipped weapon, combat modifiers or imbuements,
	// so these are the zero/default forms — but every field must be present or
	// the client over-reads the 0xA1 packet and desyncs the whole stream.
	w.AddByte(0) // 13.10 skill list (U8 count = 0)

	w.AddU32(p.Capacity) // total capacity (imbuement/feather)
	w.AddU32(p.Capacity) // base capacity
	w.AddU16(0)          // flat damage and healing

	// Weapon attack block — unarmed (fists), no elemental conversion.
	w.AddU16(0)       // attack total
	w.AddByte(0)      // CIPBIA_ELEMENTAL_PHYSICAL
	w.AddDouble(0, 4) // converted damage fraction
	w.AddByte(0)      // element

	// Imbuement leech/crit stats.
	w.AddDouble(0, 4) // life leech
	w.AddDouble(0, 4) // mana leech
	w.AddDouble(0, 4) // crit chance
	w.AddDouble(0, 4) // crit extra damage
	w.AddDouble(0, 4) // onslaught

	w.AddU16(0)       // defense
	w.AddU16(0)       // armor
	w.AddU16(0)       // party mantra
	w.AddDouble(0, 4) // mitigation
	w.AddDouble(0, 4) // dodge (ruse)
	w.AddU16(0)       // damage reflection

	w.AddByte(0) // combat absorb entries count (0 → no entries follow)

	// Forge bonuses.
	w.AddDouble(0, 4) // momentum
	w.AddDouble(0, 4) // transcendence
	w.AddDouble(0, 4) // amplification
}

func getVocationClientID(vocation uint16) byte {
	switch vocation {
	case 1, 5: // Sorcerer / Master Sorcerer
		return 13
	case 2, 6: // Druid / Elder Druid
		return 14
	case 3, 7: // Paladin / Royal Paladin
		return 12
	case 4, 8: // Knight / Elite Knight
		return 11
	case 9, 10: // Monk / Exalted Monk
		return 15
	default:
		return 0
	}
}

func (g *GameProtocol) addBasicData(w *netmsg.Writer) {
	p := g.player
	w.AddByte(opBasicData)
	w.AddByte(1) // is premium
	w.AddU32(uint32(time.Now().Unix() + 86400*365)) // premium expire timestamp
	w.AddByte(getVocationClientID(p.Vocation))
	w.AddByte(1) // has reached main (1 = allow main features like Wheel & Prey)
	w.AddU16(0)  // spell count
	w.AddByte(0) // magic shield active
}

// OnPacket dispatches steady-state game packets.
func (g *GameProtocol) OnPacket(c *network.Connection, r *netmsg.Reader) {
	var op byte
	// A panic while handling one packet (a missing Lua binding, a nil deref in a
	// teleport/map render, a malformed payload) must never take down the whole
	// server. Recover, log it with the offending opcode, and keep the session
	// alive so the player can carry on.
	defer func() {
		if rec := recover(); rec != nil && g.deps != nil && g.deps.Log != nil {
			g.deps.Log.Error("recovered panic while handling packet",
				"opcode", fmt.Sprintf("0x%02X", op), "panic", rec, "stack", string(debug.Stack()))
		}
	}()
	if g.player == nil {
		return
	}
	op = r.GetByte()
	switch op {
	case inLogout:
		c.Close()
	case inPing:
		// Client keep-alive ping — answer with a ping-back (0x1E).
		w := netmsg.NewWriter()
		w.AddByte(opPingBack)
		g.SendToClient(w)
	case 0x28: // Stash action (stow/withdraw)
		g.parseStashAction(r)
	case inPong, 0xBE:
		// Reply to our own keep-alive ping, or safely ignored opcodes (cancel attack).
	case inInventoryImbuements:
		g.parseInventoryImbuements(r)
	case inWalkNorth:
		g.manualWalk(game.DirNorth)
	case inWalkEast:
		g.manualWalk(game.DirEast)
	case inWalkSouth:
		g.manualWalk(game.DirSouth)
	case inWalkWest:
		g.manualWalk(game.DirWest)
	case inWalkNE:
		g.manualWalk(game.DirNE)
	case inWalkSE:
		g.manualWalk(game.DirSE)
	case inWalkSW:
		g.manualWalk(game.DirSW)
	case inWalkNW:
		g.manualWalk(game.DirNW)
	case inStopAutoWalk:
		g.stopAutoWalk()
	case inTurnNorth:
		g.manualTurn(game.DirNorth)
	case inTurnEast:
		g.manualTurn(game.DirEast)
	case inTurnSouth:
		g.manualTurn(game.DirSouth)
	case inTurnWest:
		g.manualTurn(game.DirWest)
	case inAutoWalk:
		g.autoWalk(r)
	case inUseItem:
		g.parseUseItem(r)
	case inUseItemWith:
		g.parseUseItemWith(r)
	case inUseWithCreature:
		g.parseUseWithCreature(r)
	case inCloseContainer:
		g.parseCloseContainer(r)
	case inContainerUp:
		g.parseContainerUp(r)
	case 0xCB: // Browse field
		g.parseBrowseField(r)
	case 0xCC: // Seek in paginated container
		g.parseSeekContainer(r)
	case inSay:
		g.handleSay(r)
	case inThrowItem:
		g.parseItemMove(r)
	case inLookAt:
		g.parseLookAt(r)
	case inAttack:
		g.parseAttack(r)
	case inFightModes:
		g.parseFightModes(r)
	case inBuyItem:
		g.parseBuyItem(r)
	case inSellItem:
		g.parseSellItem(r)
	case inCloseShop:
		g.parseCloseShop(r)
	case inRequestChannels:
		g.parseRequestChannels(r)
	case inCloseChannel:
		g.parseCloseChannel(r)
	case inHighscore:
		g.parseHighscores(r)
	case inNpcGreet:
		g.parseNpcGreet(r)
	case inInviteToParty:
		g.deps.World.PlayerInviteToParty(g.player.ID, r.GetU32())
	case inJoinParty:
		g.deps.World.PlayerJoinParty(g.player.ID, r.GetU32())
	case inRevokePartyInvite:
		g.deps.World.PlayerRevokePartyInvitation(g.player.ID, r.GetU32())
	case inPassPartyLeadership:
		g.deps.World.PlayerPassPartyLeadership(g.player.ID, r.GetU32())
	case inLeaveParty:
		g.deps.World.PlayerLeaveParty(g.player.ID)
	case inEnableSharedPartyExp:
		g.deps.World.PlayerEnableSharedPartyExperience(g.player.ID, r.GetByte() == 1)
	case 0x0F: // Ping back
		w := netmsg.NewWriter()
		w.AddByte(0x1D)
		g.SendToClient(w)
	case 0x61:
		g.parseOpenWheel(r)
	case 0x62:
		g.parseSaveWheel(r)
	case 0x8F:
		g.parseQuickLoot(r)
	case 0x90:
		g.parseLootContainer(r)
	case 0x91:
		g.parseQuickLootBlackWhitelist(r)
	case 0xB3: // Telemetry / client checks (quest log / bestiary / resource balance)
		// Handled gracefully without error
	case 0xCD:
		g.parseInspectionObject(r)
	case 0xCE:
		g.parseInspectPlayer(r)
	case 0xCF:
		g.SendBlessingsDialog()
	case 0xD0:
		g.parseOpenRewardChest(r)
	case 0xD2:
		g.SendOutfitWindow()
	case 0xD3:
		g.parseSetOutfit(r)
	case 0xD4:
		g.parseToggleMount(r)
	case 0xD8, 0xD9, 0xDA:
		g.dispatchDailyReward(op, r)
	case 0xD5:
		g.parseImbuementApply(r)
	case 0xD6:
		g.parseImbuementClear(r)
	case 0xD7:
		g.parseCloseImbuementWindow(r)
	case 0xDC:
		g.parseVIPAdd(r)
	case 0xDD:
		g.parseVIPRemove(r)
	case 0xE5:
		g.parseCyclopediaCharacterInfo(r)
	case 0xBF:
		g.parseForgeEnter(r)
	case 0xC0:
		g.parseForgeBrowseHistory(r)
	case 0xEA, 0xEB:
		g.parsePreyAction(r)
	case 0xE7:
		g.parseWheelGemAction(r)
	case 0xB2:
		g.parseImbuementAction(r)
	case 0xBA:
		g.parseTaskHuntingAction(r)
	case 0xAD:
		g.parseCyclopediaHouseAuction(r)
	case 0xAE:
		// C_SendBosstiary: open the Boss Cyclopedia -> send rules + boss list.
		g.SendBosstiaryData()
		g.SendBosstiaryInfo()
	case 0xAF:
		// C_SendBosstiarySlots: open the prowess-slots view -> rules + slots.
		g.SendBosstiaryData()
		g.SendBosstiarySlots()
	case 0xB0:
		// C_BosstiarySlot: set/remove a boss in a prowess slot.
		g.parseBosstiarySlot(r)
	case 0xE1:
		// C_BestiarySendRaces: open the bestiary -> class list + charms window.
		g.SendBestiaryRaces()
		g.SendBestiaryCharms()
	case 0xE2:
		// C_BestiarySendCreatures: monsters within a class.
		g.parseBestiarySendCreatures(r)
	case 0xE3:
		// C_BestiarySendMonsterData: one monster's detail.
		g.parseBestiaryMonsterData(r)
	case 0xE4:
		// C_BuyCharmRune: unlock/upgrade, assign, clear, or reset charms.
		g.parseSendBuyCharmRune(r)
	case inMarketLeave, 0xF4:
		// Market: player leaves the market window.
		g.parseMarketLeave()
	case inMarketBrowse:
		// Market: browse items / own offers / own history.
		g.parseMarketBrowse(r)
	case inMarketCreate:
		// Market: create a buy or sell offer.
		g.parseMarketCreateOffer(r)
	case inMarketCancel:
		// Market: cancel an existing offer.
		g.parseMarketCancelOffer(r)
	case inMarketAccept:
		// Market: accept (execute) an existing offer.
		g.parseMarketAcceptOffer(r)
	case 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xE8, 0xE9, 0xEF:
		// In-game store packets (C_OpenStore/RequestStoreOffers/BuyStoreOffer/
		// transaction history, plus GetOfferDescription/StoreEvent/TransferCoins).
		// Routed to the gamestore Lua module's onRecvbyte handler.
		g.dispatchStore(op, r)
	case inExtendedOpcode:
		// [u8 opcode][str buffer] — ignore for now.
	case inRetrieveDepotSearch:
		g.parseRetrieveDepotSearch(r)
	case inCyclopediaMonsterTracker:
		g.parseCyclopediaMonsterTracker(r)
	case inPartyAnalyzerAction:
		g.parsePartyAnalyzerAction(r)
	case inLeaderFinderWindow:
		g.parseLeaderFinderWindow(r)
	case inMemberFinderWindow:
		g.parseMemberFinderWindow(r)
	case inSetClientOptions:
		g.parseSetClientOptions(r)
	case inPlayerTyping:
		g.parsePlayerTyping(r)
	case inClientCheck:
		g.parseClientCheck(r)
	case inSetVocation:
		g.parseSetVocation(r)
	case inTeleport:
		g.parseTeleport(r)
	case inStartOfflineTraining:
		g.parseStartOfflineTraining(r)
	case inContainerAction:
		g.parseContainerAction(r)
	case inHotkeyEquip:
		g.parseHotkeyEquip(r)
	case inLookInShop:
		g.parseLookInShop(r)
	case inRequestTrade:
		g.parseRequestTrade(r)
	case inLookInTrade:
		g.parseLookInTrade(r)
	case inAcceptTrade:
		g.parseAcceptTrade(r)
	case inCloseTrade:
		g.parseCloseTrade(r)
	case inFriendSystemAction:
		g.parseFriendSystemAction(r)
	case inRotateItem:
		g.parseRotateItem(r)
	case inConfigureShowOffSocket:
		g.parseConfigureShowOffSocket(r)
	case inTextWindow:
		g.parseTextWindow(r)
	case inHouseWindow:
		g.parseHouseWindow(r)
	case inWrapableItem:
		g.parseWrapableItem(r)
	case inLookInBattleList:
		g.parseLookInBattleList(r)
	case inJoinAggression:
		g.parseJoinAggression(r)
	case inOpenDepotSearch:
		g.parseOpenDepotSearch(r)
	case inCloseDepotSearch:
		g.parseCloseDepotSearch(r)
	case inDepotSearchItemRequest:
		g.parseDepotSearchItemRequest(r)
	case inOpenParentContainer:
		g.parseOpenParentContainer(r)
	case inEditGuildMessage:
		g.parseEditGuildMessage(r)
	case inGetTextForReport:
		g.parseGetTextForReport(r)
	case inCloseNpcChannel:
		g.parseCloseNpcChannel(r)
	case inSetMonsterPodium:
		g.parseSetMonsterPodium(r)
	case inFollow:
		g.parseFollow(r)

	// --- Batch 2: previously undispatched handlers ---
	case inCharacterTradeConfig:
		g.parseCharacterTradeConfig(r)
	case inOpenChannel:
		g.parseOpenChannel(r)
	case inOpenPrivateChannel:
		g.parseOpenPrivateChannel(r)
	case inCreatePrivateChannel:
		g.parseCreatePrivateChannel(r)
	case inChannelInvite:
		g.parseChannelInvite(r)
	case inChannelExclude:
		g.parseChannelExclude(r)
	case inClientDetails:
		g.parseClientDetails(r)
	case inBossDifficultySelection:
		g.parseBossDifficultySelection(r)
	case inAimAtTarget:
		g.parseAimAtTarget(r)
	case inGetTransactionDetails:
		g.parseGetTransactionDetails(r)
	case inExivaRestrictions:
		// C++ gates this on !oldProtocol && getWorldType() == WORLD_TYPE_NO_PVP
		// (protocolgame.cpp:1947).
		if g.deps.World != nil && g.deps.World.WorldType == worldTypeNoPvp {
			g.parseExivaRestrictions(r)
		}
	case inCyclopediaMapAction:
		g.parseCyclopediaMapAction(r)
	case inVIPEdit:
		g.parseVIPEdit(r)
	case inVipGroupActions:
		g.parseVipGroupActions(r)
	case inBugReport:
		g.parseBugReport(r)
	case inSendResourceBalance:
		g.parseRequestRuleChannels(r)
	case inQuestLog:
		g.parseQuestLog(r)
	case inQuestLine:
		g.parseQuestLine(r)
	case inModalWindowAnswer:
		g.parseModalWindowAnswer(r)
	case inRewardChestCollect:
		g.parseRewardChestCollect(r)
	default:
		// Log the remaining payload so each not-yet-migrated action can be
		// mapped to its C++ parse* handler from the exact wire bytes.
		rest := r.Buffer()[r.Pos():]
		c.Logger().Debug("unhandled game opcode", "op", fmt.Sprintf("0x%02X", op), "payload_hex", fmt.Sprintf("%x", rest))
	}
}
