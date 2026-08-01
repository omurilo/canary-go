package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/network"
	"github.com/opentibiabr/canary-go/internal/tibcrypto"
	"github.com/opentibiabr/canary-go/internal/transport"
	"strings"
	"time"
)

// tibiaHeaderLength is the 2-byte little-endian body length every Tibia packet
// starts with (network.headerLength).
const tibiaHeaderLength = 2

// Login opcodes (outbound).
const (
	opLoginError      = 0x0B
	opLoginMOTD       = 0x14
	opLoginSessionKey = 0x28
	opLoginCharList   = 0x64
)

// LoginProtocol serves the account login / character-list flow (port 7171).
type LoginProtocol struct {
	deps *Deps
}

// NewLoginFactory returns a factory that builds LoginProtocol instances.
func NewLoginFactory(deps *Deps) network.ProtocolFactory {
	return func() network.Protocol { return &LoginProtocol{deps: deps} }
}
func (p *LoginProtocol) OnConnect(c *network.Connection) { c.RawFirstPacket = true }

// sendStatusString sends XML server status matching C++ ProtocolStatus::sendStatusString.
func (p *LoginProtocol) sendStatusString(c *network.Connection) {
	uptime := uint64(time.Since(serverStartTime).Seconds())
	serverName := "Canary-Go"
	serverPort := 7171
	serverIP := ""
	location := ""
	url := ""
	ownerName := ""
	ownerEmail := ""
	mapName := ""
	mapAuthor := ""
	var mapWidth, mapHeight uint32
	maxPlayers := uint32(0)
	onlineCount := uint32(0)
	uniqueIPs := uint32(0)
	peakPlayers := uint32(0)
	monstersOnline := uint32(0)
	npcsOnline := uint32(0)
	var expRate, skillRate, lootRate, magicRate, spawnRate uint32 = 1, 1, 1, 1, 1
	motd := ""
	if p.deps != nil {
		cfg := p.deps.Cfg
		if cfg != nil {
			serverName = cfg.ServerName
			serverPort = cfg.LoginPort
			serverIP = cfg.IP
			if cfg.MOTD != "" {
				motd = cfg.MOTD
			}
		}
		world := p.deps.World
		if world != nil {
			onlineCount = uint32(world.OnlineCount())
			uniqueIPs = onlineCount
			peakPlayers = onlineCount
		}
	}
	boostedCreature := ""
	boostedBoss := ""
	if p.deps != nil && p.deps.World != nil {
		boostedCreature = p.deps.World.BoostedCreature
		boostedBoss = p.deps.World.BoostedBoss
	}
	var b strings.Builder
	// Pure XML for binary status protocol (no HTTP headers for MyAAC).
	b.WriteString("HTTP/1.1 200 OK\r\n")
	b.WriteString("Content-Type: text/xml\r\n")
	b.WriteString("Connection: close\r\n")
	b.WriteString("\r\n")
	b.WriteString("<?xml version=\"1.0\"?>\n")
	b.WriteString("<tsqp version=\"1.0\">\n")
	fmt.Fprintf(&b, "\t<serverinfo uptime=\"%d\" ip=\"%s\" servername=\"%s\" port=\"%d\" location=\"%s\" url=\"%s\" server=\"%s\" version=\"1.0\" client=\"15.25\"/>\n",
		uptime, serverIP, serverName, serverPort, location, url, serverName)
	fmt.Fprintf(&b, "\t<boostedCreature name=\"%s\"/>\n", boostedCreature)
	fmt.Fprintf(&b, "\t<boostedBoss name=\"%s\"/>\n", boostedBoss)
	fmt.Fprintf(&b, "\t<owner name=\"%s\" email=\"%s\"/>\n", ownerName, ownerEmail)
	fmt.Fprintf(&b, "\t<players online=\"%d\" unique=\"%d\" max=\"%d\" peak=\"%d\"/>\n",
		onlineCount, uniqueIPs, maxPlayers, peakPlayers)
	fmt.Fprintf(&b, "\t<monsters total=\"%d\"/>\n", monstersOnline)
	fmt.Fprintf(&b, "\t<npcs total=\"%d\"/>\n", npcsOnline)
	fmt.Fprintf(&b, "\t<rates experience=\"%d\" skill=\"%d\" loot=\"%d\" magic=\"%d\" spawn=\"%d\"/>\n",
		expRate, skillRate, lootRate, magicRate, spawnRate)
	fmt.Fprintf(&b, "\t<map name=\"%s\" author=\"%s\" width=\"%d\" height=\"%d\"/>\n",
		mapName, mapAuthor, mapWidth, mapHeight)
	if motd != "" {
		fmt.Fprintf(&b, "\t<motd>%s</motd>\n", motd)
	}
	b.WriteString("</tsqp>\n")
	c.WriteRaw([]byte(b.String()))
}

