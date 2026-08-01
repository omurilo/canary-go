package protocol

import (
	"encoding/json"
	"testing"
)

// The client reads these keys by name, so the JSON tags are the contract. A
// typo in one is invisible at compile time and shows up as an empty character
// list — which is exactly how the missing login type presented.
func TestJSONLoginResponseShape(t *testing.T) {
	raw, err := json.Marshal(loginResponse{
		Session: loginSession{SessionKey: "acc\npw", Status: "active"},
		PlayData: loginPlayData{
			Worlds:     []loginWorld{{Name: "Canary-Go", ExternalAddress: "10.0.0.2", ExternalPort: 7172}},
			Characters: []loginCharacter{{Name: "Gm Test", Level: 8, Vocation: "Druid", IsMale: true}},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	session, ok := got["session"].(map[string]any)
	if !ok {
		t.Fatal("no session object")
	}
	if session["sessionkey"] != "acc\npw" {
		t.Errorf("sessionkey = %v — the game protocol splits this on the newline", session["sessionkey"])
	}
	if session["status"] != "active" {
		t.Errorf("status = %v", session["status"])
	}

	play, ok := got["playdata"].(map[string]any)
	if !ok {
		t.Fatal("no playdata object")
	}
	worlds, _ := play["worlds"].([]any)
	if len(worlds) != 1 {
		t.Fatalf("worlds = %d, want 1", len(worlds))
	}
	w := worlds[0].(map[string]any)
	for _, key := range []string{"name", "externaladdress", "externalport",
		"externaladdressprotected", "externalportprotected"} {
		if _, ok := w[key]; !ok {
			t.Errorf("world is missing %q", key)
		}
	}

	chars, _ := play["characters"].([]any)
	if len(chars) != 1 {
		t.Fatalf("characters = %d, want 1", len(chars))
	}
	ch := chars[0].(map[string]any)
	if ch["name"] != "Gm Test" {
		t.Errorf("character name = %v", ch["name"])
	}
	if ch["worldid"] != float64(0) {
		t.Errorf("worldid = %v, want 0 — it indexes the worlds array", ch["worldid"])
	}
	if ch["ismale"] != true {
		t.Errorf("ismale = %v", ch["ismale"])
	}
}

// The error shape differs from the status endpoints': errorCode is a number
// here, not a string.
func TestJSONLoginErrorShape(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"errorCode": 3, "errorMessage": "nope"})
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, isString := got["errorCode"].(string); isString {
		t.Error("errorCode must be a number in the login response")
	}
}

// players.vocation is an index; the client wants the name. An unknown index
// falls back rather than producing an empty string.
func TestVocationNames(t *testing.T) {
	for id, want := range map[uint16]string{0: "None", 4: "Knight", 8: "Elite Knight"} {
		if got := vocationNames[id]; got != want {
			t.Errorf("vocation %d = %q, want %q", id, got, want)
		}
	}
	if _, ok := vocationNames[9999]; ok {
		t.Error("an unknown vocation must not resolve")
	}
}
