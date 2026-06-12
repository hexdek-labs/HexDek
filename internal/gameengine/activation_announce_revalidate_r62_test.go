package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// activation_announce_revalidate_r62_test.go — r62.1.
//
// The ride-along legality validator's §608.2c check found activations
// (Ingenuity Engine, Gremlin Mine, Crater Elemental) whose announced
// target was illegal by the time the ability hit the stack: all three
// carry a SACRIFICE cost, and the announce-time pick (step 0.7) could
// choose the very permanent the cost was about to sacrifice — most often
// the source itself ("Sacrifice this artifact: ... target noncreature
// artifact" with the source as the policy pick). Two fixes pinned here:
//
//  1. PICKER: the cost's predicted sacrifice victim is excluded from the
//     announce-time legal set (AnnounceTargets exclude param), and the
//     payment site sacrifices exactly the predicted victim.
//  2. GATE: after costs are paid, the announced list is re-validated
//     (mirror of the resolution-time §608.2b gate); an all-illegal list
//     fizzles the activation before it reaches the stack.

// sacAbilityCard builds a battlefield permanent whose ability 0 is
// "{T}, Sacrifice <sacFilter>: <eff>".
func sacAbilityCard(gs *GameState, seat int, name string, types []string, sacBase string, eff gameast.Effect) *Permanent {
	p := addBattlefield(gs, seat, name, 2, 1, types...)
	p.Card.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost: gameast.Cost{
					Tap:       true,
					Sacrifice: &gameast.Filter{Base: sacBase, Quantifier: "one"},
				},
				Effect: eff,
			},
		},
	}
	return p
}

// TestActivateAnnounce_SacrificeSelfNeverTargetsSelf pins the picker fix:
// a sacrifice-SELF ability whose effect targets a creature must not
// announce the doomed source as its own target, even when the source is
// the engine policy's preferred pick (own creatures score -toughness, so
// the 2/1 source outranks the 2/5 bystander pre-fix).
func TestActivateAnnounce_SacrificeSelfNeverTargetsSelf(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	src := sacAbilityCard(gs, 0, "Crater Mimic", []string{"creature"}, "this",
		&gameast.Destroy{Target: gameast.Filter{Base: "creature", Targeted: true}})
	bystander := addCreature(gs, 0, "Tough Bystander", 2, 5)

	if err := ActivateAbility(gs, 0, src, 0, nil); err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	// The source was sacrificed as the cost...
	if permanentOnBattlefield(gs, src) {
		t.Fatalf("sacrifice cost was not paid: source still on battlefield")
	}
	// ...and the effect must have destroyed the bystander (the only legal
	// target once the doomed source is excluded), not fizzled on a
	// self-target that died to the cost.
	if permanentOnBattlefield(gs, bystander) {
		t.Fatalf("announced target was the about-to-be-sacrificed source (pre-r62.1 bug): bystander survived and the ability fizzled")
	}
}

// TestActivateAnnounce_PostCostGateFizzlesDeadTarget pins the gate fix:
// when a CALLER-supplied announced target is removed by paying the
// activation cost (here: the target IS the sacrifice victim), the
// activation fizzles per §608.2b before reaching the stack — no stack
// item with an illegal target, no effect.
func TestActivateAnnounce_PostCostGateFizzlesDeadTarget(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	// Source: artifact with "{T}, Sacrifice a creature: Destroy target creature".
	src := sacAbilityCard(gs, 0, "Murder Altar", []string{"artifact"}, "creature",
		&gameast.Destroy{Target: gameast.Filter{Base: "creature", Targeted: true}})
	// The seat's ONLY creature — both the sacrifice victim and the
	// caller-announced target.
	ownBear := addCreature(gs, 0, "Own Bear", 2, 2)
	enemy := addCreature(gs, 1, "Enemy Dragon", 6, 6)

	targets := []Target{{Kind: TargetKindPermanent, Permanent: ownBear, Seat: 0}}
	if err := ActivateAbility(gs, 0, src, 0, targets); err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	// Cost paid: bear sacrificed.
	if permanentOnBattlefield(gs, ownBear) {
		t.Fatalf("sacrifice cost was not paid")
	}
	// Announced target died to the cost → the ability must fizzle, NOT
	// retarget onto the enemy dragon (the announced target is binding).
	if !permanentOnBattlefield(gs, enemy) {
		t.Fatalf("fizzled ability re-aimed at a fresh target: Enemy Dragon destroyed")
	}
	if countEvents(gs, "activation_fizzle") == 0 {
		t.Fatalf("expected an activation_fizzle event (rule 608.2b, all announced targets illegal after costs)")
	}
	if len(gs.Stack) != 0 {
		t.Fatalf("fizzled activation left an item on the stack")
	}
}

// TestActivateAnnounce_LegalityValidatorClean is the validator-level pin:
// the exact Gremlin-Mine shape (sacrifice-self + targeted effect, source
// matches its own target filter) run with the ride-along validator
// attached must produce ZERO violations. Pre-r62.1 this flagged §608.2c
// ("announced target no longer legal: target_illegal_permanent").
func TestActivateAnnounce_LegalityValidatorClean(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Legality = NewLegalityValidator(42)

	// "Sacrifice this artifact: destroy target artifact" where the source
	// itself is a legal pick for its own effect filter.
	src := sacAbilityCard(gs, 0, "Gremlin Mimic", []string{"artifact"}, "this",
		&gameast.Destroy{Target: gameast.Filter{Base: "artifact", Targeted: true}})
	// Toughness 5 so the engine policy (own permanents score -toughness)
	// PREFERS the doomed 2/1 source over the trinket pre-fix — making
	// this test a real falsifier: without the exclusion the announce
	// picks the source, the sacrifice cost kills it, and the validator
	// flags §608.2c.
	other := addBattlefield(gs, 0, "Other Trinket", 0, 5, "artifact")

	if err := ActivateAbility(gs, 0, src, 0, nil); err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	for _, v := range gs.Legality.Violations {
		t.Errorf("legality violation on the fixed path: rule=%s detail=%s", v.Rule, v.Detail)
	}
	// And the effect landed on the legal non-doomed artifact.
	if permanentOnBattlefield(gs, other) {
		t.Fatalf("expected the announced target (Other Trinket) to be destroyed")
	}
}
