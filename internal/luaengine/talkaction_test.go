package luaengine

import (
	"testing"

	"github.com/omurilo/canary-go/internal/game"
	"github.com/omurilo/canary-go/internal/talkactions"
)

func TestTalkActionRegisterAndCall(t *testing.T) {
	e := newTestEngine()
	script := `
		local talk = TalkAction("/i")
		talk:separator(" ")
		talk:groupType("god")
		
		talkaction_called = false
		talkaction_param = ""
		
		function talk.onSay(player, words, param, type)
			talkaction_called = true
			talkaction_param = param
			return true
		end
		
		talk:register()
	`
	if err := e.DoString(script); err != nil {
		t.Fatalf("load talkaction script: %v", err)
	}
	ta, param := talkactions.FindByWords("/i sword")
	if ta == nil {
		t.Fatal("talkaction not registered under its words")
	}
	if ta.Words != "/i" {
		t.Errorf("expected words /i, got %q", ta.Words)
	}
	if param != "sword" {
		t.Errorf("expected param sword, got %q", param)
	}
	if ta.GroupType != "god" {
		t.Errorf("expected groupType god, got %q", ta.GroupType)
	}
	
	// Create player
	w := e.world
	p := &game.Player{Name: "God"}
	p.SetPosition(game.Position{X: 100, Y: 100, Z: 7})
	w.AddPlayer(p, nil)
	
	// Call
	success := e.CallTalkAction(ta, p, 1, "/i sword", param)
	if !success {
		t.Fatal("CallTalkAction returned false")
	}
	
	// Check global vars
	called := e.L.GetGlobal("talkaction_called")
	if called.String() != "true" {
		t.Errorf("talkaction onSay was not called, got: %v", called.String())
	}
	
	passedParam := e.L.GetGlobal("talkaction_param")
	if passedParam.String() != "sword" {
		t.Errorf("talkaction onSay param = %q, want sword", passedParam.String())
	}
}