// sendMyAACStatus sends pure XML status (no HTTP headers) for MyAAC connections.
func (p *LoginProtocol) sendMyAACStatus(c *network.Connection) {
	uptime := uint64(time.Since(serverStartTime).Seconds())
	serverName := "Canary-Go"
	serverIP := ""
	onlineCount := uint32(0)
	uniqueIPs := uint32(0)
	if p.deps != nil {
		cfg := p.deps.Cfg
		if cfg != nil {
			serverName = cfg.ServerName
			serverIP = cfg.IP
		}
		if p.deps.World != nil {
			onlineCount = uint32(p.deps.World.OnlineCount())
			uniqueIPs = onlineCount
		}
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\"?>\n")
	b.WriteString("<tsqp version=\"1.0\">\n")
	fmt.Fprintf(&b, "\t<serverinfo uptime=\"%d\" ip=\"%s\" servername=\"%s\" port=\"%d\" server=\"Canary\" version=\"1.0\" client=\"15.25\"/>\n",
		uptime, serverIP, serverName, 7171)
	fmt.Fprintf(&b, "\t<owner name=\"\" email=\"\"/>\n")
	fmt.Fprintf(&b, "\t<players online=\"%d\" unique=\"%d\" max=\"2000\" peak=\"0\"/>\n", onlineCount, uniqueIPs)
	fmt.Fprintf(&b, "\t<monsters total=\"0\"/>\n\t<npcs total=\"0\"/>\n")
	fmt.Fprintf(&b, "\t<rates experience=\"1\" skill=\"1\" loot=\"1\" magic=\"1\" spawn=\"1\"/>\n")
	fmt.Fprintf(&b, "\t<map name=\"\" author=\"\" width=\"0\" height=\"0\"/>\n")
	b.WriteString("</tsqp>\n")
	c.WriteRaw([]byte(b.String()))
}

func (p *LoginProtocol) OnDisconnect(c *network.Connection)               {}
func (p *LoginProtocol) OnPacket(c *network.Connection, r *netmsg.Reader) {}

// handleHTTPRequest parses an HTTP request body and returns the appropriate JSON response.
func (p *LoginProtocol) handleHTTPRequest(c *network.Connection, body []byte) {

	var req struct {
		Type string `json:"type"`
	}
	// Try to extract JSON body from HTTP request (find the JSON after headers)
	raw := string(body)
	jsonStart := strings.Index(raw, "{")
	if jsonStart < 0 {
		jsonError(c, "Invalid request")
		return
	}
	if err := json.Unmarshal([]byte(raw[jsonStart:]), &req); err != nil {
		jsonError(c, "Invalid JSON")
		return
	}
	switch req.Type {
	case "cacheinfo":
		jsonCacheInfo(c, p)
	case "boostedcreature":
		jsonBoostedCreature(c, p)
	case "eventschedule":
		jsonEmpty(c) // not implemented
	case "showoff":
		jsonEmpty(c) // not implemented
	default:
		jsonError(c, "Unknown type: "+req.Type)
	}
}
func jsonError(c *network.Connection, msg string) {
	resp, _ := json.Marshal(map[string]string{"errorMessage": msg, "errorCode": "400"})
	sendJSON(c, resp)
}
func jsonEmpty(c *network.Connection) {
	resp, _ := json.Marshal(map[string]string{})
	sendJSON(c, resp)
}
func sendJSON(c *network.Connection, data []byte) {
	resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(data), string(data))
	_ = c.WriteRaw([]byte(resp))
	time.Sleep(50 * time.Millisecond) // Give client time to receive response
}
func jsonCacheInfo(c *network.Connection, p *LoginProtocol) {
	online := uint32(0)
	if p.deps != nil && p.deps.World != nil {
		online = uint32(p.deps.World.OnlineCount())
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"playersonline":        online,
		"discord_online":       0,
		"gamingyoutubestreams": 0,
		"gamingyoutubeviewer":  0,
		"youtube_link":         "",
		"discord_link":         "",
	})
	sendJSON(c, resp)
}
func jsonBoostedCreature(c *network.Connection, p *LoginProtocol) {
	creatureRaceID := uint16(0)
	bossRaceID := uint16(0)

	boostedCreature := ""
	boostedBoss := ""
	if p.deps != nil && p.deps.World != nil {
		boostedCreature = p.deps.World.BoostedCreature
		if boostedCreature == "" || boostedCreature == "default" {
			boostedCreature = "Dragon" // sensible fallback
		}
		p.deps.World.EnsureBoostedBoss()
		boostedBoss = p.deps.World.BoostedBoss

		// NOTE: Lua monster types live in World.TypeRegistry.Monsters, NOT World.Monsters
		// (which is always nil). Use the registry that the Lua loader populates.
		reg := p.deps.World.TypeRegistry
		if reg != nil {
			// Look up the boosted creature's bestiary race ID
			if name := strings.TrimSpace(boostedCreature); name != "" && name != "default" {
				if mt, ok := reg.Monsters[strings.ToLower(name)]; ok {
					creatureRaceID = mt.RaceID
				}
			}
			// Look up the boosted boss's bosstiary race ID
			if name := strings.TrimSpace(boostedBoss); name != "" && name != "None" && name != "default" {
				if mt, ok := reg.Monsters[strings.ToLower(name)]; ok {
					if mt.BosstiaryRaceID != 0 {
						bossRaceID = mt.BosstiaryRaceID
					} else {
						bossRaceID = mt.RaceID // fallback
					}
				}
			}
		}
	}
	// The client's setBoostedCreatureAndBoss expects:
	//   data.creatureraceid (or data.raceid for backwards compat)
	//   data.bossraceid
	resp, _ := json.Marshal(map[string]interface{}{
		"boostedCreature": boostedCreature,
		"boostedBoss":     boostedBoss,
		"raceId":          creatureRaceID, // backwards compat
		"creatureraceid":  creatureRaceID,
		"bossraceid":      bossRaceID,
		"bonus":           "",
		"bonusXp":         0,
		"bonusLoot":       0,
		"bonusSkill":      0,
	})
	sendJSON(c, resp)
}

