package game

import (
	"math/rand"
	"time"
)

// ForgeAction types matching client protocol (0xBF)
const (
	ForgeActionFusion        uint8 = 0
	ForgeActionTransfer      uint8 = 1
	ForgeActionDustToSliver  uint8 = 2
	ForgeActionSliverToCore  uint8 = 3
	ForgeActionIncreaseLimit uint8 = 4
)

type ForgeEngine struct{}

var GlobalForge = &ForgeEngine{}

// FuseItems executes item fusion (actionType = 0).
func (fe *ForgeEngine) FuseItems(
	player *Player,
	firstItemID uint16,
	tier uint8,
	secondItemID uint16,
	usedCore bool,
	reduceTierLoss bool,
	convergence bool,
) (bool, uint8) {
	if player == nil {
		return false, 0
	}

	// Costs
	dustCost := uint32(100)
	if convergence {
		dustCost = 130
	}

	if !player.RemoveForgeDust(dustCost) {
		player.SendTextMessage(20, "You do not have enough dust.")
		return false, 0
	}

	coreCount := uint32(0)
	if usedCore {
		coreCount++
	}
	if reduceTierLoss {
		coreCount++
	}

	if coreCount > 0 && !player.RemoveForgeCores(coreCount) {
		player.AddForgeDust(dustCost) // refund
		player.SendTextMessage(20, "You do not have enough Exaltation Cores.")
		return false, 0
	}

	// Success chance calculation
	baseRate := uint32(50)
	if usedCore {
		baseRate += 15
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	roll := uint32(r.Intn(100) + 1)
	success := roll <= baseRate || convergence

	// Bonus roll (0 = none, 1 = core kept, etc)
	bonus := uint8(0)
	if !convergence && r.Intn(100) < 10 {
		bonus = 1
		if usedCore {
			player.AddForgeCores(1)
		}
	}

	return success, bonus
}

// TransferTier executes tier transfer (actionType = 1).
func (fe *ForgeEngine) TransferTier(
	player *Player,
	donorItemID uint16,
	tier uint8,
	targetItemID uint16,
	convergence bool,
) bool {
	if player == nil {
		return false
	}

	dustCost := uint32(100)
	if convergence {
		dustCost = 160
	}

	if !player.RemoveForgeDust(dustCost) {
		player.SendTextMessage(20, "You do not have enough dust.")
		return false
	}

	return true
}

// ResourceConversion handles Dust->Slivers, Slivers->Core, Increase Limit (actionType 2..4).
func (fe *ForgeEngine) ResourceConversion(player *Player, actionType uint8) bool {
	if player == nil {
		return false
	}

	switch actionType {
	case ForgeActionDustToSliver: // 20 Dust -> 1 Sliver
		if player.RemoveForgeDust(20) {
			player.AddForgeSlivers(1)
			return true
		}
		player.SendTextMessage(20, "You need 20 dust to convert to 1 sliver.")

	case ForgeActionSliverToCore: // 3 Slivers -> 1 Core
		if player.RemoveForgeSlivers(3) {
			player.AddForgeCores(1)
			return true
		}
		player.SendTextMessage(20, "You need 3 slivers to convert to 1 core.")

	case ForgeActionIncreaseLimit: // 50 Dust -> +3 Dust Limit (max 225)
		if player.GetForgeDustLimit() >= 225 {
			player.SendTextMessage(20, "You have reached the maximum dust limit.")
			return false
		}
		if player.RemoveForgeDust(50) {
			player.ForgeDustLimit += 3
			if player.ForgeDustLimit > 225 {
				player.ForgeDustLimit = 225
			}
			return true
		}
		player.SendTextMessage(20, "You need 50 dust to increase your dust limit.")
	}

	return false
}
