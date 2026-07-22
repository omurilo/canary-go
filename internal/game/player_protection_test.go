package game

import "testing"

func TestCannotBeAttacked(t *testing.T) {
	cases := []struct {
		name    string
		p       *Player
		protect bool
	}{
		{"normal player", &Player{GroupID: 1}, false},
		{"tutor", &Player{GroupID: 2}, false},
		{"senior tutor", &Player{GroupID: 3}, false},
		{"gamemaster", &Player{GroupID: 4}, true},
		{"community manager", &Player{GroupID: 5}, true},
		{"god group", &Player{GroupID: 6}, true},
		{"god account", &Player{GroupID: 1, AccountType: 5}, true},
		{"ghost", &Player{GroupID: 1, Ghost: true}, true},
	}
	for _, c := range cases {
		if got := c.p.CannotBeAttacked(); got != c.protect {
			t.Errorf("%s: CannotBeAttacked=%v want %v", c.name, got, c.protect)
		}
	}
}
