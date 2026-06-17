package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Live-grinder OUTCOME firespot: "Ezrim Agency Chief — investigate twice,
// expected 2 Clues, got 4" (an OVER-count). Ezrim has a per_card OnETB handler
// (ezrimETB → 2 Clues) that fully implements its parsed "investigate twice" AST
// ETB trigger. When CAST (the common case), the engine's cast-path ETB dispatch
// (resolvePermanentSpellETB) fired the generic AST trigger AND invoked the
// per_card hook — because, unlike the non-cast path (FirePermanentETBTriggers),
// it lacked the per_card-owns double-fire gate. Result: investigate ran twice
// (once per path) → 4 Clues. The fix adds the gate to the cast path and declares
// Ezrim's OnETB ownership; this regression pins 2 Clues on a real cast.
func TestEzrim_CastPath_InvestigateTwice(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	ezrim := &gameengine.Card{
		Name:  "Ezrim, Agency Chief",
		Owner: 0,
		Types: []string{"legendary", "creature"},
		AST: &gameast.CardAST{
			Name: "Ezrim, Agency Chief",
			Abilities: []gameast.Ability{
				&gameast.Triggered{
					Trigger: gameast.Trigger{Event: "etb"},
					Effect:  &gameast.ModificationEffect{ModKind: "investigate", Args: []interface{}{2}},
					Raw:     "when ~ enters, investigate twice",
				},
			},
		},
	}
	gs.Stack = append(gs.Stack, &gameengine.StackItem{Card: ezrim, Controller: 0, Kind: "spell"})
	gameengine.ResolveStackTop(gs)
	gameengine.DrainStack(gs)

	got := countClues(gs, 0)
	t.Logf("Ezrim cast → %d Clues (want 2)", got)
	if got != 2 {
		t.Errorf("Ezrim cast created %d Clues, want 2 (per_card OnETB + AST ETB trigger double-fire)", got)
	}
}
