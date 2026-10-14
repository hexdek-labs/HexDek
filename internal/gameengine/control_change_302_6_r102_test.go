package gameengine

// r102 regressions — CR §302.6 across a control change, and the §702.10c
// half of haste that went with it. See control_change_302_6_r102.go.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r102Settled builds a creature that has been under seat `seat`'s control
// for turns: on the battlefield, untapped, NOT summoning-sick. This is the
// state the defect needed — a permanent steal of a freshly-played creature
// looks correct by accident, because it was already sick.
func r102Settled(gs *GameState, seat int, name string) *Permanent {
	p := addCreature(gs, seat, name, 3, 3)
	p.SummoningSick = false
	p.Tapped = false
	return p
}

// (1) The defect. A PERMANENT steal — Mind Control, no printed haste —
// must leave the creature summoning-sick for its new controller.
func TestControlChange_PermanentSteal_IsSummoningSick_r102(t *testing.T) {
	gs := newCombatGame(t)
	aura := addBattlefield(gs, 0, "Mind Control", 0, 0, "enchantment", "aura")
	victim := r102Settled(gs, 1, "Settled Beast")

	if !canAttackGS(gs, victim) {
		t.Fatal("precondition: the creature must be attack-capable for its OWNER")
	}

	resolveGainControl(gs, aura, &gameast.GainControl{}) // no duration → while-source

	if victim.Controller != 0 {
		t.Fatalf("precondition: Mind Control should move control to seat 0, got %d", victim.Controller)
	}
	if !victim.SummoningSick {
		t.Error("CR 302.6: a creature that just changed hands has not been under its " +
			"new controller's control since their most recent turn began")
	}
	if canAttackGS(gs, victim) {
		t.Error("CR 302.6: a permanently stolen creature must not attack the turn it changes hands")
	}
}

// (2) The revert direction. Control coming HOME is a control change too:
// the owner's control was interrupted, so it was not continuous.
func TestControlChange_RevertToOwner_IsSummoningSick_r102(t *testing.T) {
	gs := newCombatGame(t)
	src := addBattlefield(gs, 0, "Act of Treason", 0, 0, "enchantment")
	victim := r102Settled(gs, 1, "Borrowed Beast")

	resolveGainControl(gs, src, &gameast.GainControl{Duration: "until_end_of_turn"})
	if victim.Controller != 0 || victim.SummoningSick {
		t.Fatalf("precondition: the Threaten-class steal should hand over a hasty creature "+
			"(controller=%d sick=%v)", victim.Controller, victim.SummoningSick)
	}

	ExpireTempControlGrants(gs) // the §514.2 cleanup-step revert

	if victim.Controller != 1 {
		t.Fatalf("precondition: control must revert to the owner, got %d", victim.Controller)
	}
	if !victim.SummoningSick {
		t.Error("CR 302.6: the owner did NOT control this continuously since their most " +
			"recent turn began — it spent the turn under the thief's control")
	}
}

// (3) NEGATIVE GUARD. The Threaten class prints "untap it; it gains haste",
// and that must survive the new stamp. The clear is ordered after the
// reparent precisely so the stamp cannot swallow it.
func TestControlChange_UntilEOTSteal_StillHasty_r102(t *testing.T) {
	gs := newCombatGame(t)
	src := addBattlefield(gs, 0, "Act of Treason", 0, 0, "enchantment")
	victim := addCreature(gs, 1, "Stolen Beast", 3, 3)
	victim.Tapped = true
	victim.SummoningSick = true

	resolveGainControl(gs, src, &gameast.GainControl{Duration: "until_end_of_turn"})

	if victim.Tapped {
		t.Error("an until-EOT steal must untap the creature")
	}
	if victim.SummoningSick {
		t.Error("an until-EOT steal grants functional haste — the §302.6 stamp must be cleared after it")
	}
	if !canAttackGS(gs, victim) {
		t.Error("the stolen creature should be able to attack this turn")
	}
}

// (4) The §702.10c arm of haste. §702.10b (attacking) always had its haste
// check; the {T}-ability gate did not, so a creature that entered with
// haste could attack but could not use its own tap ability.
func TestHaste_TapAbilityAllowedWhileSick_r102(t *testing.T) {
	ast := &gameast.CardAST{
		Name: "Hasty Tapper",
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost:   gameast.Cost{Tap: true},
				Effect: &gameast.AddMana{Pool: []gameast.ManaSymbol{{Color: []string{"C"}}}},
			},
		},
	}

	// With haste: §702.10c says the tap ability is legal.
	gs := newFixtureGame(t)
	p := addBattlefieldWithAST(gs, 0, "Hasty Tapper", 2, 2, ast, "creature")
	p.SummoningSick = true
	SetKeywordFlags(gs, p, "haste")
	if err := ActivateAbility(gs, 0, p, 0, nil); err != nil {
		t.Errorf("CR 702.10c: haste must permit a {T} ability on a summoning-sick creature: %v", err)
	}

	// Without haste: §302.6 still refuses it.
	gs2 := newFixtureGame(t)
	q := addBattlefieldWithAST(gs2, 0, "Plain Tapper", 2, 2, ast, "creature")
	q.SummoningSick = true
	if err := ActivateAbility(gs2, 0, q, 0, nil); err == nil {
		t.Error("CR 302.6: a summoning-sick creature WITHOUT haste must not use a {T} ability")
	}
}

// (5) The exchange path. Both halves of Gilded Drake / Volatile Stormdrake
// class swaps are control changes.
func TestExchangeControl_BothSidesSummoningSick_r102(t *testing.T) {
	gs := newCombatGame(t)
	mine := r102Settled(gs, 0, "My Settled Creature")
	theirs := r102Settled(gs, 1, "Their Settled Creature")

	ExchangeControl(gs, mine, theirs)

	if mine.Controller != 1 || theirs.Controller != 0 {
		t.Fatalf("precondition: the exchange should swap controllers (got %d / %d)",
			mine.Controller, theirs.Controller)
	}
	if !mine.SummoningSick || !theirs.SummoningSick {
		t.Errorf("CR 302.6: both sides of an exchange changed hands (sick: %v / %v)",
			mine.SummoningSick, theirs.SummoningSick)
	}
}
