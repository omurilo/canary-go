package protocol

import (
	"testing"

	"github.com/opentibiabr/canary-go/internal/game"
)

func TestStaffCommandPermissions(t *testing.T) {
	normalPlayer := &game.Player{
		Name:        "NormalUser",
		AccountType: 1,
		GroupID:     1,
	}

	godPlayer := &game.Player{
		Name:        "GodUser",
		AccountType: 5,
		GroupID:     6,
	}

	// Normal player must fail permission check for staff groups
	if hasTalkActionPermission(normalPlayer, "gamemaster") {
		t.Error("normal player should not have gamemaster permission")
	}
	if hasTalkActionPermission(normalPlayer, "god") {
		t.Error("normal player should not have god permission")
	}
	if hasTalkActionPermission(normalPlayer, "tutor") {
		t.Error("normal player should not have tutor permission")
	}

	// Normal player has permission for normal group
	if !hasTalkActionPermission(normalPlayer, "normal") {
		t.Error("normal player should have normal permission")
	}

	// God player has full permissions
	if !hasTalkActionPermission(godPlayer, "gamemaster") {
		t.Error("god player should have gamemaster permission")
	}
	if !hasTalkActionPermission(godPlayer, "god") {
		t.Error("god player should have god permission")
	}

	gpNormal := &GameProtocol{player: normalPlayer}
	if !gpNormal.handleCommand("/i 2160 100") {
		t.Error("handleCommand should return true for slash command")
	}

	gpGod := &GameProtocol{player: godPlayer}
	if !gpGod.handleCommand("/nonexistent") {
		t.Error("handleCommand should handle slash command without error")
	}
}
