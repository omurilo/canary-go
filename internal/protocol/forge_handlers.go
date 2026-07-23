package protocol

import (
	"github.com/opentibiabr/canary-go/internal/game"
	"github.com/opentibiabr/canary-go/internal/netmsg"
)

// SendForgeResourceUpdate sends packet 0x89 with current dust, slivers, cores, and limit to player.
func SendForgeResourceUpdate(p *game.Player) {
	if p == nil || p.Session == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0x89)
	w.AddU64(uint64(p.GetForgeDust()))
	w.AddU64(uint64(p.GetForgeDustLimit()))
	w.AddU64(uint64(p.GetForgeSlivers()))
	w.AddU64(uint64(p.GetForgeCores()))
	p.Session.SendToClient(w)
}

// SendForgeResult sends packet 0x8A with forge operation results.
func SendForgeResult(
	p *game.Player,
	actionType uint8,
	firstItemID uint16,
	tier uint8,
	secondItemID uint16,
	success bool,
	bonus uint8,
	convergence bool,
) {
	if p == nil || p.Session == nil {
		return
	}

	w := netmsg.NewWriter()
	w.AddByte(0x8A)
	w.AddByte(actionType)
	if convergence {
		w.AddByte(1)
	} else {
		w.AddByte(0)
	}
	if success {
		w.AddByte(1)
	} else {
		w.AddByte(0)
	}
	w.AddU16(firstItemID)
	w.AddByte(tier)
	w.AddU16(secondItemID)
	w.AddByte(tier)

	if actionType == game.ForgeActionTransfer {
		w.AddByte(0x00) // bonus none for transfer
	} else {
		w.AddByte(bonus)
		if bonus == 1 {
			w.AddByte(1) // cores kept count
		}
	}

	p.Session.SendToClient(w)
	SendForgeResourceUpdate(p)
}

// parseForgeEnter parses client packet 0xBF.
func (g *GameProtocol) parseForgeEnter(r *netmsg.Reader) {
	if g.player == nil {
		return
	}

	actionType := r.GetByte()

	var convergence bool
	var firstItem uint16
	var tier uint8
	var secondItem uint16
	var usedCore bool
	var reduceTierLoss bool

	if actionType == game.ForgeActionFusion || actionType == game.ForgeActionTransfer {
		convergence = r.GetByte() == 1
		firstItem = r.GetU16()
		tier = r.GetByte()
		secondItem = r.GetU16()
	}

	if actionType == game.ForgeActionFusion {
		if !convergence {
			usedCore = r.GetByte() == 1
			reduceTierLoss = r.GetByte() == 1
		}
		success, bonus := game.GlobalForge.FuseItems(
			g.player,
			firstItem,
			tier,
			secondItem,
			usedCore,
			reduceTierLoss,
			convergence,
		)
		SendForgeResult(g.player, actionType, firstItem, tier, secondItem, success, bonus, convergence)

	} else if actionType == game.ForgeActionTransfer {
		success := game.GlobalForge.TransferTier(g.player, firstItem, tier, secondItem, convergence)
		SendForgeResult(g.player, actionType, firstItem, tier, secondItem, success, 0, convergence)

	} else if actionType <= game.ForgeActionIncreaseLimit {
		game.GlobalForge.ResourceConversion(g.player, actionType)
		SendForgeResourceUpdate(g.player)
	}
}

// parseForgeBrowseHistory parses client packet 0xC0.
func (g *GameProtocol) parseForgeBrowseHistory(r *netmsg.Reader) {
	if g.player == nil {
		return
	}
	// Stub forge history browse
}
