package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mana_ability_priority_r63_test.go — pins the CR §605.3 PRIORITY-CONTEXT
// contract that complements the inline-resolution fix (commit 31362542):
//
//   §605.3a — A player may activate a mana ability whenever they have
//     priority, AND in doing so they DO NOT pass priority (they may tap
//     several lands, float the mana, and still hold priority to cast a
//     spell in the same window). The ability resolves immediately and is
//     NOT put on the stack, so no player may respond to it.
//   §605.3b — A player may also activate a mana ability while casting a
//     spell or activating an ability, in the middle of paying its cost
//     ("out of band"), even though they technically don't have priority
//     at that instant and even while another object is on the stack.
//
// These tests assert that ActivateAbility, on the mana-ability branch,
// (1) never pushes a stack item, (2) never opens a PriorityRound / drains
// the stack (so priority is retained and the ability is unrespondable),
// and (3) is not gated on an empty stack or on whose turn it is. The
// non-mana contrast test pins that the OTHER branch genuinely does push +
// open priority, so the fork is real.

// singleManaLandAST builds a "{T}: Add {G}" land ability.
func singleManaLandAST(name string) *gameast.CardAST {
	return activatedManaAST(name, &gameast.AddMana{
		Pool: []gameast.ManaSymbol{{Raw: "{G}", Color: []string{"G"}}},
	})
}

// (countEvents lives in resolve_test.go — same package.)

// §605.3a (A): tapping mana sources at priority does NOT pass priority.
// The player floats three mana across three activations — the stack stays
// empty the whole time — then is still able to act in the same priority
// window by casting a 3-cost spell out of the floated mana.
func TestManaAbility_RetainsPriority_CanCastSameWindow(t *testing.T) {
	gs := newTestGameState(2)
	gs.Active = 0
	gs.Phase = "precombat_main"

	lands := []*Permanent{
		lmPerm(gs, 0, "Forest 1", singleManaLandAST("Forest 1"), "land"),
		lmPerm(gs, 0, "Forest 2", singleManaLandAST("Forest 2"), "land"),
		lmPerm(gs, 0, "Forest 3", singleManaLandAST("Forest 3"), "land"),
	}

	for i, land := range lands {
		before := len(gs.Stack)
		if err := ActivateAbility(gs, 0, land, 0, nil); err != nil {
			t.Fatalf("land %d activate: %v", i, err)
		}
		// Retaining priority means the activation neither placed an object
		// on the stack nor advanced/passed priority. The stack is the
		// observable proxy: a mana ability that wrongly used the stack
		// (or opened a priority round) would leave / churn it.
		if len(gs.Stack) != before {
			t.Fatalf("land %d: mana ability changed stack size %d→%d — it must "+
				"resolve inline and retain priority, not pass it (CR §605.3a)",
				i, before, len(gs.Stack))
		}
	}

	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("expected 3 floated mana after tapping 3 lands, got %d", gs.Seats[0].ManaPool)
	}

	// Still our priority window: cast a 3-cost spell out of the floated
	// mana. If activating the mana abilities had passed priority, the
	// floated mana would have emptied and/or the turn structure advanced;
	// the successful cast proves the window (and the mana) survived.
	spell := addHandCardWithEffect(gs, 0, "Llanowar Visionary", 3,
		&gameast.Scry{Count: *gameast.NumInt(1)}, "creature")
	if err := CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("cast in same priority window failed: %v — floated mana / "+
			"priority did not survive the mana activations", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected floated mana fully spent on the 3-cost spell, got %d left",
			gs.Seats[0].ManaPool)
	}

	// The whole sequence used the inline §605.3a marker and NEVER the
	// non-mana §602.1d stack-push marker for the land activations.
	if got := countEvents(gs, "activate_mana_ability"); got != 3 {
		t.Errorf("expected 3 activate_mana_ability (605.3a) events, got %d", got)
	}
	if got := countEvents(gs, "activate_ability"); got != 0 {
		t.Errorf("mana abilities must NOT emit the non-mana activate_ability "+
			"(602.1d) stack-push marker, got %d", got)
	}
}

