package gameengine

import (
	"testing"
)

// Live grinder firespot (STATE-INTEGRITY/CombatLegality, recurring):
//
//	"Phyrexian Myr Token" (seat 3) is attacking with summoning sickness
//	and no haste
//
// Root cause: NOT the declare-attackers gate (which correctly rejects
// summoning-sick no-haste creatures) and NOT token sickness tracking
// (tokens correctly enter SummoningSick=true). The token legally attacked
// WITH haste — Brudiclad, Telchor Engineer grants "creature tokens you
// control have haste" via a flag-stamp sweep, so the freshly-minted
// Phyrexian Myr token has haste at declare-attackers. When Brudiclad later
// leaves combat (e.g. dies to combat damage), its LTB handler strips that
// granted haste. The Myr keeps SummoningSick=true (it entered this turn),
// so a post-declaration census saw attacking + sick + no-haste and
// phantom-flagged a legal attacker.
//
// CR §508.1a is a ONE-TIME declaration-time check, not a continuous
// requirement: a creature that legally passed declare-attackers stays a
// legal attacker even if it loses haste mid-combat. The census now honors
// flagDeclaredAttacker (set at declaration, cleared by EndOfCombatStep) as
// a carve-out, mirroring the existing §508.1g flagEnteredAttacking carve.

// TestSummoningSick_TokenThisTurnCannotAttackWithoutHaste pins the
// declaration gate: a token created this turn (SummoningSick=true) with NO
// haste is filtered out of the legal attacker pool; the same token WITH
// haste (Flags["kw:haste"]) is declared. This guards the side the firespot
// is NOT on — token sickness + the gate are both correct.
func TestSummoningSick_TokenThisTurnCannotAttackWithoutHaste(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	gs.Step = "declare_attackers"

	addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	// A token minted this turn — summoning-sick, no haste.
	tok := addBattlefield(gs, 0, "Phyrexian Myr Token", 1, 1, "creature", "artifact", "token")
	tok.SummoningSick = true

	attackers := DeclareAttackers(gs, 0)
	for _, p := range attackers {
		if p == tok {
			t.Fatalf("summoning-sick token without haste must not attack")
		}
	}
	if !tok.SummoningSick {
		t.Errorf("token SummoningSick should stay true (no §506.3 license)")
	}

	// Now grant haste (Brudiclad's "creature tokens you control have
	// haste" flag-stamp) — the same token may now attack.
	if tok.Flags == nil {
		tok.Flags = map[string]int{}
	}
	tok.Flags["kw:haste"] = 1

	attackers = DeclareAttackers(gs, 0)
	found := false
	for _, p := range attackers {
		if p == tok {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("summoning-sick token WITH haste must be a legal attacker")
	}
}

// TestCombatLegality_DeclaredAttackerLosesHasteMidCombat is the regression
// for the fix: a token that legally attacked with granted haste, then lost
// the haste mid-combat (Brudiclad leaving), must NOT trip CombatLegality.
func TestCombatLegality_DeclaredAttackerLosesHasteMidCombat(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	gs.Step = "declare_attackers"

	addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	// Token minted this turn with Brudiclad-granted haste.
	tok := addBattlefield(gs, 0, "Phyrexian Myr Token", 1, 1, "creature", "artifact", "token")
	tok.SummoningSick = true
	tok.Flags["kw:haste"] = 1

	DeclareAttackers(gs, 0)

	// It was legally declared.
	if !permFlag(tok, flagDeclaredAttacker) {
		t.Fatalf("token with haste should be a declared attacker")
	}
	if !permFlag(tok, flagAttacking) {
		t.Fatalf("token should be attacking")
	}

	// Brudiclad leaves combat: its LTB strips the granted haste. The token
	// is still SummoningSick (entered this turn) and still attacking.
	delete(tok.Flags, "kw:haste")
	if tok.HasKeyword("haste") || gs.HasKeywordOf(tok, "haste") {
		t.Fatalf("haste should be stripped for the test setup")
	}
	if !tok.SummoningSick {
		t.Fatalf("token should still be summoning-sick")
	}

	// Post-declaration census (e.g. after combat-damage step) must stay
	// clean — losing haste after a legal declaration is rules-correct.
	gs.Step = "end_of_combat"
	if err := checkCombatLegality(gs); err != nil {
		t.Fatalf("CombatLegality must not flag a legally-declared attacker that lost haste: %v", err)
	}
}

// TestCombatLegality_UndeclaredSickAttackerStillFlagged guards the carve-out
// from over-reaching: a creature that is attacking + summoning-sick +
// no-haste WITHOUT having been declared (no flagDeclaredAttacker, no §508.1g
// flagEnteredAttacking) is a genuine illegal-attacker bug and must still be
// flagged.
func TestCombatLegality_UndeclaredSickAttackerStillFlagged(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase = "combat"
	gs.Step = "end_of_combat"

	addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	bad := addBattlefield(gs, 0, "Illegally Attacking", 3, 3, "creature")
	bad.SummoningSick = true
	bad.Flags[flagAttacking] = 1
	// No flagDeclaredAttacker, no flagEnteredAttacking, no haste.

	if err := checkCombatLegality(gs); err == nil {
		t.Fatalf("CombatLegality must still flag an undeclared sick no-haste attacker")
	}
}
