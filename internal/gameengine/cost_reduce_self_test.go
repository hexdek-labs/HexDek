package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Generic this_spell_colored_cost_reduce coverage: a spell with a self
// cost-reduction static now actually costs less. Before this the kind was in
// the log-only static bucket and the spell was charged full price.

func mkCostReduceSpell(cmc int, amt, clause string) *Card {
	args := []interface{}{amt}
	if clause != "" {
		args = append(args, clause)
	}
	return &Card{
		Name: "Test Reducer", Owner: 0, CMC: cmc, Types: []string{"sorcery"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "this_spell_colored_cost_reduce", Args: args,
			}},
		}},
	}
}

func TestCostReduceSelf_ForEachCreatureInGraveyard(t *testing.T) {
	gs := newTestGame(t, 2)
	for i := 0; i < 3; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			&Card{Name: "Dead Bear", Owner: 0, Types: []string{"creature"}})
	}
	// Non-creatures in GY should not count.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&Card{Name: "Old Spell", Owner: 0, Types: []string{"instant"}})

	card := mkCostReduceSpell(6, "{1}", "for each creature card in your graveyard")
	cost := CalculateTotalCost(gs, card, 0)
	if cost != 3 {
		t.Fatalf("expected 6 - 3 = 3, got %d", cost)
	}
}

func TestCostReduceSelf_ForEachInstantSorceryInGraveyard(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Graveyard = []*Card{
		{Name: "Bolt", Owner: 0, Types: []string{"instant"}},
		{Name: "Divination", Owner: 0, Types: []string{"sorcery"}},
		{Name: "Bear", Owner: 0, Types: []string{"creature"}}, // not counted
	}
	card := mkCostReduceSpell(5, "{1}", "for each instant and sorcery card in your graveyard")
	if got := CalculateTotalCost(gs, card, 0); got != 3 {
		t.Fatalf("expected 5 - 2 = 3, got %d", got)
	}
}

func TestCostReduceSelf_Party(t *testing.T) {
	gs := newTestGame(t, 2)
	addTestPerm(gs, 0, "Cleric Guy", "creature", "cleric")
	addTestPerm(gs, 0, "Rogue Guy", "creature", "rogue")
	card := mkCostReduceSpell(5, "{1}", "for each creature in your party")
	// party = 2 (cleric + rogue) → 5 - 2 = 3.
	if got := CalculateTotalCost(gs, card, 0); got != 3 {
		t.Fatalf("expected 5 - 2 = 3 (party of 2), got %d", got)
	}
}

func TestCostReduceSelf_IfConditionMetAndNot(t *testing.T) {
	gs := newTestGame(t, 2)
	card := mkCostReduceSpell(4, "{2}", "if you've cast another spell this turn")
	// Not cast another spell yet → full cost 4.
	if got := CalculateTotalCost(gs, card, 0); got != 4 {
		t.Fatalf("expected full cost 4 when no other spell cast, got %d", got)
	}
	gs.Seats[0].Turn.SpellsCast = 1
	if got := CalculateTotalCost(gs, card, 0); got != 2 {
		t.Fatalf("expected 4 - 2 = 2 after casting another spell, got %d", got)
	}
}

func TestCostReduceSelf_TargetDependentSkipped(t *testing.T) {
	gs := newTestGame(t, 2)
	// "if it targets a tapped creature" is target-dependent → no reduction.
	card := mkCostReduceSpell(3, "{2}", "if it targets a tapped creature")
	if got := CalculateTotalCost(gs, card, 0); got != 3 {
		t.Fatalf("target-dependent clause must NOT reduce (full cost 3), got %d", got)
	}
}

func TestCostReduceSelf_UnknownClauseNoReduction(t *testing.T) {
	gs := newTestGame(t, 2)
	card := mkCostReduceSpell(4, "{1}", "if the stars align in mysterious ways")
	if got := CalculateTotalCost(gs, card, 0); got != 4 {
		t.Fatalf("unrecognized clause must not reduce, got %d", got)
	}
}

func TestCostReduceSelf_FloorsAtZero(t *testing.T) {
	gs := newTestGame(t, 2)
	for i := 0; i < 5; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			&Card{Name: "Dead Bear", Owner: 0, Types: []string{"creature"}})
	}
	card := mkCostReduceSpell(2, "{1}", "for each creature card in your graveyard")
	// 2 - 5 floored at 0.
	if got := CalculateTotalCost(gs, card, 0); got != 0 {
		t.Fatalf("expected cost floored at 0, got %d", got)
	}
}

func TestCostReduceSelf_NoStaticUnaffected(t *testing.T) {
	gs := newTestGame(t, 2)
	plain := &Card{Name: "Plain Spell", Owner: 0, CMC: 4, Types: []string{"sorcery"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{}}}
	if got := CalculateTotalCost(gs, plain, 0); got != 4 {
		t.Fatalf("a spell without the static must cost full price 4, got %d", got)
	}
}
