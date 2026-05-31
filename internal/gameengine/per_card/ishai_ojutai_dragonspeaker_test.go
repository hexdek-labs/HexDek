package per_card

import (
	"testing"
)

func TestIshai_PlusOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	ishai := addPerm(gs, 0, "Ishai, Ojutai Dragonspeaker", "creature", "bird")
	pre := ishai.Counters["+1/+1"]
	ishaiOppCast(gs, ishai, map[string]interface{}{"caster_seat": 1})
	if ishai.Counters["+1/+1"] != pre+1 {
		t.Errorf("expected Ishai +1 counter on opponent cast, got %d → %d", pre, ishai.Counters["+1/+1"])
	}
}

func TestIshai_NoFireOnOwnCast(t *testing.T) {
	gs := newGame(t, 2)
	ishai := addPerm(gs, 0, "Ishai, Ojutai Dragonspeaker", "creature", "bird")
	pre := ishai.Counters["+1/+1"]
	ishaiOppCast(gs, ishai, map[string]interface{}{"caster_seat": 0})
	if ishai.Counters["+1/+1"] != pre {
		t.Errorf("Ishai must not pump on her controller's own casts")
	}
}

func TestIshai_StacksAcrossMultipleOppCasts(t *testing.T) {
	gs := newGame(t, 2)
	ishai := addPerm(gs, 0, "Ishai, Ojutai Dragonspeaker", "creature", "bird")
	for i := 0; i < 5; i++ {
		ishaiOppCast(gs, ishai, map[string]interface{}{"caster_seat": 1})
	}
	if ishai.Counters["+1/+1"] != 5 {
		t.Errorf("expected 5 counters after 5 opp casts, got %d", ishai.Counters["+1/+1"])
	}
}
