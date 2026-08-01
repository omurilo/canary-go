package protocol

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
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
	if hasGroupPermission(normalPlayer, "gamemaster") {
		t.Error("normal player should not have gamemaster permission")
	}
	if hasGroupPermission(normalPlayer, "god") {
		t.Error("normal player should not have god permission")
	}
	if hasGroupPermission(normalPlayer, "tutor") {
		t.Error("normal player should not have tutor permission")
	}

	// Normal player has permission for normal group
	if !hasGroupPermission(normalPlayer, "normal") {
		t.Error("normal player should have normal permission")
	}

	// God player has full permissions
	if !hasGroupPermission(godPlayer, "gamemaster") {
		t.Error("god player should have gamemaster permission")
	}
	if !hasGroupPermission(godPlayer, "god") {
		t.Error("god player should have god permission")
	}

	gpNormal := &GameProtocol{player: normalPlayer}
	if !gpNormal.handleCommand("/i 2160 100") {
		t.Error("handleCommand should return true for slash command")
	}

	// An UNKNOWN slash command must report false. handleCommand's contract is
	// "did I consume this?", and its caller (game_actions.go:548) falls through to
	// spells, then Lua talkactions, then chat when it does not — the same order as
	// Game::playerSay. Reporting true here would swallow every custom talkaction.
	gpGod := &GameProtocol{player: godPlayer}
	if gpGod.handleCommand("/nonexistent") {
		t.Error("an unknown slash command must fall through so talkactions get a chance")
	}
	// A known one is consumed.
	if !gpGod.handleCommand("/pos") {
		t.Error("handleCommand should consume a known command")
	}
}
