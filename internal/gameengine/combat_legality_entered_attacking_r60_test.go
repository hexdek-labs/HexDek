package gameengine

import (
	"testing"
)

// Loki r60 trajectory at seed 42 (cumulative): r41 1652 -> r44 402 ->
// round 1 52 -> round 2 10 -> round 3 6. Of the residual 6, two were
// the same bit-stable signature:
//
//   "Behemoth of Vault 0" (seat 0) is attacking with summoning sickness
//   and no haste
//
// Bit-stable across r41 / r44 / round 1 / round 2 / round 3.
//
// Root cause: CR §506.3 "enters the battlefield tapped and attacking"
// creatures (created by seven per_card handlers — Raph & Mikey, Kaalia,
// Sauron, Strefan, Winota, Satya, ninjutsu) all stamp Flags["attacking"]=1
// on a freshly-created permanent whose SummoningSick is still true. The
// scoop-in loop at combat.go:533 (the canonical "you are attacking per
// §506.3" moment) DIDN'T clear SummoningSick before, so any mid-combat
// game end (TakeTurn returns early on gs.CheckEnd() before
// EndOfCombatStep clears flagAttacking) left the bad SS=true +
// attacking=true combo for checkCombatLegality to flag.
//
// Fix: the scoop-in now clears SummoningSick before adding the creature
// to the attacker pool. CR §506.3's "license to attack" is also a
// license to ignore §302.1 summoning sickness for this combat — the
// rules-correct semantic.

// TestDeclareAttackers_ScoopInClearsSummoningSick pins the new clear:
// a creature with Flags["attacking"]=1 + SummoningSick=true gets scooped
// in AND has SS cleared.
func TestDeclareAttackers_ScoopInClearsSummoningSick(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	gs.Step = "declare_attackers"

	// Opponent must exist as a defender (gs.LivingOpponents must be non-empty).
	addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	// "Entered tapped and attacking" creature on seat 0 — what Raph &
	// Mikey / Kaalia / Sauron / Strefan / Winota / Satya / ninjutsu create.
	cheated := addBattlefield(gs, 0, "Behemoth of Vault 0", 6, 6, "creature", "artifact")
	cheated.SummoningSick = true
	cheated.Flags["attacking"] = 1

	attackers := DeclareAttackers(gs, 0)

	// The scoop-in should include the cheated creature.
	found := false
	for _, p := range attackers {
		if p == cheated {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("scoop-in should pick up Flags[attacking]=1 creature; got %d attackers (%v)", len(attackers), attackers)
	}

	// SS must be cleared by the scoop-in.
	if cheated.SummoningSick {
		t.Errorf("SummoningSick should be cleared by §506.3 scoop-in")
	}
	// flagAttacking stays true (it's still attacking).
	if !permFlag(cheated, flagAttacking) {
		t.Errorf("flagAttacking should remain true after scoop-in")
	}
}

// TestCombatLegality_NoLeakOnMidCombatGameEnd is the cross-validation
// test: simulate the "Behemoth of Vault 0" scenario where the game ends
// mid-combat (loki's TakeTurn returns early before EndOfCombatStep
// clears flagAttacking) and verify CombatLegality stays clean.
func TestCombatLegality_NoLeakOnMidCombatGameEnd(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	gs.Step = "declare_attackers"

	addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	cheated := addBattlefield(gs, 0, "Behemoth of Vault 0", 6, 6, "creature", "artifact")
	cheated.SummoningSick = true
	cheated.Flags["attacking"] = 1

	DeclareAttackers(gs, 0)

	// Simulate mid-combat game end — TakeTurn's `if gs.CheckEnd() { return }`
	// fires before EndOfCombatStep runs, so gs.Phase stays "combat" and
	// flagAttacking stays set. checkCombatLegality fires after that.
	gs.Step = "end_of_combat"

	if err := checkCombatLegality(gs); err != nil {
		t.Fatalf("CombatLegality must stay clean post-scoop-in: %v", err)
	}
}

// TestDeclareAttackers_StandardAttackerNotAffected guards the negative
// case: a normal attacker (declared via the legal-pool path, never
// summoning-sick because it's been on the battlefield) is unchanged by
// the new SS-clearing behavior.
func TestDeclareAttackers_StandardAttackerNotAffected(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	gs.Step = "declare_attackers"

	addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	normal := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	normal.SummoningSick = false
	// No Flags["attacking"] — it'll go through the canAttack legal pool.

	attackers := DeclareAttackers(gs, 0)

	if len(attackers) == 0 {
		t.Fatalf("normal attacker should be declared")
	}
	// SS stays false (it was already false; the clear loop doesn't run for it).
	if normal.SummoningSick {
		t.Errorf("normal attacker SummoningSick should stay false")
	}
}

// TestDeclareAttackers_SummoningSickNotInAttackerPool guards the
// other negative case: a summoning-sick creature WITHOUT
// Flags["attacking"]=1 should NOT be picked up — canAttack filters it
// out of the legal pool, and the scoop-in doesn't see it.
func TestDeclareAttackers_SummoningSickNotInAttackerPool(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	gs.Step = "declare_attackers"

	addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	sicky := addBattlefield(gs, 0, "Just Cast", 3, 3, "creature")
	sicky.SummoningSick = true
	// NO Flags["attacking"] — this creature was NOT cheated in via §506.3.

	attackers := DeclareAttackers(gs, 0)

	for _, p := range attackers {
		if p == sicky {
			t.Fatalf("summoning-sick non-§506.3 creature should not attack")
		}
	}
	// SS must stay true — it was never granted §506.3 license.
	if !sicky.SummoningSick {
		t.Errorf("SS should stay true for a normal summoning-sick creature")
	}
}
