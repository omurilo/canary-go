package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseReportViolation handles a player's rule violation report.
// Format: [u8 reason][u8 action][str characterName][str comment]
//
// Intentionally NOT dispatched: the docstring used to claim 0xE6, but 0xE6 is
// parseBugReport (protocolgame.cpp:2015). Rule-violation reports are 0xF2, which
// upstream keeps commented out (`// case 0xF2: parseRuleViolationReport(msg);`).
// Kept so the logic is ready if 0xF2 is ever enabled.
func (g *GameProtocol) parseReportViolation(r *netmsg.Reader) {
	reason := r.GetByte()
	action := r.GetByte()
	characterName := r.GetString()
	comment := r.GetString()

	// EventCallback playerOnReportRuleViolation(player, targetName, reportType,
	// reportReason, comment, translation) — (void).
	if g.deps.Events != nil && g.player != nil {
		g.deps.Events.ExecutePlayerOnReportRuleViolation(
			g.player, characterName, action, reason, comment, "")
	}

	// Store the report in memory for potential review by moderators.
	if p := g.player; p != nil && g.deps != nil && g.deps.Log != nil {
		p.ViolationReports = append(p.ViolationReports, game.ReportViolationEntry{
			ReporterID: p.ID,
			Character:  characterName,
			Reason:     reason,
			Comment:    comment,
		})

		g.deps.Log.Info("rule violation report",
			"player", p.Name,
			"reason", reason,
			"action", action,
			"character", characterName,
			"comment", comment,
		)
	}
}

// parseSendResourceBalance handles 0xED, the client asking for a resource refresh.
// Ports ProtocolGame::parseSendResourceBalance (protocolgame.cpp): it is a REQUEST
// with no reply of its own — the answer is the ordinary resource balance packets.
//
// This opcode used to be routed to a "rule violation channels" handler that replied
// with 0xED and a single zero byte. Outbound 0xED is sendMessageDialog, which is
// opcode + type + STRING, so the client read the type and then looked for a string
// that was not there: "not enough bytes (2) available at position 2", and it died.
// The inbound and outbound opcode spaces are separate, and 0xED means different
// things in each.
func (g *GameProtocol) parseSendResourceBalance(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	// The modern profile prefixes a byte the handler discards.
	if r.Remaining() > 0 {
		_ = r.GetByte()
	}
	g.sendResourceBalances()
}
