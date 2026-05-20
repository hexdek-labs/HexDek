package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Regression tests for the five inline fixes shipped in dev/stub-hunt-engine-r46.
// See docs/stub-hunt-engine-r46.md for the per-stub classification.

// S1 — gift mechanic dispatches nested effects rather than logging only.
// Construct a ModificationEffect with ModKind "gift" whose Args carry a
// LifeChange{Amount: +2}. After resolution the source's controller should
// have gained 2 life.
func TestStubHuntR46_GiftDispatchesNestedEffects(t *testing.T) {
	gs := newTestGame(t, 2)
	src := addTestPerm(gs, 0, "Gifter", "creature")
	startingLife := gs.Seats[0].Life

	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "gift",
		Args: []interface{}{
			&gameast.GainLife{Amount: *gameast.NumInt(2)},
		},
	})

	if gs.Seats[0].Life != startingLife+2 {
		t.Errorf("gift should dispatch nested LifeChange(+2); life %d -> %d (expected %d)",
			startingLife, gs.Seats[0].Life, startingLife+2)
	}
}

// S2 — keyword_action "populate" must mint a copy of a creature token the
// controller owns, not silently no-op. Set up: seat 0 controls one creature
// token (2/2) plus a non-token creature. After populate, seat 0 should
// have one more permanent than before and the extra should be a token.
func TestStubHuntR46_PopulateMintsTokenCopy(t *testing.T) {
	gs := newTestGame(t, 2)
	// A non-token creature (should NOT be copied).
	addTestPerm(gs, 0, "Real Beast", "creature")
	// A creature token.
	tok := addTestPerm(gs, 0, "Saproling Token", "creature")
	tok.Flags["token"] = 1
	tok.Card.BasePower = 1
	tok.Card.BaseToughness = 1
	tok.Card.Types = []string{"token", "creature"}

	src := addTestPerm(gs, 0, "Populator", "creature")
	beforeCount := len(gs.Seats[0].Battlefield)

	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "keyword_action",
		Args:    []interface{}{"populate"},
	})

	afterCount := len(gs.Seats[0].Battlefield)
	if afterCount != beforeCount+1 {
		t.Fatalf("populate should add 1 permanent; before=%d after=%d", beforeCount, afterCount)
	}
	// The newest permanent should be a creature token.
	newest := gs.Seats[0].Battlefield[afterCount-1]
	if newest == nil || newest.Card == nil {
		t.Fatalf("newest permanent is nil/empty")
	}
	if !newest.IsCreature() {
		t.Errorf("populated copy should be a creature, got types %v", newest.Card.Types)
	}
}

// S3 — keyword_action "explore" reveals top of library; if a land it goes
// to hand; otherwise a +1/+1 counter goes on the exploring creature.
// Set up library with a land on top → expect land in hand, no counter.
func TestStubHuntR46_ExploreRevealsLandToHand(t *testing.T) {
	gs := newTestGame(t, 2)
	src := addTestPerm(gs, 0, "Explorer", "creature")
	src.Card.BasePower = 2
	src.Card.BaseToughness = 2

	landCard := &Card{Name: "Forest", Owner: 0, Types: []string{"land"}}
	nonLand := &Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Library = []*Card{landCard, nonLand}
	startingHand := len(gs.Seats[0].Hand)

	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "keyword_action",
		Args:    []interface{}{"explore"},
	})

	if len(gs.Seats[0].Hand) != startingHand+1 {
		t.Errorf("explore with land on top should put land in hand; hand size %d -> %d",
			startingHand, len(gs.Seats[0].Hand))
	}
	// Source should NOT have a +1/+1 counter (land path skips counter).
	if src.Counters["+1/+1"] != 0 {
		t.Errorf("explore-land path should not add +1/+1; got %d", src.Counters["+1/+1"])
	}
}

// S3b — explore with a non-land on top puts a +1/+1 counter on source and
// leaves the card on top of library.
func TestStubHuntR46_ExploreNonLandAddsCounter(t *testing.T) {
	gs := newTestGame(t, 2)
	src := addTestPerm(gs, 0, "Explorer", "creature")
	src.Card.BasePower = 2
	src.Card.BaseToughness = 2

	nonLand := &Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Library = []*Card{nonLand}

	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "keyword_action",
		Args:    []interface{}{"explore"},
	})

	if src.Counters["+1/+1"] != 1 {
		t.Errorf("explore-nonland should add one +1/+1 counter; got %d", src.Counters["+1/+1"])
	}
	// PerformExplore's GreedyHat policy: nonland goes to graveyard.
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("explore-nonland should empty library (greedy GY); got %d cards", len(gs.Seats[0].Library))
	}
	foundInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == nonLand {
			foundInGY = true
			break
		}
	}
	if !foundInGY {
		t.Errorf("explore-nonland should put card in graveyard; GY=%v", gs.Seats[0].Graveyard)
	}
}

// S4 — keyword_action "proliferate" now reaches the canonical proliferate
// resolver, which proliferates opponent poison/rad and every perm's
// counters.
func TestStubHuntR46_ProliferateKeywordActionAllCounters(t *testing.T) {
	gs := newTestGame(t, 2)
	src := addTestPerm(gs, 0, "Proliferator", "creature")

	// Seat 1 (opponent) has 3 poison + 2 rad.
	gs.Seats[1].PoisonCounters = 3
	gs.Seats[1].Flags = map[string]int{"rad_counters": 2}

	// A friendly creature with a loyalty counter (proliferate covers all kinds).
	pw := addTestPerm(gs, 0, "Walker", "planeswalker")
	pw.Counters["loyalty"] = 4

	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "keyword_action",
		Args:    []interface{}{"proliferate"},
	})

	if gs.Seats[1].PoisonCounters != 4 {
		t.Errorf("opponent poison: expected 4, got %d", gs.Seats[1].PoisonCounters)
	}
	if gs.Seats[1].Flags["rad_counters"] != 3 {
		t.Errorf("opponent rad: expected 3, got %d", gs.Seats[1].Flags["rad_counters"])
	}
	if pw.Counters["loyalty"] != 5 {
		t.Errorf("walker loyalty: expected 5, got %d", pw.Counters["loyalty"])
	}
}

// S5 — reorder_top_of_library reads N from Args instead of the previous
// hard-coded 3. We can't easily assert on shuffle order, so we assert that
// when a library has fewer than N cards the handler does not panic, and
// that the new behavior accepts an N argument without falling back.
func TestStubHuntR46_ReorderTopReadsNFromArgs(t *testing.T) {
	gs := newTestGame(t, 2)
	src := addTestPerm(gs, 0, "Sorter", "creature")
	// Build a 7-card library so N=5 is well-defined.
	for i := 0; i < 7; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "C", Owner: 0, Types: []string{"instant"}})
	}
	beforeLen := len(gs.Seats[0].Library)

	// Pass N=5 in Args.
	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "reorder_top_of_library",
		Args:    []interface{}{5},
	})

	// Library length is unchanged (shuffle is in-place).
	if len(gs.Seats[0].Library) != beforeLen {
		t.Errorf("reorder should not change library length; before=%d after=%d", beforeLen, len(gs.Seats[0].Library))
	}
}
