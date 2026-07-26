package protocol

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/opentibiabr/canary-go/internal/netmsg"
	"github.com/opentibiabr/canary-go/internal/network"
)

// StatusProtocol handles server status requests (XML/binary), mirroring C++
// ProtocolStatus. It runs on a configurable STATUS_PORT.
type StatusProtocol struct {
	deps *Deps
}

// NewStatusFactory creates StatusProtocol instances.
func NewStatusFactory(deps *Deps) network.ProtocolFactory {
	return func() network.Protocol {
		return &StatusProtocol{deps: deps}
	}
}

func (p *StatusProtocol) OnConnect(c *network.Connection) {}
func (p *StatusProtocol) OnFirstPacket(c *network.Connection, body []byte) {
	// Status port receives a raw first packet (no XTEA).
	r := netmsg.NewReader(body)
	p.OnPacket(c, r)
}
func (p *StatusProtocol) OnDisconnect(c *network.Connection) {}

// OnPacket dispatches the status request.
func (p *StatusProtocol) OnPacket(c *network.Connection, r *netmsg.Reader) {
	if r.Remaining() < 1 {
		return
	}
	packetType := r.GetByte()
	switch packetType {
	case 0x01:
		var requestedInfo uint16
		if r.Remaining() >= 2 {
			requestedInfo = r.GetU16()
		}
		var characterName string
		if requestedInfo&0x01 != 0 && r.Remaining() > 0 {
			_ = r.GetString()
		}
		p.sendInfo(c, requestedInfo, characterName)
	default:
		p.sendStatusString(c)
	}
	c.Close()
}

// sendStatusString sends full XML server status.
func (p *StatusProtocol) sendStatusString(c *network.Connection) {
	uptime := uint64(time.Since(serverStartTime).Seconds())

	var onlineCount, maxPlayers uint32
	var serverName, serverVersion, clientVersion string

	if p.deps != nil && p.deps.World != nil {
		onlineCount = uint32(p.deps.World.OnlineCount())
		maxPlayers = 2000
		serverName = "Canary-Go"
		serverVersion = "1.0"
		clientVersion = "13.15"
	}

	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\"?>\n")
	b.WriteString("<tsqp version=\"1.0\">\n")
	fmt.Fprintf(&b, "\t<serverinfo uptime=\"%d\" ip=\"\" servername=\"%s\" port=\"%d\" location=\"\" url=\"\" server=\"%s\" version=\"%s\" client=\"%s\"/>\n",
		uptime, serverName, 7171, serverName, serverVersion, clientVersion)
	fmt.Fprintf(&b, "\t<owner name=\"\" email=\"\"/>\n")
	fmt.Fprintf(&b, "\t<players online=\"%d\" unique=\"%d\" max=\"%d\" peak=\"0\"/>\n", onlineCount, onlineCount, maxPlayers)
	fmt.Fprintf(&b, "\t<monsters total=\"0\"/>\n")
	fmt.Fprintf(&b, "\t<npcs total=\"0\"/>\n")
	fmt.Fprintf(&b, "\t<rates experience=\"1.0\" skill=\"1.0\" loot=\"1.0\" magic=\"1.0\" spawn=\"1.0\"/>\n")
	fmt.Fprintf(&b, "\t<map name=\"\" author=\"\"/>\n")
	b.WriteString("</tsqp>\n")

	result := b.String()
	
	c.WriteRaw([]byte(result))
	slog.Default().Info("status: sending XML status", "bytes", len(result))
}

func (p *StatusProtocol) sendInfo(c *network.Connection, requestedInfo uint16, characterName string) {
	w := netmsg.NewWriter()
	
	_ = w
	_ = requestedInfo
	_ = characterName
	slog.Default().Info("status: binary info request", "flags", requestedInfo)
}

var serverStartTime = time.Now()
