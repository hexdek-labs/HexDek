package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Loki r60 extreme-stress / seed 1337 game 8921 turn 41 cleanup
// surfaced 40 CardIdentity hits in a single game. The pod was:
// Magda, Brazen Outlaw / Shiko, Paragon of the Way / Spider-Gwen,
// Free Spirit / Zidane, Tantalus Thief. 40 hits = one deep persistent
// leak that the invariant kept re-detecting every turn for ~8-10
// turns.
//
// Bisection through the three handlers identified Zidane as the
// source: its ETB control-steal registers a `next_end_step` delayed
// trigger to return the stolen creature. The delayed trigger blindly
// re-added the captured *Permanent to the original owner's
// battlefield, with no check for whether the perm had LEFT play in
// the meantime (died → graveyard, exiled, bounced to hand).
//
// CR §611.3c: when a control-changing effect ends, control reverts
// to the owner ONLY IF the permanent is still on the battlefield.
// If it's left play, the control-end is a no-op — there is no
// battlefield object to return.
//
// Pre-fix consequence: the dead creature gets re-added to original
// owner's battlefield as a Permanent, but the underlying *Card is
// already in graveyard / exile / hand. Same *Card pointer in two
// zones → CardIdentity violation. Same defensive-check pattern as
// Adric / Oketra / Gisa / Athreos / The Reaper §704.6d.

// TestZidane_EOTReturn_NoOpWhenCapturedDied pins the bit-stable
// signature: Zidane steals a creature, the creature dies (its
// *Card moves to graveyard) BEFORE the end-step delayed trigger
// fires. The trigger must NOT re-add the *Permanent to any
// battlefield — otherwise the *Card lives in both graveyard and
// battlefield.
func TestZidane_EOTReturn_NoOpWhenCapturedDied(t *testing.T) {
	gs := newGame(t, 2)

	// Zidane on seat 0; Dragon on seat 1 (the steal target).
	zidane := addPerm(gs, 0, "Zidane, Tantalus Thief", "creature", "legendary")
	dragon := addPerm(gs, 1, "Dragon", "creature")
	dragon.Card.BasePower = 5
	dragon.Card.BaseToughness = 5
	dragonCard := dragon.Card

	zidaneTantalusThiefETB(gs, zidane)

	// Confirm steal: Dragon's *Permanent is now on seat 0's battlefield.
	stillOnZero := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == dragon {
			stillOnZero = true
			break
		}
	}
	if !stillOnZero {
		t.Fatal("precondition: Zidane should have stolen the Dragon onto seat 0")
	}

	// Dragon dies before end of turn — remove the *Permanent from seat 0
	// and route the *Card to its original owner's graveyard.
	for i, p := range gs.Seats[0].Battlefield {
		if p == dragon {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield[:i], gs.Seats[0].Battlefield[i+1:]...)
			break
		}
	}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, dragonCard)

	// Find the registered delayed trigger and fire it directly. (Calling
	// the EffectFn closure mirrors what FireDelayedTriggers does at
	// end_of_turn step; we avoid driving the full turn pipeline so the
	// test stays focused.)
	var trigger *gameengine.DelayedTrigger
	for _, dt := range gs.DelayedTriggers {
		if dt != nil && dt.TriggerAt == "next_end_step" && dt.SourceCardName == "Zidane, Tantalus Thief" {
			trigger = dt
			break
		}
	}
	if trigger == nil {
		t.Fatal("expected Zidane to have registered a next_end_step delayed trigger")
	}
	trigger.EffectFn(gs)

	// Pin the invariant: Dragon's *Card lives in seat 1's graveyard
	// ONLY. No Permanent on any battlefield wraps it.
	wrapsOnBattlefield := 0
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card == dragonCard {
				wrapsOnBattlefield++
				t.Errorf("Dragon *Card should not be on any battlefield post-EOT-return (perm has left play); found on seat %d", seatIdx)
			}
		}
	}
	if wrapsOnBattlefield != 0 {
		t.Fatalf("REGRESSION: Dragon *Permanent re-added to battlefield despite leaving play; pre-fix shape (%d wraps)", wrapsOnBattlefield)
	}

	// Confirm *Card is in seat 1's graveyard exactly once.
	gyOccurrences := 0
	for _, c := range gs.Seats[1].Graveyard {
		if c == dragonCard {
			gyOccurrences++
		}
	}
	if gyOccurrences != 1 {
		t.Fatalf("expected Dragon *Card in seat 1 graveyard exactly once; got %d", gyOccurrences)
	}

	// Lifelink/haste flags should have been cleaned up so they don't
	// ride along with a future reanimation that rewraps the same *Card.
	if dragon.Flags["kw:lifelink"] != 0 {
		t.Errorf("kw:lifelink should be cleared post-EOT-return; got %d", dragon.Flags["kw:lifelink"])
	}
	if dragon.Flags["kw:haste"] != 0 {
		t.Errorf("kw:haste should be cleared post-EOT-return; got %d", dragon.Flags["kw:haste"])
	}
}

