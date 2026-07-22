package protocol

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/network"
	"github.com/opentibiabr/canary-go/internal/tibcrypto"
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
	// Inbound party opcodes (0xA3..0xA8). NOTE: 0xA3 collides with the OUTBOUND
	// opCancelTarget const — these are a separate inbound namespace.
	inInviteToParty        = 0xA3
	inJoinParty            = 0xA4
	inRevokePartyInvite    = 0xA5
	inPassPartyLeadership  = 0xA6
	inLeaveParty           = 0xA7
	inEnableSharedPartyExp = 0xA8
)

// GameProtocol is one game-server session.
type GameProtocol struct {
	deps *Deps
	conn *network.Connection

	challengeTS   uint32
	challengeRand uint8
	loggedIn      bool

	player *game.Player
	known  map[uint32]bool

	statementID uint32

	pingStop chan struct{} // closed once to stop the keep-alive ping loop
	pingOnce sync.Once

	actionMu sync.Mutex    // serializes player movement (walk/turn/auto-walk step)
	walkGen  atomic.Uint64 // bumping cancels the in-flight auto-walk path
}

// openContainerByCID returns the container open under a client cid, preserving
// the (item, ok) shape callers expect. The open-container state is the single
// source of truth on game.Player (see Player.openContainers).
func (g *GameProtocol) openContainerByCID(cid uint8) (*game.Item, bool) {
	if g.player == nil {
		return nil, false
	}
	c := g.player.GetContainerByID(cid)
	return c, c != nil
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

// NewGameFactory returns a factory building GameProtocol instances.
func NewGameFactory(deps *Deps) network.ProtocolFactory {
	return func() network.Protocol {
		return &GameProtocol{
			deps:       deps,
			known:      make(map[uint32]bool),
			pingStop:   make(chan struct{}),
		}
	}
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
func (g *GameProtocol) Player() *game.Player { return g.player }

// SendToClient implements game.Session.
func (g *GameProtocol) SendToClient(w *netmsg.Writer) {
	if g.conn != nil {
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
	// The game connection uses the CurrentModern transport: block-count outer
	// length for every packet, including the pre-encryption challenge and login.
	c.Codec().EnableModernFraming()

	// Deterministic-ish challenge derived from the clock; values are echoed back.
	now := uint32(time.Now().Unix())
	g.challengeTS = now
	g.challengeRand = uint8(now & 0xFF)

	// Modern challenge layout (ProtocolGame::sendLoginChallenge, CurrentLogin-
	// Challenge): [adler32(payload)][0x01][0x1F][ts u32][rand u8][0x71]. The adler
	// covers the 8 payload bytes; the codec prepends the u16 length (encryption is
	// still off here). Real clients validate this checksum and the 0x01 marker.
	payload := netmsg.NewWriter()
	payload.AddByte(0x01)
	payload.AddByte(opChallenge)
	payload.AddU32(g.challengeTS)
	payload.AddByte(g.challengeRand)
	payload.AddByte(0x71) // trailing constant
	pb := payload.Bytes()

	w := netmsg.NewWriter()
	w.AddU32(tibcrypto.Adler32(pb))
	w.AddBytes(pb)
	_ = c.Send(w)
}

// OnDisconnect saves and removes the player.
func (g *GameProtocol) OnDisconnect(c *network.Connection) {
	g.stopPingLoop()
	if g.player == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Update last logout timestamp and execute creature event logout callbacks
	g.player.LastLogout = uint64(time.Now().Unix())
	g.deps.Lua.ExecuteCreatureOnLogout(g.player)

	if err := g.deps.DB.SavePlayer(ctx, g.player); err != nil {
		c.Logger().Warn("save on disconnect failed", "err", err)
	}
	g.deps.World.RemovePlayer(g.player.ID)
	g.broadcastRemove(g.player)
	c.Logger().Info("player logged out", "name", g.player.Name, "online", g.deps.World.OnlineCount())
	g.player = nil
}

// firstGamePacketHeaderBytes mirrors CurrentModern's serverFirstPacketHeaderBytes
// (CHECKSUM_LENGTH + 2): the first game packet leads with a 4-byte checksum slot
// and 2 further bytes that the reference server skips before the login fields.
const firstGamePacketHeaderBytes = 6

// OnFirstPacket parses the game login request. The wire layout mirrors
// ProtocolGame::onRecvFirstMessage for the Current (1525) profile.
func (g *GameProtocol) OnFirstPacket(c *network.Connection, body []byte) {
	g.conn = c
	if len(body) < firstGamePacketHeaderBytes {
		c.Logger().Debug("game: short first packet", "len", len(body))
		return
	}
	r := netmsg.NewReader(body[firstGamePacketHeaderBytes:])

	os := r.GetU16()
	_ = r.GetU16() // protocol version
	clientVersion := r.GetU32()
	_ = r.GetString() // client version string
	_ = r.GetString() // asset hash
	_ = r.GetByte()   // preview state

	if r.Remaining() < tibcrypto.BlockSize {
		c.Logger().Debug("game: short packet, no RSA block")
		return
	}
	block := r.GetBytes(tibcrypto.BlockSize)
	if err := g.deps.RSA.Decrypt(block); err != nil || block[0] != 0 {
		c.Logger().Debug("game: rsa decrypt failed")
		return
	}
	// Everything from here is inside the single 128-byte RSA block: XTEA key,
	// gamemaster flag, session key, character name and the challenge echo.
	br := netmsg.NewReader(block[1:])
	key := tibcrypto.KeyFromBytes(br.GetBytes(16))
	_ = br.GetByte() // is gamemaster
	sessionKey := br.GetString()
	charName := br.GetString()
	echoTS := br.GetU32()
	echoRand := br.GetByte()

	// Enable encryption for everything from here on.
	g.conn.Codec().EnableModernGame(key)

	if clientVersion != 0 && clientVersion != ClientVersion {
		g.disconnect("Wrong client version. Please use a 13.x (protocol 1525) client.")
		return
	}
	if echoTS != g.challengeTS || echoRand != g.challengeRand {
		c.Logger().Debug("game: challenge mismatch", "gotTS", echoTS, "wantTS", g.challengeTS)
		// Non-fatal for our own client; continue.
	}

	account, password := splitSessionKey(sessionKey)
	c.Logger().Info("game login", "char", charName, "account", account, "os", os)

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
	_ = g.conn.Send(w)
	g.conn.Close()
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
	w.AddU16(0)                      // store coin package size
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

	g.addStats(w)
	g.addSkills(w)

	// 0x82 world light.
	w.AddByte(opWorldLight)
	w.AddByte(0xFF) // full daylight
	w.AddByte(0xD7)

	// 0x8D creature light.
	w.AddByte(opCreatureLight)
	w.AddU32(p.ID)
	w.AddByte(0)
	w.AddByte(0)

	g.addBasicData(w)

	g.SendToClient(w)

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
	w.AddU32(p.MaxHealth)

	totalCap := p.Capacity * 100
	usedCap := g.getPlayerTotalWeight()
	freeCap := uint32(0)
	if totalCap > usedCap {
		freeCap = totalCap - usedCap
	}
	w.AddU32(freeCap)
	
	w.AddU64(p.Experience)
	w.AddU16(p.Level)
	w.AddU16(0)   // level percent (0-10000)
	w.AddU16(100) // base xp gain
	w.AddU16(0)   // grinding xp boost
	w.AddU16(0)   // xp boost percent
	w.AddU16(0)   // stamina multiplier
	w.AddU32(p.Mana)
	w.AddU32(p.MaxMana)
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

func (g *GameProtocol) addBasicData(w *netmsg.Writer) {
	p := g.player
	w.AddByte(opBasicData)
	w.AddByte(1) // is premium
	w.AddU32(0)  // premium expire ts
	w.AddByte(uint8(p.Vocation))
	w.AddByte(0) // has reached main
	w.AddU16(0)  // spell count
	w.AddByte(0) // magic shield active
}

// OnPacket dispatches steady-state game packets.
func (g *GameProtocol) OnPacket(c *network.Connection, r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	op := r.GetByte()
	switch op {
	case inLogout:
		c.Close()
	case inPing, 0x1C:
		// Client keep-alive ping — answer with a ping-back (0x1E).
		w := netmsg.NewWriter()
		w.AddByte(opPingBack)
		g.SendToClient(w)
	case inPong, 0x60, 0xBE:
		// Reply to our own keep-alive ping, or safely ignored opcodes (imbuements/cancel attack).
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
	case inExtendedOpcode:
		// [u8 opcode][str buffer] — ignore for now.
	default:
		// Log the remaining payload so each not-yet-migrated action can be
		// mapped to its C++ parse* handler from the exact wire bytes.
		rest := r.Buffer()[r.Pos():]
		c.Logger().Debug("unhandled game opcode", "op", fmt.Sprintf("0x%02X", op), "payload_hex", fmt.Sprintf("%x", rest))
	}
}
