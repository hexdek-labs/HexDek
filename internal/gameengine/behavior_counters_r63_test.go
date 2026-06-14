package gameengine

// r63 — behavior-counter probe.
//   Part B: Kulrath Knight — "Creatures your opponents control with counters on
//           them can't attack or block" (a §508/§509 continuous combat
//           restriction keyed on ANY counter, opponent-only).
//   Part A bug: finality counter (CR §122.1h) — a permanent with a finality
//           counter that would go to a graveyard from the battlefield is exiled
//           instead.

import "testing"

func cardInSlice(zone []*Card, c *Card) bool {
	for _, x := range zone {
		if x == c {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Part B — Kulrath Knight
// ---------------------------------------------------------------------------

// A creature an opponent controls that has a counter can't be declared as an
// attacker while that opponent has a Kulrath Knight — and CAN once the counter
// is gone (dynamic).
func TestKulrath_CounterCreatureCantAttack(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 1, "Kulrath Knight", 4, 4) // opponent (seat 1) controls it
	attacker := addCreature(gs, 0, "Counter Bear", 2, 2)
	attacker.Counters["+1/+1"] = 1

	if canAttackGS(gs, attacker) {
		t.Errorf("a counter-bearing creature must not be able to attack while an opponent has Kulrath Knight")
	}

	// Remove the counter — restriction lifts.
	delete(attacker.Counters, "+1/+1")
	if !canAttackGS(gs, attacker) {
		t.Errorf("once the counter is gone the creature should be able to attack again")
	}
}

// Any counter qualifies — a stun counter (behavior counter, no stat effect)
// also locks the creature out of combat.
func TestKulrath_StunCounterAlsoLocks(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 1, "Kulrath Knight", 4, 4)
	c := addCreature(gs, 0, "Stunned Attacker", 2, 2)
	c.Counters["stun"] = 1
	if canAttackGS(gs, c) {
		t.Errorf("a creature with a stun counter must be locked by an opponent's Kulrath Knight")
	}
}

// Blocking is restricted the same way.
func TestKulrath_CounterCreatureCantBlock(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 1, "Kulrath Knight", 4, 4)
	atkr := addCreature(gs, 1, "Opp Attacker", 3, 3)
	blocker := addCreature(gs, 0, "Counter Blocker", 2, 2)
	blocker.Counters["-1/-1"] = 1

	if canBlockGS(gs, atkr, blocker) {
		t.Errorf("a counter-bearing creature must not be able to block while an opponent has Kulrath Knight")
	}
	delete(blocker.Counters, "-1/-1")
	if !canBlockGS(gs, atkr, blocker) {
		t.Errorf("once the counter is gone the creature should be able to block again")
	}
}

// Kulrath only affects OPPONENTS' creatures — its controller's own
// counter-bearing creatures are unaffected.
func TestKulrath_OwnCreaturesUnaffected(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 1, "Kulrath Knight", 4, 4)
	ownCreature := addCreature(gs, 1, "My Counter Creature", 2, 2)
	ownCreature.Counters["+1/+1"] = 1

	if !canAttackGS(gs, ownCreature) {
		t.Errorf("Kulrath's controller's own counter-creature must still be able to attack (opponent-only static)")
	}
}

// A creature with NO counter is unaffected by Kulrath.
func TestKulrath_NoCounterUnaffected(t *testing.T) {
	gs := newCombatGame(t)
	addCreature(gs, 1, "Kulrath Knight", 4, 4)
	plain := addCreature(gs, 0, "Plain Bear", 2, 2)

	if !canAttackGS(gs, plain) {
		t.Errorf("a creature without counters must be able to attack despite an opponent's Kulrath Knight")
	}
}

// ---------------------------------------------------------------------------
// Part A — finality counter (CR §122.1h)
// ---------------------------------------------------------------------------

// A creature with a finality counter that is destroyed is EXILED, not put into
// the graveyard.
func TestFinalityCounter_DestroyedToExile(t *testing.T) {
	gs := newCombatGame(t)
	perm := addCreature(gs, 0, "Reanimated Threat", 2, 2)
	perm.Counters["finality"] = 1
	card := perm.Card

	DestroyPermanent(gs, perm, nil)

	if cardInSlice(gs.Seats[0].Graveyard, card) {
		t.Errorf("a finality-countered creature must NOT go to the graveyard")
	}
	if !cardInSlice(gs.Seats[0].Exile, card) {
		t.Errorf("a finality-countered creature must be exiled instead (CR §122.1h)")
	}
}

// Control: a creature WITHOUT a finality counter goes to the graveyard.
func TestFinalityCounter_NoCounterToGraveyard(t *testing.T) {
	gs := newCombatGame(t)
	perm := addCreature(gs, 0, "Ordinary Creature", 2, 2)
	card := perm.Card

	DestroyPermanent(gs, perm, nil)

	if !cardInSlice(gs.Seats[0].Graveyard, card) {
		t.Errorf("a creature without a finality counter should go to the graveyard")
	}
	if cardInSlice(gs.Seats[0].Exile, card) {
		t.Errorf("a creature without a finality counter should NOT be exiled")
	}
}