// TestZidane_EOTReturn_NormalPathStillReturnsLiveCreature pins the
// legitimate path: the captured perm is still on the battlefield at
// end of turn. The trigger must successfully return it to the
// original owner — defends against narrowing the gate too far.
func TestZidane_EOTReturn_NormalPathStillReturnsLiveCreature(t *testing.T) {
	gs := newGame(t, 2)

	zidane := addPerm(gs, 0, "Zidane, Tantalus Thief", "creature", "legendary")
	dragon := addPerm(gs, 1, "Dragon", "creature")
	dragon.Card.BasePower = 5
	dragon.Card.BaseToughness = 5

	zidaneTantalusThiefETB(gs, zidane)

	// Dragon is alive on seat 0's battlefield at end of turn.
	var trigger *gameengine.DelayedTrigger
	for _, dt := range gs.DelayedTriggers {
		if dt != nil && dt.TriggerAt == "next_end_step" && dt.SourceCardName == "Zidane, Tantalus Thief" {
			trigger = dt
			break
		}
	}
	if trigger == nil {
		t.Fatal("expected a next_end_step delayed trigger from Zidane")
	}
	trigger.EffectFn(gs)

	// Dragon's *Permanent should be back on seat 1's battlefield.
	on1 := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == dragon {
			on1 = true
			break
		}
	}
	if !on1 {
		t.Fatal("EOT return must restore the live creature to original owner's battlefield")
	}
	on0 := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == dragon {
			on0 = true
			break
		}
	}
	if on0 {
		t.Fatal("EOT return must remove the perm from Zidane's battlefield")
	}
	if dragon.Controller != 1 {
		t.Errorf("Dragon's Controller should be original owner (1); got %d", dragon.Controller)
	}
	if dragon.Flags["kw:lifelink"] != 0 || dragon.Flags["kw:haste"] != 0 {
		t.Errorf("lifelink/haste should be cleared on EOT return; flags=%v", dragon.Flags)
	}
}

// TestZidane_EOTReturn_NoOpWhenCapturedExiled covers the exile
// variant: someone exiled the stolen Dragon during the turn. The
// EOT return must no-op — same shape as the death case but the
// *Card lives in exile instead of graveyard.
func TestZidane_EOTReturn_NoOpWhenCapturedExiled(t *testing.T) {
	gs := newGame(t, 2)

	zidane := addPerm(gs, 0, "Zidane, Tantalus Thief", "creature", "legendary")
	dragon := addPerm(gs, 1, "Dragon", "creature")
	dragon.Card.BasePower = 5
	dragon.Card.BaseToughness = 5
	dragonCard := dragon.Card

	zidaneTantalusThiefETB(gs, zidane)

	// Dragon is exiled mid-turn.
	for i, p := range gs.Seats[0].Battlefield {
		if p == dragon {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield[:i], gs.Seats[0].Battlefield[i+1:]...)
			break
		}
	}
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, dragonCard)

	var trigger *gameengine.DelayedTrigger
	for _, dt := range gs.DelayedTriggers {
		if dt != nil && dt.TriggerAt == "next_end_step" && dt.SourceCardName == "Zidane, Tantalus Thief" {
			trigger = dt
			break
		}
	}
	if trigger == nil {
		t.Fatal("expected a next_end_step delayed trigger")
	}
	trigger.EffectFn(gs)

	// *Card should be in exile only — not also on any battlefield.
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card == dragonCard {
				t.Errorf("REGRESSION: exiled Dragon *Card found on seat %d battlefield post-EOT", seatIdx)
			}
		}
	}
	exileCount := 0
	for _, c := range gs.Seats[1].Exile {
		if c == dragonCard {
			exileCount++
		}
	}
	if exileCount != 1 {
		t.Fatalf("Dragon *Card should be in exile exactly once; got %d", exileCount)
	}
}
