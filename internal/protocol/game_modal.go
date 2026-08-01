package protocol

import (
	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/netmsg"
)

// sendModalWindow sends a modal dialog to the client (opcode 0x7D).
func (g *GameProtocol) sendModalWindow(modal *game.ModalWindow) {
	if g.player == nil {
		return
	}
	g.player.AddModalWindow(modal.ID)

	w := netmsg.NewWriter()
	w.AddByte(0x7D)
	w.AddU32(modal.ID)
	w.AddString(modal.Title)
	w.AddString(modal.Message)

	// Buttons
	w.AddByte(uint8(len(modal.Buttons)))
	for _, btn := range modal.Buttons {
		w.AddString(btn.Text)
		w.AddByte(btn.ID)
	}

	// Choices
	w.AddByte(uint8(len(modal.Choices)))
	for _, ch := range modal.Choices {
		w.AddString(ch.Text)
		w.AddByte(ch.ID)
	}

	w.AddByte(modal.DefaultEscapeButton)
	w.AddByte(modal.DefaultEnterButton)
	if modal.Priority {
		w.AddByte(0x01)
	} else {
		w.AddByte(0x00)
	}

	g.player.Session.SendToClient(w)
}

// parseModalWindowAnswer handles the client's modal window response (opcode 0x7E).
func (g *GameProtocol) parseModalWindowAnswer(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	id := r.GetU32()
	button := r.GetByte()
	choice := r.GetByte()

	// Forward to the world which validates the window, removes it, and fires
	// the OnModalWindowAnswer hook (set in main.go to invoke Lua).
	g.deps.World.PlayerAnswerModalWindow(g.player, id, button, choice)
}

// SendModalWindow is the Session-accessible entry point used by Lua scripts to
// send a modal window to a player. It delegates to the unexported sendModalWindow.
func (g *GameProtocol) SendModalWindow(modal *game.ModalWindow) {
	g.sendModalWindow(modal)
}
