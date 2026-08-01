package protocol

import (
	"fmt"
	"os"
	"strings"
)

// Temporary diagnostic for the item-duplication reports: a player moved an item on
// the floor and unusable copies stayed behind, and one purchase arrived as two
// boxes. Both are consistent with the client being told to ADD something it is
// never told to REMOVE, which this makes visible.
//
// Off unless CANARY_DUMP_ITEM_OPCODES is set, so it costs nothing in normal runs:
//
//	CANARY_DUMP_ITEM_OPCODES=1 ./bin/canary -config config.lua -logLevel debug
//
// Set it to "all" to log every outbound opcode instead of only the item ones.

var dumpItemOpcodes = os.Getenv("CANARY_DUMP_ITEM_OPCODES")

// itemOpcodeNames are the outbound opcodes that add, move or remove a thing. If the
// server duplicates, the same tile or container will show an add with no matching
// remove; if the server is right and the client is drawing twice, the same add will
// appear twice.
var itemOpcodeNames = map[byte]string{
	0x69: "UpdateTile",
	0x6A: "TileAddThing",
	0x6B: "TileTransform",
	0x6C: "TileRemoveThing",
	0x6D: "CreatureMove",
	0x6E: "ContainerOpen",
	0x6F: "ContainerClose",
	0x70: "ContainerAddItem",
	0x71: "ContainerUpdateItem",
	0x72: "ContainerRemoveItem",
	0x78: "InventoryItem",
	0x79: "InventoryEmpty",
}

// logOutboundPacket records one packet when the dump is enabled.
func (g *GameProtocol) logOutboundPacket(b []byte) {
	if dumpItemOpcodes == "" || len(b) == 0 || g.deps == nil || g.deps.Log == nil {
		return
	}
	op := b[0]
	name, interesting := itemOpcodeNames[op]
	if !interesting {
		if !strings.EqualFold(dumpItemOpcodes, "all") {
			return
		}
		name = "other"
	}

	player := ""
	if g.player != nil {
		player = g.player.Name
	}
	fields := []any{
		"op", fmt.Sprintf("0x%02X", op), "name", name, "len", len(b), "player", player,
	}
	// The tile opcodes carry a position and often a stackpos right after the opcode;
	// decoding them here is what makes a missing remove obvious in the log.
	switch op {
	case 0x69, 0x6A, 0x6B, 0x6C, 0x6D:
		if len(b) >= 6 {
			x := uint16(b[1]) | uint16(b[2])<<8
			y := uint16(b[3]) | uint16(b[4])<<8
			z := b[5]
			fields = append(fields, "pos", fmt.Sprintf("%d,%d,%d", x, y, z))
			if op == 0x6A || op == 0x6C || op == 0x6B {
				if len(b) >= 7 {
					fields = append(fields, "stackpos", b[6])
				}
			}
			if op == 0x6D && len(b) >= 12 {
				nx := uint16(b[7]) | uint16(b[8])<<8
				ny := uint16(b[9]) | uint16(b[10])<<8
				fields = append(fields, "stackpos", b[6], "to", fmt.Sprintf("%d,%d,%d", nx, ny, b[11]))
			}
		}
	case 0x70, 0x71, 0x72:
		if len(b) >= 3 {
			fields = append(fields, "cid", b[1], "slot", b[2])
		}
	case 0x78, 0x79:
		if len(b) >= 2 {
			fields = append(fields, "slot", b[1])
		}
	}

	head := b
	if len(head) > 24 {
		head = head[:24]
	}
	fields = append(fields, "hex", fmt.Sprintf("% X", head))
	g.deps.Log.Info("SEND", fields...)
}

// logInboundOpcode records what the client asked for. The outbound half alone was
// not enough: a player clicked "unwrap" in the context menu and nothing happened,
// and with only the server's side of the wire there was no way to tell whether the
// request never arrived or arrived and was dropped.
//
// Every inbound opcode is logged, not a filtered set — the whole question is which
// one the client actually sends for an action, and filtering assumes the answer.
func (g *GameProtocol) logInboundOpcode(op byte, rest []byte) {
	if dumpItemOpcodes == "" || g.deps == nil || g.deps.Log == nil {
		return
	}
	player := ""
	if g.player != nil {
		player = g.player.Name
	}
	head := rest
	if len(head) > 16 {
		head = head[:16]
	}
	g.deps.Log.Info("RECV", "op", fmt.Sprintf("0x%02X", op), "player", player,
		"payloadLen", len(rest), "hex", fmt.Sprintf("% X", head))
}
