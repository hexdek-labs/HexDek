package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func mkFixedSaga(gs *GameState, seat int) *Permanent {
	p := addBattlefield(gs, seat, "Fixed Saga", 0, 0, "enchantment", "saga")
	p.Card.AST = &gameast.CardAST{
		Name: "Fixed Saga",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{ModKind: "saga_chapter", Args: []interface{}{"I"}}},
			&gameast.Static{Modification: &gameast.Modification{ModKind: "saga_chapter", Args: []interface{}{"II"}}},
			&gameast.Static{Modification: &gameast.Modification{ModKind: "saga_chapter", Args: []interface{}{"III"}}},
		},
	}
	if p.Counters == nil {
		p.Counters = map[string]int{}
	}
	return p
}

// TestSaga_InitialLoreCounter_NoRegression_714_3a: a Saga still enters with
// exactly one lore counter, its final chapter is detected, and chapter I fires.
func TestSaga_InitialLoreCounter_NoRegression_714_3a(t *testing.T) {
	gs := newCombatGame(t)
	p := mkFixedSaga(gs, 0)

	initSagaLoreCounters(gs, p)

	if p.Counters["lore"] != 1 {
		t.Fatalf("Saga should enter with 1 lore counter, got %d", p.Counters["lore"])
	}
	if p.Counters["saga_final_chapter"] != 3 {
		t.Errorf("final chapter = %d, want 3", p.Counters["saga_final_chapter"])
	}
	if got := sagaChapterAmounts(gs); len(got) != 1 || got[0] != 1 {
		t.Errorf("chapters fired = %v, want [1]", got)
	}
}

// TestSaga_InitialLoreCounter_DoublingSeason_714_3a pins the Aug 7 2026
// behavior confirmed by judges/wizards: the first lore counter routes through
// the §616 doubler chain, so with Doubling Season a Saga enters with TWO lore
// counters and fires chapters I AND II in ascending stack order (not just II).
func TestSaga_InitialLoreCounter_DoublingSeason_714_3a(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	p := mkFixedSaga(gs, 0)

	initSagaLoreCounters(gs, p)

	if p.Counters["lore"] != 2 {
		t.Fatalf("with Doubling Season the Saga should enter with 2 lore counters, got %d", p.Counters["lore"])
	}
	if got := sagaChapterAmounts(gs); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("chapters fired = %v, want [1 2] (I then II, in stack order)", got)
	}
}
