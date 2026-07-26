package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/network"
	"github.com/opentibiabr/canary-go/internal/tibcrypto"
	"github.com/opentibiabr/canary-go/internal/transport"
)
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
func (p *LoginProtocol) OnConnect(c *network.Connection) { c.RawFirstPacket = true; println("LoginProtocol.OnConnect") }
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
	maxPlayers := uint32(2000)
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
	fmt.Fprintf(&b, "\t<serverinfo uptime=\"%d\" ip=\"%s\" servername=\"%s\" port=\"%d\" location=\"%s\" url=\"%s\" server=\"%s\" version=\"1.0\" client=\"13.15\"/>\n",
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
		if cfg != nil { serverName = cfg.ServerName; serverIP = cfg.IP }
		if p.deps.World != nil {
			onlineCount = uint32(p.deps.World.OnlineCount())
			uniqueIPs = onlineCount
		}
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\"?>\n")
	b.WriteString("<tsqp version=\"1.0\">\n")
	fmt.Fprintf(&b, "\t<serverinfo uptime=\"%d\" ip=\"%s\" servername=\"%s\" port=\"%d\" server=\"Canary\" version=\"1.0\" client=\"13.15\"/>\n",
		uptime, serverIP, serverName, 7171)
	fmt.Fprintf(&b, "\t<owner name=\"\" email=\"\"/>\n")
	fmt.Fprintf(&b, "\t<players online=\"%d\" unique=\"%d\" max=\"2000\" peak=\"0\"/>\n", onlineCount, uniqueIPs)
	fmt.Fprintf(&b, "\t<monsters total=\"0\"/>\n\t<npcs total=\"0\"/>\n")
	fmt.Fprintf(&b, "\t<rates experience=\"1\" skill=\"1\" loot=\"1\" magic=\"1\" spawn=\"1\"/>\n")
	fmt.Fprintf(&b, "\t<map name=\"\" author=\"\" width=\"0\" height=\"0\"/>\n")
	b.WriteString("</tsqp>\n")
	c.WriteRaw([]byte(b.String()))
}

func (p *LoginProtocol) OnDisconnect(c *network.Connection) { println("LoginProtocol.OnDisconnect") }
func (p *LoginProtocol) OnPacket(c *network.Connection, r *netmsg.Reader) {}
// handleHTTPRequest parses an HTTP request body and returns the appropriate JSON response.
func (p *LoginProtocol) handleHTTPRequest(c *network.Connection, body []byte) {
	fmt.Printf("handleHTTPRequest: body=%s\n", string(body))
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
		"playersonline":          online,
		"discord_online":         0,
		"gamingyoutubestreams":   0,
		"gamingyoutubeviewer":    0,
		"youtube_link":           "",
		"discord_link":           "",
	})
	sendJSON(c, resp)
}
func jsonBoostedCreature(c *network.Connection, p *LoginProtocol) {
	boostedCreature := ""
	boostedBoss := ""
	if p.deps != nil && p.deps.World != nil {
		boostedCreature = p.deps.World.BoostedCreature
		boostedBoss = p.deps.World.BoostedBoss
	}
	// The client expects fields that setBoostedCreatureAndBoss can parse
	resp, _ := json.Marshal(map[string]interface{}{
		"boostedCreature": boostedCreature,
		"boostedBoss":     boostedBoss,
		"raceId":          0,
		"bonus":           "",
		"bonusXp":         0,
		"bonusLoot":       0,
		"bonusSkill":      0,
	})
	sendJSON(c, resp)
}
// OnFirstPacket parses the login request, authenticates and replies.
func (p *LoginProtocol) OnFirstPacket(c *network.Connection, body []byte) {
	fmt.Printf("OnFirstPacket: len=%d hex=%.30x\n", len(body), body)
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
	body = transport.StripFirstPacketChecksum(body)
	r := netmsg.NewReader(body)
	_ = r.GetByte()       // protocol id (0x01)
	_ = r.GetU16()        // operating system
	version := r.GetU16() // protocol version
	clientVersion := r.GetU32()
	r.Skip(13) // 12 asset signatures + 1 preview byte
	if r.Remaining() < tibcrypto.BlockSize {
		c.Logger().Debug("login: short packet, no RSA block")
		return
	}
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
		p.disconnect(c, "Unable to load your characters.")
		return
	}
	p.sendMOTD(c)
	p.sendSessionKey(c, account, password)
	p.sendCharacterList(c, acc, chars)
	c.Close()
}
func (p *LoginProtocol) disconnect(c *network.Connection, msg string) {
	w := netmsg.NewWriter()
	w.AddByte(opLoginError)
	w.AddString(msg)
	_ = c.Send(w)
	c.Close()
}
func (p *LoginProtocol) sendMOTD(c *network.Connection) {
	if p.deps.Cfg.MOTD == "" {
		return
	}
	w := netmsg.NewWriter()
	w.AddByte(opLoginMOTD)
	w.AddString(fmt.Sprintf("1\n%s", p.deps.Cfg.MOTD))
	_ = c.Send(w)
}
func (p *LoginProtocol) sendSessionKey(c *network.Connection, account, password string) {
	w := netmsg.NewWriter()
	w.AddByte(opLoginSessionKey)
	w.AddString(account + "\n" + password)
	_ = c.Send(w)
}
func (p *LoginProtocol) sendCharacterList(c *network.Connection, acc *db.Account, chars []db.Character) {
	w := netmsg.NewWriter()
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
	_ = c.Send(w)
}
