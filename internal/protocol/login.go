package protocol

import (
	"context"
	"fmt"
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

func (p *LoginProtocol) OnConnect(c *network.Connection)                  {}
func (p *LoginProtocol) OnDisconnect(c *network.Connection)               {}
func (p *LoginProtocol) OnPacket(c *network.Connection, r *netmsg.Reader) {}

// OnFirstPacket parses the login request, authenticates and replies.
func (p *LoginProtocol) OnFirstPacket(c *network.Connection, body []byte) {
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
