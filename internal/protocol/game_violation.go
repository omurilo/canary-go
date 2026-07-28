package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// parseReportViolation handles a player's rule violation report (opcode 0xE6).
// Format: [u8 reason][u8 action][str characterName][str comment]
func (g *GameProtocol) parseReportViolation(r *netmsg.Reader) {
	reason := r.GetByte()
	action := r.GetByte()
	characterName := r.GetString()
	comment := r.GetString()

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
