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
// exactly one lore counter and its final chapter is detected.
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
}

// TestSaga_InitialLoreCounter_RoutesThroughDoublerChain_714_3a proves the
// r64 fix: the first lore counter now goes through the canonical
// PutCountersTriggered chokepoint (§616 would_put_counter chain), not a raw
// AddCounter. With Doubling Season in play the initial lore counter is
// doubled to 2 — a raw AddCounter would leave it at 1.
//
// NOTE (pending 7174n1c review): whether Doubling Season SHOULD double a
// Saga's starting lore counter is the open rules fork flagged in
// initSagaLoreCounters (CR §714.3a's new replacement-effect framing). This
// test pins the current implementation's behavior; if the ruling carves lore
// out of doubling, both the handler and this expectation change together.
func TestSaga_InitialLoreCounter_RoutesThroughDoublerChain_714_3a(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	p := mkFixedSaga(gs, 0)

	initSagaLoreCounters(gs, p)

	if p.Counters["lore"] != 2 {
		t.Errorf("with Doubling Season the initial lore counter should route "+
			"through the doubler chain to 2 (proves canonical path, not raw "+
			"AddCounter); got %d", p.Counters["lore"])
	}
}