// §605.3b: a mana ability can be activated with another object already on
// the stack (the engine proxy for "mid-cost-payment / out of band"). The
// activation must produce mana WITHOUT disturbing the stack — it is not a
// stack object and does not open a priority round that could resolve /
// reorder the pending item.
func TestManaAbility_MidCostPayment_NonEmptyStack(t *testing.T) {
	gs := newTestGameState(2)
	gs.Active = 0
	gs.Phase = "precombat_main"

	// A pending spell sits on the stack (someone is mid-cast / mid-payment).
	pending := &StackItem{
		Kind:       "spell",
		Controller: 1,
		Card:       &Card{Name: "Pending Spell", Owner: 1, Types: []string{"instant"}},
	}
	gs.Stack = []*StackItem{pending}

	land := lmPerm(gs, 0, "Forest", singleManaLandAST("Forest"), "land")
	if err := ActivateAbility(gs, 0, land, 0, nil); err != nil {
		t.Fatalf("mana ability with non-empty stack must be legal (CR §605.3b): %v", err)
	}

	if len(gs.Stack) != 1 || gs.Stack[0] != pending {
		t.Fatalf("mana ability disturbed the stack: want [pending], got %d items", len(gs.Stack))
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("expected 1 mana floated mid-payment, got %d", gs.Seats[0].ManaPool)
	}
}

// §605.3b corollary: a mana ability is not gated on whose turn it is — a
// non-active player can tap a land for mana (e.g. to pay for an instant in
// response on someone else's turn).
func TestManaAbility_NonActiveSeatCanActivate(t *testing.T) {
	gs := newTestGameState(2)
	gs.Active = 0
	gs.Phase = "precombat_main"

	land := lmPerm(gs, 1, "Forest", singleManaLandAST("Forest"), "land")
	if err := ActivateAbility(gs, 1, land, 0, nil); err != nil {
		t.Fatalf("non-active seat mana ability must be legal: %v", err)
	}
	if gs.Seats[1].ManaPool != 1 {
		t.Errorf("expected non-active seat to float 1 mana, got %d", gs.Seats[1].ManaPool)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("non-active mana ability must not use the stack, got %d items", len(gs.Stack))
	}
}

// §605.3a unrespondable + fork-is-real: a mana ability resolves inline
// (no stack push, 605.3a marker), while a non-mana activated ability on
// the very same kind of permanent DOES push to the stack and open priority
// (602.1d marker). If the mana branch wrongly went through the stack path
// this contrast collapses.
func TestManaAbility_NotRespondable_VsNonMana(t *testing.T) {
	gs := newTestGameState(2)
	gs.Active = 0
	gs.Phase = "precombat_main"

	// Mana ability — inline, unrespondable.
	manaLand := lmPerm(gs, 0, "Forest", singleManaLandAST("Forest"), "land")
	if err := ActivateAbility(gs, 0, manaLand, 0, nil); err != nil {
		t.Fatalf("mana activate: %v", err)
	}
	if len(gs.Stack) != 0 {
		t.Fatalf("mana ability must not place a respondable object on the stack, got %d", len(gs.Stack))
	}
	if countEvents(gs, "activate_mana_ability") != 1 {
		t.Errorf("expected the inline 605.3a marker for the mana ability")
	}

	// Non-mana activated ability (a tap-to-scry) on a comparable permanent.
	// Scry produces no mana → IsManaAbility is false → the stack path runs:
	// push (602.1d) + PriorityRound + DrainStack.
	rock := lmPerm(gs, 0, "Seer Stone",
		activatedManaAST("Seer Stone", &gameast.Scry{Count: *gameast.NumInt(1)}),
		"artifact")
	if IsManaAbility(rock, 0) {
		t.Fatalf("a scry ability must NOT be classified as a mana ability")
	}
	if err := ActivateAbility(gs, 0, rock, 0, nil); err != nil {
		t.Fatalf("non-mana activate: %v", err)
	}
	if countEvents(gs, "activate_ability") != 1 {
		t.Errorf("non-mana ability must emit the 602.1d stack-push marker (got %d) — "+
			"this proves the mana branch is a genuinely separate, stack-free path",
			countEvents(gs, "activate_ability"))
	}
	// The non-mana ability was pushed AND drained, so the stack is empty
	// again — but the marker proves it took the respondable path.
	if len(gs.Stack) != 0 {
		t.Errorf("non-mana ability should have drained after its priority round, got %d", len(gs.Stack))
	}
}
