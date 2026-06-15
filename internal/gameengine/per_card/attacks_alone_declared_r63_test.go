package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// CR §508.4a: a creature "attacks alone" iff it is the only creature
// DECLARED as an attacker. Creatures put onto the battlefield attacking
// (§506.3 — tokens, ninjutsu, Kaalia) were never declared and do not
// count. Regression for the r63 exalted/attacks-alone probe: the Rafiq /
// Squall / Grunn handlers counted IsAttacking() (which includes the §506.3
// scoop-in creatures), diverging from exalted's authoritative
// `len(declared)==1` and wrongly suppressing the trigger when a token
// entered attacking next to a lone declared attacker.

// A lone DECLARED attacker still attacks alone even when a token has been
// put onto the battlefield attacking alongside it.
func TestRafiq_LoneDeclaredAttacker_TokenEnteredAttacking_StillFires(t *testing.T) {
	gs := newGame(t, 2)
	rafiq := addPerm(gs, 0, "Rafiq of the Many", "creature")
	declared := addPerm(gs, 0, "Knight of the Reliquary", "creature")
	setAttacking(declared) // declared this combat

	// A token enters the battlefield attacking — NOT declared.
	token := addPerm(gs, 0, "Soldier Token", "creature", "token")
	setEnteredAttacking(token)

	rafiqAttacksAlone(gs, rafiq, map[string]interface{}{})

	if declared.Flags["kw:double_strike"] != 1 {
		t.Errorf("lone DECLARED attacker must still attack alone despite a §506.3 "+
			"entered-attacking token; double_strike=%d", declared.Flags["kw:double_strike"])
	}
	if token.Flags["kw:double_strike"] == 1 {
		t.Errorf("the entered-attacking token must NOT receive the attacks-alone bonus")
	}
}

// Two genuinely DECLARED attackers → not alone, no trigger.
func TestRafiq_TwoDeclaredAttackers_DoesNotFire(t *testing.T) {
	gs := newGame(t, 2)
	rafiq := addPerm(gs, 0, "Rafiq of the Many", "creature")
	a := addPerm(gs, 0, "Bear A", "creature")
	b := addPerm(gs, 0, "Bear B", "creature")
	setAttacking(a)
	setAttacking(b)

	rafiqAttacksAlone(gs, rafiq, map[string]interface{}{})
	if a.Flags["kw:double_strike"] == 1 || b.Flags["kw:double_strike"] == 1 {
		t.Errorf("two declared attackers are not 'alone' — Rafiq must not fire")
	}
}

// Squall mirrors Rafiq: lone declared attacker + entered-attacking token
// still fires.
func TestSquall_LoneDeclared_TokenEntered_StillFires(t *testing.T) {
	gs := newGame(t, 2)
	squall := addPerm(gs, 0, "Squall, SeeD Mercenary", "creature")
	declared := addPerm(gs, 0, "Lone Rider", "creature")
	setAttacking(declared)
	token := addPerm(gs, 0, "Goblin Token", "creature", "token")
	setEnteredAttacking(token)

	squallSeedAttackAlone(gs, squall, map[string]interface{}{})
	if declared.Flags["kw:double_strike"] != 1 {
		t.Errorf("Squall: lone declared attacker must attack alone despite entered token")
	}
}

// Grunn doubles only when GRUNN is the lone DECLARED attacker; a token
// entering attacking beside it does not break alone-ness, and a token-only
// "attack" (Grunn not declared) does not count.
func TestGrunn_LoneDeclared_TokenEntered_Doubles(t *testing.T) {
	gs := newGame(t, 2)
	grunn := addPerm(gs, 0, "Grunn, the Lonely King", "creature")
	grunn.Card.BasePower, grunn.Card.BaseToughness = 6, 6
	setAttacking(grunn) // declared
	token := addPerm(gs, 0, "Beast Token", "creature", "token")
	setEnteredAttacking(token)

	beforeP := grunn.Power()
	grunnAttacksAlone(gs, grunn, map[string]interface{}{})
	if grunn.Power() <= beforeP {
		t.Errorf("Grunn declared-alone (token entered beside it) should double P/T; "+
			"power %d -> %d", beforeP, grunn.Power())
	}
}

func TestGrunn_EnteredAttackingNotDeclared_DoesNotDouble(t *testing.T) {
	gs := newGame(t, 2)
	grunn := addPerm(gs, 0, "Grunn, the Lonely King", "creature")
	grunn.Card.BasePower, grunn.Card.BaseToughness = 6, 6
	// Grunn entered attacking but was NOT declared → it did not "attack alone".
	setEnteredAttacking(grunn)

	beforeP := grunn.Power()
	grunnAttacksAlone(gs, grunn, map[string]interface{}{})
	if grunn.Power() != beforeP {
		t.Errorf("a Grunn put onto the battlefield attacking was never declared and "+
			"must not double; power %d -> %d", beforeP, grunn.Power())
	}
}

var _ = gameengine.MergeNone // keep the gameengine import anchored
