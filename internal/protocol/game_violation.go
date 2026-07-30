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

// parseRequestRuleChannels handles the client requesting the violation channel list
// (opcode 0xEC).
func (g *GameProtocol) parseRequestRuleChannels(r *netmsg.Reader) {
	g.sendRuleChannels()
}

// sendRuleChannels sends the list of rule violation channels to the client
// (opcode 0xED). Currently returns an empty list; a full implementation would
// populate the list with open violation reports that this player can review.
func (g *GameProtocol) sendRuleChannels() {
	w := netmsg.NewWriter()
	w.AddByte(0xED) // opcode
	w.AddByte(0x00) // count (none available for now)
	g.SendToClient(w)
}
