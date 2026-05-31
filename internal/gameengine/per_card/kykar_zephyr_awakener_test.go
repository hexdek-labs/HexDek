package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestKykarZephyr_NoncreatureCastBlinksAnotherCreature(t *testing.T) {
	gs := newGame(t, 2)
	kykar := addPerm(gs, 0, "Kykar, Zephyr Awakener", "creature", "bird", "cleric")
	target := addPerm(gs, 0, "Mulldrifter", "creature")
	pre := len(gs.DelayedTriggers)

	kykarZephyrNoncreatureCast(gs, kykar, map[string]interface{}{"caster_seat": 0})

	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == target {
			stillOnBF = true
		}
	}
	if stillOnBF {
		t.Errorf("expected target exiled from battlefield")
	}
	if len(gs.DelayedTriggers) != pre+1 {
		t.Errorf("expected 1 delayed trigger for the next_end_step return")
	}
}

func TestKykarZephyr_NoOtherCreatureSpawnsSpiritToken(t *testing.T) {
	gs := newGame(t, 2)
	kykar := addPerm(gs, 0, "Kykar, Zephyr Awakener", "creature", "bird")
	kykarZephyrNoncreatureCast(gs, kykar, map[string]interface{}{"caster_seat": 0})
	foundSpirit := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Spirit Token" {
			foundSpirit = true
		}
	}
	if !foundSpirit {
		t.Errorf("expected Spirit Token when no other creature to blink, got %v",
			permNames(gs.Seats[0].Battlefield))
	}
}

func TestKykarZephyr_DoesNotFireOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	kykar := addPerm(gs, 0, "Kykar, Zephyr Awakener", "creature", "bird")
	kykarZephyrNoncreatureCast(gs, kykar, map[string]interface{}{"caster_seat": 1})
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Spirit Token" {
			t.Errorf("Kykar must not fire on opponent's cast")
		}
	}
}

func TestKykarZephyr_BlinkReturnsTargetUnderOwnerControl(t *testing.T) {
	gs := newGame(t, 2)
	kykar := addPerm(gs, 0, "Kykar, Zephyr Awakener", "creature", "bird")
	// Opponent's creature controlled by seat 0 (e.g. stolen). Owner=1.
	stolen := addPerm(gs, 0, "Opp Mulldrifter", "creature")
	stolen.Card.Owner = 1

	kykarZephyrNoncreatureCast(gs, kykar, map[string]interface{}{"caster_seat": 0})

	dt := gs.DelayedTriggers[len(gs.DelayedTriggers)-1]
	dt.EffectFn(gs)

	foundUnderOwner := false
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Opp Mulldrifter" {
			foundUnderOwner = true
		}
	}
	if !foundUnderOwner {
		t.Errorf("blinked card should return under owner (seat 1)")
	}
}

var _ = gameengine.GameState{}
