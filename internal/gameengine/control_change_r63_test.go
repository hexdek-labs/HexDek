package gameengine

// r63 — layer-2 control-change probe (CR §613.1b / §108.3). Three bugs:
//   (b) GainControl{until_end_of_turn} stole but never untapped / cleared
//       summoning sickness, so a Threaten-class steal couldn't attack.
//   (c) a permanent control source (Control Magic / Mind Control / Sower)
//       leaving the battlefield never reverted control to the owner.
//   (d) control change never invalidated the characteristics cache, so
//       anthems / "creatures you control" statics went stale.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// (b) An until-end-of-turn steal untaps + clears summoning sickness (functional
// haste) so the stolen creature can attack, and reverts to its owner at EOT.
func TestControl_UntilEOT_UntapsHastesRevertsAtEOT(t *testing.T) {
	gs := newCombatGame(t)
	src := addBattlefield(gs, 0, "Act of Treason", 0, 0, "enchantment")
	victim := addCreature(gs, 1, "Stolen Beast", 3, 3)
	victim.Tapped = true
	victim.SummoningSick = true

	resolveGainControl(gs, src, &gameast.GainControl{Duration: "until_end_of_turn"})

	if victim.Controller != 0 {
		t.Fatalf("control should move to seat 0, got %d", victim.Controller)
	}
	if victim.Tapped {
		t.Errorf("an until-EOT steal must untap the creature")
	}
	if victim.SummoningSick {
		t.Errorf("an until-EOT steal must clear summoning sickness (functional haste)")
	}
	if !canAttackGS(gs, victim) {
		t.Errorf("the stolen creature should be able to attack this turn")
	}

	ExpireTempControlGrants(gs)
	if victim.Controller != 1 {
		t.Errorf("an until-EOT steal must revert to the owner (seat 1) at cleanup, got %d", victim.Controller)
	}
}

// (c) A permanent (Control Magic) steal survives end of turn but reverts to the
// owner the moment its source leaves the battlefield.
func TestControl_PermanentSteal_RevertsWhenSourceLeaves(t *testing.T) {
	gs := newCombatGame(t)
	aura := addBattlefield(gs, 0, "Control Magic", 0, 0, "enchantment", "aura")
	victim := addCreature(gs, 1, "Stolen Creature", 2, 2)

	resolveGainControl(gs, aura, &gameast.GainControl{}) // no duration → while-source

	if victim.Controller != 0 {
		t.Fatalf("Control Magic should move control to seat 0, got %d", victim.Controller)
	}

	// End of turn must NOT revert a while-source-on-battlefield steal.
	ExpireTempControlGrants(gs)
	if victim.Controller != 0 {
		t.Errorf("a Control Magic steal must NOT revert at end of turn")
	}

	// Source leaves the battlefield → control returns to the OWNER.
	ExpireSourceGrants(gs, aura.Timestamp)
	if victim.Controller != 1 {
		t.Errorf("control must revert to owner (seat 1) when Control Magic leaves, got %d", victim.Controller)
	}
}

// (d) A "creatures you control get +1/+1" anthem recomputes the instant control
// changes — the stolen creature gains the new controller's anthem.
func TestControl_AnthemRecomputesOnSteal(t *testing.T) {
	gs := newCombatGame(t)
	anthem := addBattlefield(gs, 0, "Glorious Anthem", 0, 0, "enchantment")
	registerAnthemPT(gs, anthem, 1, 1, "test_control_anthem", func(_ *GameState, tt *Permanent) bool {
		return tt.Controller == anthem.Controller && tt.IsCreature()
	})
	victim := addCreature(gs, 1, "Plain Bear", 2, 2)

	// Populate the cache while the creature is the opponent's: 2/2.
	if got := GetEffectiveCharacteristics(gs, victim); got.Power != 2 || got.Toughness != 2 {
		t.Fatalf("before steal: want 2/2, got %d/%d", got.Power, got.Toughness)
	}

	src := addBattlefield(gs, 0, "Mind Control", 0, 0, "enchantment", "aura")
	resolveGainControl(gs, src, &gameast.GainControl{})

	// Now under seat 0's anthem — must read 3/3 (cache invalidated on steal).
	if got := GetEffectiveCharacteristics(gs, victim); got.Power != 3 || got.Toughness != 3 {
		t.Errorf("after steal the anthem must recompute: want 3/3, got %d/%d", got.Power, got.Toughness)
	}
}
