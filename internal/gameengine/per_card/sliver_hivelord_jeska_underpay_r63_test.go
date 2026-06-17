package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Live-grinder §601.2f-h firespot: casting Sliver Hivelord reads "under-paid
// (free or discounted without announcement)" — announced 5, pool delta showed
// -3 spent (before=5 after=8). Not a discount: mana was ADDED during the cast
// window. In the The-First-Sliver deck, casting Sliver Hivelord drains the
// cascade chain inline, and Jeska's Will resolves INSIDE the cast's
// announcement window, adding {R}-per-opponent-hand-card via a raw ManaPool +=
// (the legacy ritual path) that — unlike gameengine.AddMana — did NOT credit
// gs.Legality.NoteManaAdd. The uncredited in-window add made the cast's pool
// delta read as a free/discounted cast. (Mirror of the Hashaton NoteManaSpend
// fix, on the ADD side.)
func TestSliverHivelord_JeskaInWindowAdd_NotFlaggedAsUnderpay(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	gs.Legality = gameengine.NewLegalityValidator(1)

	// An opponent with 3 cards in hand → Jeska's Will Mode 1 adds 3.
	gs.Seats[1].Hand = []*gameengine.Card{
		{Name: "C1", Owner: 1}, {Name: "C2", Owner: 1}, {Name: "C3", Owner: 1},
	}

	hivelord := &gameengine.Card{
		Name:  "Sliver Hivelord",
		Owner: 0,
		// "cost:5" makes CalculateTotalCost (BaseCostAtAnnounce) read 5.
		Types: []string{"legendary", "creature", "sliver", "cost:5"},
		AST:   &gameast.CardAST{Name: "Sliver Hivelord"},
	}
	gs.Seats[0].ManaPool = 5
	gameengine.EnsureTypedPool(gs.Seats[0])

	// --- Sliver Hivelord announced; its {W}{U}{B}{R}{G} leaves the pool. ---
	obs := gs.Legality.BeginCast(gs, 0, hivelord)
	if obs.BaseCostAtAnnounce != 5 {
		t.Fatalf("fixture: Hivelord announced cost = %d, want 5", obs.BaseCostAtAnnounce)
	}
	gs.Seats[0].ManaPool -= 5
	gameengine.SyncManaAfterSpend(gs.Seats[0])

	// --- Jeska's Will resolves INSIDE the window, adding 3 mana. ---
	jeskasWillResolve(gs, &gameengine.StackItem{Controller: 0, Card: &gameengine.Card{Name: "Jeska's Will", Owner: 0}})
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("fixture: Jeska's Will should add 3 (opp hand=3), pool=%d", gs.Seats[0].ManaPool)
	}

	// --- The cast window closes; PoolAfter snapshotted. ---
	gs.Legality.FinishCast(gs, obs, &gameengine.StackItem{Controller: 0, Card: hivelord})

	for _, lv := range gs.Legality.Violations {
		if lv.Rule == "601.2f-h" {
			t.Errorf("Jeska's Will in-window mana add misread as Sliver Hivelord under-pay: %s", lv.Detail)
		}
	}
}
