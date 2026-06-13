package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// land_mana_ability_r63_test.go — pins the CR §605.3 mana-ability
// classification fix (ModificationEffect mana ModKinds resolve inline,
// not on the stack) and the filter-land add_mana_effect colored output.

func lmPerm(gs *GameState, seat int, name string, ast *gameast.CardAST, types ...string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Owner: seat, Types: append([]string{}, types...), AST: ast},
		Controller: seat, Owner: seat,
		Flags:    map[string]int{},
		Counters: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func activatedManaAST(name string, eff gameast.Effect) *gameast.CardAST {
	return &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Activated{Cost: gameast.Cost{Tap: true}, Effect: eff},
		},
	}
}

// --- Fix #1: CR §605.3 — count-scaled mana is a mana ability ----------

func TestLandMana_AddManaPer_IsManaAbility(t *testing.T) {
	gs := newTestGameState(2)
	cradle := lmPerm(gs, 0, "Gaea's Cradle",
		activatedManaAST("Gaea's Cradle", &gameast.ModificationEffect{
			ModKind: "add_mana_per", Args: []interface{}{"{G}", "creature you control"},
		}), "land")
	for i := 0; i < 3; i++ {
		lmPerm(gs, 0, "Bear", nil, "creature")
	}
	if !IsManaAbility(cradle, 0) {
		t.Fatalf("add_mana_per must be classified as a mana ability (CR §605.1a)")
	}
	if err := ActivateAbility(gs, 0, cradle, 0, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	// Resolved inline, not on the stack.
	if len(gs.Stack) != 0 {
		t.Errorf("mana ability must not linger on the stack, got %d items", len(gs.Stack))
	}
	// Exactly 3 (no double-add from hook + AST resolution).
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("expected 3 mana, got %d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Mana == nil || gs.Seats[0].Mana.G != 3 {
		t.Errorf("expected 3 green in typed pool, got %+v", gs.Seats[0].Mana)
	}
	// The inline path emits the §605.3a marker.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "activate_mana_ability" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected activate_mana_ability event (inline §605.3a resolution)")
	}
}

func TestLandMana_AddManaEffect_IsManaAbility(t *testing.T) {
	gs := newTestGameState(2)
	gate := lmPerm(gs, 0, "Mystic Gate",
		activatedManaAST("Mystic Gate", &gameast.ModificationEffect{
			ModKind: "add_mana_effect", Args: []interface{}{"add {w}{w}, {w}{u}, or {u}{u}"},
		}), "land")
	if !IsManaAbility(gate, 0) {
		t.Fatalf("add_mana_effect must be classified as a mana ability")
	}
	if err := ActivateAbility(gs, 0, gate, 0, nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("mana ability must not use the stack, got %d items", len(gs.Stack))
	}
}

// choose_color is deliberately NOT a mana ability (ambiguous text).
func TestLandMana_ChooseColor_NotManaAbility(t *testing.T) {
	gs := newTestGameState(2)
	crater := lmPerm(gs, 0, "Meteor Crater",
		activatedManaAST("Meteor Crater", &gameast.ModificationEffect{
			ModKind: "choose_color",
		}), "land")
	if IsManaAbility(crater, 0) {
		t.Errorf("choose_color must NOT be classified as a mana ability")
	}
}

// --- Fix #2: filter lands add TWO colored mana, not one generic -------

func TestAddManaEffect_FilterLand_TwoColored(t *testing.T) {
	gs := newTestGameState(2)
	gate := lmPerm(gs, 0, "Mystic Gate", nil, "land")
	resolveModificationEffect(gs, gate, &gameast.ModificationEffect{
		ModKind: "add_mana_effect", Args: []interface{}{"add {w}{w}, {w}{u}, or {u}{u}"},
	})
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("filter land should add 2 mana, got %d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Mana == nil || gs.Seats[0].Mana.W != 1 || gs.Seats[0].Mana.U != 1 {
		t.Errorf("expected 1W + 1U, got %+v", gs.Seats[0].Mana)
	}
}

func TestAddManaEffect_FilterLand_BlackRed(t *testing.T) {
	gs := newTestGameState(2)
	cairns := lmPerm(gs, 0, "Graven Cairns", nil, "land")
	resolveModificationEffect(gs, cairns, &gameast.ModificationEffect{
		ModKind: "add_mana_effect", Args: []interface{}{"add {b}{b}, {b}{r}, or {r}{r}"},
	})
	if gs.Seats[0].Mana == nil || gs.Seats[0].Mana.B != 1 || gs.Seats[0].Mana.R != 1 {
		t.Errorf("expected 1B + 1R, got %+v", gs.Seats[0].Mana)
	}
}

// Variable-quantity storage-land text keeps the conservative 1-generic
// fallback (X is a cost-driven count we don't model here).
func TestAddManaEffect_VariableText_FallsBackToGeneric(t *testing.T) {
	gs := newTestGameState(2)
	pools := lmPerm(gs, 0, "Calciform Pools", nil, "land")
	resolveModificationEffect(gs, pools, &gameast.ModificationEffect{
		ModKind: "add_mana_effect", Args: []interface{}{"add x mana in any combination of {w} and/or {u}"},
	})
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("variable add_mana_effect should fall back to 1 generic, got %d", gs.Seats[0].ManaPool)
	}
}