// OnFirstPacket parses the login request, authenticates and replies.
func (p *LoginProtocol) OnFirstPacket(c *network.Connection, body []byte) {

	// MyAAC binary status: [2-byte len] + FF FF "info"
	if len(body) >= 8 && body[0] == 0x06 && body[1] == 0x00 && body[2] == 0xFF && body[3] == 0xFF && string(body[4:8]) == "info" {
		p.sendMyAACStatus(c)
		c.Close()
		return
	}
	// Legacy FF FF "info" without length header
	if len(body) >= 6 && body[0] == 0xFF && body[1] == 0xFF && string(body[2:6]) == "info" {
		p.sendMyAACStatus(c)
		c.Close()
		return
	}
	// HTTP requests: POST → JSON, GET → XML
	if len(body) > 0 && body[0] >= 'A' && body[0] <= 'Z' {
		if len(body) >= 4 && string(body[:4]) == "POST" {
			p.handleHTTPRequest(c, body)
		} else {
			p.sendStatusString(c)
		}
		c.Close()
		return
	}
	// Everything below is the binary Tibia login, ProtocolLogin::onRecvFirstMessage
	// (protocollogin.cpp:203-240). The MyAAC and HTTP probes above are a Go-only
	// addition and match on the still-attached length header, which is why the
	// framing is only stripped here.
	body, modernPad := stripLoginFraming(body, c)
	body = transport.StripFirstPacketChecksum(body)
	if modernPad {
		var ok bool
		if body, ok = stripModernPadding(body); !ok {
			c.Logger().Debug("login: invalid modern padding")
			return
		}
	}
	r := netmsg.NewReader(body)
	protoID := r.GetByte() // protocol id (0x01), consumed by the service dispatch in C++
	clientOS := r.GetU16() // msg.skipBytes(2)      protocollogin.cpp:209
	version := r.GetU16()  // msg.get<uint16_t>()   protocollogin.cpp:211

	// resolveLoginLayout, reduced to the one thing it decides here: how much
	// pre-RSA metadata to skip (AccountLoginLayout::bytesToSkipBeforeRsa,
	// protocol_profile.cpp:287-325). 17 for the modern and 11.00 profiles — a u32
	// client version, three u32 asset signatures and a preview byte — and 12 for
	// 8.60, which reads the signatures itself rather than skipping them.
	//
	// This was a bare Skip(13) after an inline GetU32, which is the same 17 bytes
	// for the modern profile only. Naming it means an 8.60 login on this port is a
	// visible gap rather than a silent misread.
	skip := loginBytesToSkipBeforeRSA(ProtocolVersion(version))
	if r.Remaining() < skip+tibcrypto.BlockSize {
		c.Logger().Warn("login: packet too short for the RSA block",
			"version", version, "remaining", r.Remaining(), "need", skip+tibcrypto.BlockSize)
		return
	}
	clientVersion := uint32(0)
	if skip >= 4 {
		clientVersion = r.GetU32()
		r.Skip(skip - 4)
	} else {
		r.Skip(skip)
	}
	c.Logger().Info("login: parsed prelude", "protoId", protoID, "clientOS", clientOS,
		"version", version, "clientVersion", clientVersion, "payloadLen", len(body))
	block := r.GetBytes(tibcrypto.BlockSize)
	if err := p.deps.RSA.Decrypt(block); err != nil {
		c.Logger().Debug("login: rsa decrypt failed", "err", err)
		return
	}
	if block[0] != 0 {
		c.Logger().Debug("login: rsa leading byte non-zero")
		return
	}
	br := netmsg.NewReader(block[1:])
	key := tibcrypto.KeyFromBytes(br.GetBytes(16))
	account := br.GetString()
	password := br.GetString()
	// From here responses are XTEA-encrypted with an Adler checksum.
	c.Codec().EnableModernLogin(key)
	c.Logger().Info("login attempt", "account", account, "version", version, "client", clientVersion)
	if account == "" {
		p.disconnect(c, "Invalid account name.")
		return
	}
	if password == "" {
		p.disconnect(c, "Invalid password.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	acc, err := p.deps.DB.LoadAccount(ctx, account)
	if err != nil || !db.VerifyPassword(password, acc.Password) {
		p.disconnect(c, "Account name or password is not correct.")
		return
	}
	chars, err := p.deps.DB.ListCharacters(ctx, acc.ID)
	if err != nil {
		c.Logger().Warn("login: listing characters failed", "account", account, "err", err)
		p.disconnect(c, "Unable to load your characters.")
		return
	}
	names := make([]string, 0, len(chars))
	for _, ch := range chars {
		names = append(names, ch.Name)
	}
	c.Logger().Info("login: character list", "account", account, "accountId", acc.ID,
		"count", len(chars), "names", strings.Join(names, ","),
		"worldName", p.deps.Cfg.ServerName, "worldIP", p.deps.Cfg.IP,
		"gamePort", p.deps.Cfg.GamePort)
	// ONE message carrying all three parts, as getCharacterList does
	// (protocollogin.cpp:60-148: a single OutputMessage, one send() at the end).
	//
	// These used to be three separate sends, and the client only ever saw the
	// first: ProtocolLogin:onRecv drains the opcodes of one message and then
	// calls self:disconnect() unconditionally
	// (otclient/modules/gamelib/protocollogin.lua:156-186). With a MOTD
	// configured the client read the MOTD, hung up, and never received the
	// character list — login "worked" and the list was empty.
	w := netmsg.NewWriter()
	p.writeMOTD(w)
	p.writeSessionKey(w, account, password)
	p.writeCharacterList(w, acc, chars)
	_ = c.Send(w)
	c.Close()
}
func (p *LoginProtocol) disconnect(c *network.Connection, msg string) {
	w := netmsg.NewWriter()
	w.AddByte(opLoginError)
	w.AddString(msg)
	_ = c.Send(w)
	c.Close()
}
func (p *LoginProtocol) writeMOTD(w *netmsg.Writer) {
	if p.deps.Cfg.MOTD == "" {
		return
	}
	w.AddByte(opLoginMOTD)
	w.AddString(fmt.Sprintf("1\n%s", p.deps.Cfg.MOTD))
}
func (p *LoginProtocol) writeSessionKey(w *netmsg.Writer, account, password string) {
	w.AddByte(opLoginSessionKey)
	w.AddString(account + "\n" + password)
}
func (p *LoginProtocol) writeCharacterList(w *netmsg.Writer, acc *db.Account, chars []db.Character) {
	w.AddByte(opLoginCharList)
	// Worlds list (single world).
	w.AddByte(1) // number of worlds
	w.AddByte(0) // world id
	w.AddString(p.deps.Cfg.ServerName)
	w.AddString(p.deps.Cfg.IP)
	w.AddU16(uint16(p.deps.Cfg.GamePort))
	w.AddByte(0) // preview/world-flag
	// Characters list.
	n := len(chars)
	if n > 255 {
		n = 255
	}
	w.AddByte(byte(n))
	for i := 0; i < n; i++ {
		w.AddByte(0) // world id
		w.AddString(chars[i].Name)
	}
	// Premium info.
	premDays := acc.PremDays
	if premDays < 0 {
		premDays = 0
	}
	premTrunc := premDays
	if premTrunc > 255 {
		premTrunc = 255
	}
	isPremium := byte(0)
	var premLastDay uint32
	if premDays > 0 {
		isPremium = 1
		premLastDay = uint32(time.Now().Add(time.Duration(premDays) * 24 * time.Hour).Unix())
	}
	w.AddByte(byte(premTrunc))
	w.AddByte(isPremium)
	w.AddU32(premLastDay)
}

// stripLoginFraming removes the outer 2-byte header from a first login packet
// and reports whether the client used the modern (>= 1405) framing, which also
// carries a padding byte the caller has to remove after the checksum.
//
// The login service reads in RawFirstPacket mode so it can recognise HTTP and
// MyAAC status probes, so the header is still attached and has to be parsed by
// hand. The two forms are:
//
//	legacy   u16 = body length         (OutputMessage::writeMessageSize)
//	modern   u16 = XTEA block count    (OutputMessage::writeHeaderSize, >= 1405)
//	              = (messageSize - 4) / 8, the -4 being the checksum
//
// Only the legacy form was recognised. A 13.x client's header is a block count,
// which never equals the byte length, so the two bytes were left in front of the
// payload; every field after that read two bytes early and the RSA block came
// out as noise. The first decrypted byte then failed its "must be zero" check
// and the connection was dropped in silence — the reported
//
//	rsa leading byte non-zero
//
// The two tests cannot both pass: declared == declared*8+4 has no solution.
func stripLoginFraming(body []byte, c *network.Connection) (out []byte, modernPad bool) {
	if len(body) < tibiaHeaderLength {
		return body, false
	}
	declared := int(body[0]) | int(body[1])<<8
	rest := len(body) - tibiaHeaderLength

	// Modern: block count * 8 + the 4 checksum bytes, the same arithmetic
	// Codec.DecodeBodySize does for the game port.
	if declared*8+4 == rest {
		if c != nil {
			c.Logger().Debug("login: modern framing", "blocks", declared, "bytes", rest)
		}
		return body[tibiaHeaderLength:], true
	}
	if declared == rest {
		return body[tibiaHeaderLength:], false
	}
	// Neither: leave it alone rather than guess. Dump the head so the actual
	// shape is identifiable instead of being inferred from two numbers — a
	// mismatch here is either a framing form nobody has ported or a truncated
	// read, and the bytes say which.
	if c != nil {
		head := body
		if len(head) > 48 {
			head = head[:48]
		}
		c.Logger().Warn("login: unrecognised outer header",
			"declared", declared, "rest", rest, "total", len(body),
			"hex", fmt.Sprintf("% X", head), "ascii", printableASCII(head))
	}
	return body, false
}

// printableASCII renders a byte slice with non-printable bytes as dots, so a
// text protocol arriving on the binary path is obvious at a glance.
func printableASCII(b []byte) string {
	out := make([]byte, len(b))
	for i, ch := range b {
		if ch >= 0x20 && ch < 0x7F {
			out[i] = ch
		} else {
			out[i] = '.'
		}
	}
	return string(out)
}

// stripModernPadding removes OutputMessage::writePaddingAmount's leading count
// byte and the trailing filler it describes. The first login packet is not
// XTEA-encrypted — encryption is only enabled after it is sent — but a 13.x
// client pads it anyway, so the padding is sitting in the clear.
func stripModernPadding(body []byte) ([]byte, bool) {
	if len(body) < 1 {
		return body, false
	}
	pad := int(body[0])
	end := len(body) - pad
	if end < 1 {
		return body, false
	}
	return body[1:end], true
}

// loginBytesToSkipBeforeRSA is AccountLoginLayout::bytesToSkipBeforeRsa
// (src/server/network/protocol/protocol_profile.cpp:287-325), the pre-RSA
// metadata each profile carries after the protocol version:
//
//	modern / 11.00   17   u32 client version + 3 u32 asset signatures + preview byte
//	8.60             12   the three signatures, which that profile reads rather
//	                      than skips so the asset contract can pick the profile
//
// The 8.60 case is listed for completeness. Nothing resolves a profile from the
// signatures here yet, so an 8.60 login on this port gets the right byte count
// and no profile switch — a gap, but a visible one.
func loginBytesToSkipBeforeRSA(version ProtocolVersion) int {
	if version <= VersionCipsoft860 {
		return 12
	}
	return 17
}
