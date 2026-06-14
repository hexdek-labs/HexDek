package gameengine

// r63 — cascade chaining INTO a multi-cascade card (CR §702.85b).
//
// A card with multiple cascade instances ("Cascade, cascade" — Maelstrom
// Wanderer ×2, Apex Devastator ×4) fires cascade once per instance when it is
// cast. The CastSpell entry path already loops CascadeCount times, but the
// CHAINED path (when an outer cascade casts a multi-cascade card for free) fired
// cascade only ONCE regardless of the card's instance count — so a cascaded-into
// Maelstrom Wanderer cascaded once instead of twice. This pins the loop.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestCascadeChain_IntoMultiCascadeCard_FiresEachInstance(t *testing.T) {
	gs := cascAuditGame()

	// A cascade×2 nonland of MV 4 (the outer cascade, MV 6, hits this), with
	// two cheaper nonlands beneath it for its two chained cascades to find.
	multi := &Card{
		Name: "Maelstrom Wanderer", Owner: 0, Types: []string{"instant"}, CMC: 4,
		AST: &gameast.CardAST{
			Name: "Maelstrom Wanderer",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "cascade"},
				&gameast.Keyword{Name: "cascade"},
			},
		},
	}
	hitA := &Card{Name: "Hit A", Owner: 0, Types: []string{"instant"}, CMC: 1, AST: &gameast.CardAST{Name: "Hit A"}}
	hitB := &Card{Name: "Hit B", Owner: 0, Types: []string{"instant"}, CMC: 1, AST: &gameast.CardAST{Name: "Hit B"}}
	gs.Seats[0].Library = []*Card{multi, hitA, hitB}

	if !ApplyCascade(gs, 0, 6, "Outer Cascade Source") {
		t.Fatal("outer cascade should hit the cascade×2 card")
	}

	// Outer trigger (1) + the multi-cascade card's TWO chained instances (2)
	// = 3 cascade_trigger events. The pre-fix single chained call produced 2.
	if n := countEvents(gs, "cascade_trigger"); n != 3 {
		t.Fatalf("cascade×2 card cascaded into should fire BOTH instances: cascade_trigger=%d, want 3", n)
	}
	// Both cheaper cards were cascaded (cast) — neither remains in the library.
	for _, c := range gs.Seats[0].Library {
		if c != nil && (c.Name == "Hit A" || c.Name == "Hit B") {
			t.Fatalf("%s should have been cascaded by the second instance, but remains in library", c.Name)
		}
	}
}
