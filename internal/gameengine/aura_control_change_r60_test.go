package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// aura_control_change_r60_test.go — CR §303.4c / §303.4f / §704.5m
// regression: Auras with controller-restriction enchant clauses must
// fall off when control of the enchanted creature shifts to violate
// the restriction.
//
// CR §303.4c: "An Aura's enchant ability defines what the Aura can be
// enchanting."
// CR §303.4f: "If an Aura is attached to an illegal object or player,
// it's put into its owner's graveyard. This is a state-based action."
// CR §704.5m wraps the SBA itself.
//
// The pre-fix engine's `auraTargetMatchesEnchantClause` only checked
// the bare type-noun ("creature", "land"), so Aurelia's-Fury-style
// "Enchant creature you control" Auras stayed glued to creatures that
// had been Mind-Controlled away. The fix extends the clause check with
// a `parseEnchantControllerClause` pass that recognises "you control"
// and "an opponent controls" qualifiers; sba704_5m's existing call
// site at sba.go:912 already routes the violation to destroyPermSBA.
//
// Two paths covered:
//
//  1. "Enchant creature you control" Aura stays on its host while the
//     controller relationship holds; falls off the moment control of
//     the enchanted creature changes to an opponent (the Mind Control
//     case the user prompt called out).
//
//  2. "Enchant creature an opponent controls" Aura (Confiscate-style,
//     though the canonical case is Threaten-but-keep auras) falls off
//     if you gain control of the enchanted creature.
//
// A negative companion confirms the existing type-only check still
// works for Auras without a controller qualifier (defends against
// over-narrowing the helper).

func attachAuraWith(t *testing.T, gs *GameState, seat int, name string, oracleText string) *Permanent {
	t.Helper()
	aura := addBattlefield(gs, seat, name, 0, 0, "enchantment", "aura")
	aura.Card.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: oracleText},
		},
	}
	return aura
}

// TestAuraEnchantClause_YouControl_FallsOffOnControlChange pins the
// Mind-Control scenario: seat 0's Aura "Enchant creature you control"
// is attached to seat 0's creature; seat 1 steals the creature; the
// Aura must drop to graveyard the next SBA pass.
func TestAuraEnchantClause_YouControl_FallsOffOnControlChange(t *testing.T) {
	gs := newFixtureGame(t)
	creature := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	aura := attachAuraWith(t, gs, 0,
		"Holy Strength of Your Creatures",
		"Enchant creature you control. Enchanted creature gets +1/+2.",
	)
	aura.AttachedTo = creature

	// Sanity: stable on the pre-change state.
	StateBasedActions(gs)
	if !permanentOnBattlefield(gs, aura) {
		t.Fatal("aura should still be on the battlefield while controller-restriction holds")
	}

	// Seat 1 takes control of the creature (Mind Control / Threaten).
	// We mutate Controller directly rather than going through
	// resolveGainControl so the test pins the SBA layer in isolation;
	// resolveGainControl's responsibility is just to flip Controller +
	// move the perm slice membership, both of which the addBattlefield
	// fixture already arranged.
	moveBetweenSeats(gs, creature, 0, 1)

	StateBasedActions(gs)

	if permanentOnBattlefield(gs, aura) {
		t.Fatal("aura with 'enchant creature you control' must fall off after the enchanted creature " +
			"changes controllers to an opponent (CR §303.4f / §704.5m)")
	}
	// And it must be in its OWNER's graveyard (the aura's owner is seat 0
	// — the original controller; control of the aura itself never changed).
	foundInOwnerGraveyard := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == aura.Card {
			foundInOwnerGraveyard = true
			break
		}
	}
	if !foundInOwnerGraveyard {
		t.Errorf("aura should land in seat 0's graveyard (its owner); " +
			"seat 0 graveyard size=%d, seat 1 graveyard size=%d",
			len(gs.Seats[0].Graveyard), len(gs.Seats[1].Graveyard))
	}
}

// TestAuraEnchantClause_OpponentControls_FallsOffWhenYouSteal pins
// the inverse — an "Enchant creature an opponent controls" Aura must
// fall off when you take control of the enchanted creature.
func TestAuraEnchantClause_OpponentControls_FallsOffWhenYouSteal(t *testing.T) {
	gs := newFixtureGame(t)
	// Seat 1's creature is the host; seat 0 controls the Aura.
	creature := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")
	aura := attachAuraWith(t, gs, 0,
		"Pacifism Hostile",
		"Enchant creature an opponent controls. Enchanted creature can't attack.",
	)
	aura.AttachedTo = creature

	// Sanity: stable while the controller restriction holds.
	StateBasedActions(gs)
	if !permanentOnBattlefield(gs, aura) {
		t.Fatal("aura should still be on battlefield while opponent-control restriction holds")
	}

	// Seat 0 takes control of the creature.
	moveBetweenSeats(gs, creature, 1, 0)

	StateBasedActions(gs)

	if permanentOnBattlefield(gs, aura) {
		t.Fatal("aura with 'enchant creature an opponent controls' must fall off when the enchanted " +
			"creature becomes controlled by the aura's controller (CR §303.4f / §704.5m)")
	}
}

// TestAuraEnchantClause_NoQualifier_UnaffectedByControlChange pins the
// negative path: an Aura with a plain "Enchant creature" clause (no
// controller qualifier) must NOT fall off on a control change. Defends
// against the new helper over-firing.
func TestAuraEnchantClause_NoQualifier_UnaffectedByControlChange(t *testing.T) {
	gs := newFixtureGame(t)
	creature := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	aura := attachAuraWith(t, gs, 0,
		"Pacifism",
		"Enchant creature. Enchanted creature can't attack or block.",
	)
	aura.AttachedTo = creature

	moveBetweenSeats(gs, creature, 0, 1)
	StateBasedActions(gs)

	if !permanentOnBattlefield(gs, aura) {
		t.Fatal("plain 'enchant creature' aura must NOT fall off on a mere control change " +
			"(CR §702.6 — control change alone does not detach auras)")
	}
}

// moveBetweenSeats is a test helper that moves perm from src seat's
// battlefield slice to dst seat's battlefield slice and updates
// p.Controller — mirrors what resolveGainControl does, isolated so
// the SBA-layer test doesn't depend on the resolve.go pipeline.
func moveBetweenSeats(gs *GameState, perm *Permanent, src, dst int) {
	bf := gs.Seats[src].Battlefield
	for i, p := range bf {
		if p == perm {
			gs.Seats[src].Battlefield = append(bf[:i], bf[i+1:]...)
			break
		}
	}
	perm.Controller = dst
	gs.Seats[dst].Battlefield = append(gs.Seats[dst].Battlefield, perm)
}
