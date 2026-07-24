package bestiary

import "testing"

func TestKillStatus(t *testing.T) {
	th := Thresholds{FirstUnlock: 100, SecondUnlock: 1000, ToKill: 2500}
	cases := []struct {
		kills uint32
		want  uint8
	}{
		{0, 1}, {99, 1}, {100, 2}, {999, 2}, {1000, 3}, {2499, 3}, {2500, 4}, {9999, 4},
	}
	for _, c := range cases {
		if got := KillStatus(c.kills, th); got != c.want {
			t.Errorf("KillStatus(%d) = %d, want %d", c.kills, got, c.want)
		}
	}
	if !IsComplete(2500, th) || IsComplete(2499, th) {
		t.Error("IsComplete boundary wrong")
	}
}

func TestCrossings(t *testing.T) {
	th := Thresholds{FirstUnlock: 100, SecondUnlock: 1000, ToKill: 2500}
	if !CrossedStage(0, 1, th) {
		t.Error("initial kill should be a stage crossing")
	}
	if !CrossedStage(99, 1, th) {
		t.Error("crossing FirstUnlock should be a stage crossing")
	}
	if CrossedStage(101, 1, th) {
		t.Error("no boundary between 101 and 102")
	}
	if !CrossedCompletion(2499, 1, th) {
		t.Error("2499+1 should complete")
	}
	if CrossedCompletion(2500, 1, th) {
		t.Error("already complete, not a new completion")
	}
}
