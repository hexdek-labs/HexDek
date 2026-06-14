package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func inExile(seat *gameengine.Seat, card *gameengine.Card) bool {
	for _, c := range seat.Exile {
		if c == card {
			return true
		}
	}
	return false
}

// (g) Iroh, Grand Lotus — "During your turn, each non-Lesson instant/sorcery
// in your graveyard has flashback = its mana cost; each Lesson has flashback
// {1}." Verifies the per-card grant is registered, consulted at cast time,
// gated to the controller's turn, tiered for Lessons, and exiles on resolve.
// (The CLAUDE.md issue log flagged this as an unimplemented phase_scoped_static
// scaffold; this confirms the r60 grant pipeline closes it end-to-end.)
func TestIroh_GrantsFlashbackToGraveyardInstants(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature")
	gameengine.InvokeETBHook(gs, iroh) // registers the GraveyardFlashbackGrant

	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1,
		ManaCostString: "{R}", AST: &gameast.CardAST{Name: "Lightning Bolt"}}
	lesson := &gameengine.Card{Name: "Introduction to Prophecy", Owner: 0,
		Types: []string{"sorcery", "lesson"}, CMC: 3, AST: &gameast.CardAST{Name: "Introduction to Prophecy"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt, lesson)
	gs.Seats[0].ManaPool = 20

	// Non-Lesson instant → flashback at its mana cost (1).
	if cost, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, bolt); !ok || cost != 1 {
		t.Fatalf("Iroh must grant flashback to {R} Bolt at cost 1, got cost=%d ok=%v", cost, ok)
	}
	// Lesson → flashback {1}.
	if cost, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, lesson); !ok || cost != 1 {
		t.Fatalf("Iroh must grant Lessons flashback {1}, got cost=%d ok=%v", cost, ok)
	}

	// Cast the Bolt via Iroh's grant; it must end in exile.
	if _, err := gameengine.CastFlashback(gs, 0, bolt, -1); err != nil {
		t.Fatalf("CastFlashback via Iroh grant failed: %v", err)
	}
	gameengine.DrainStack(gs)
	if !inExile(gs.Seats[0], bolt) {
		t.Error("Iroh-flashbacked spell must end in exile")
	}
}

// "During your turn" gating: on an opponent's turn, Iroh grants nothing.
func TestIroh_GrantGatedToControllerTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1 // opponent's turn
	iroh := addPerm(gs, 0, "Iroh, Grand Lotus", "creature")
	gameengine.InvokeETBHook(gs, iroh)

	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1,
		ManaCostString: "{R}", AST: &gameast.CardAST{Name: "Lightning Bolt"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	if _, ok := gameengine.EffectiveFlashbackCostFromGraveyardGrants(gs, 0, bolt); ok {
		t.Error("Iroh's grant is 'during your turn' — must not apply on an opponent's turn")
	}
}
