package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_discover_freecast_r63_test.go — regressions for the r63 discover
// free-cast fix. PerformDiscover's free-cast path used to push a bare
// StackItem (no Effect, no cast bookkeeping) and return, so the discovered
// spell never ran the canonical cast pipeline: cast-triggers didn't fire, it
// never resolved, and discover-off-a-discover never chained (the cascade-family
// bug). The fix mirrors cascade.go's corrected free-cast block.

// (1)/(4) The free cast goes through the canonical pipeline: cast-triggers
// fire and the discovered spell actually resolves (here, a creature onto the
// battlefield) rather than sitting unresolved on the stack.
func TestPerformDiscover_FreeCastRunsCanonicalPipeline(t *testing.T) {
	gs := newMiscGame(t)
	// CMC-3 creature on top → discovered hit, cast for free (CMC >= 3 heuristic).
	creature := addMiscLibrary(gs, 0, "Discovered Beast", 3, "creature")
	creature.BasePower, creature.BaseToughness = 3, 3

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	sawCast := false
	TriggerHook = func(_ *GameState, ev string, ctx map[string]interface{}) {
		if ev == "spell_cast" {
			if c, _ := ctx["card"].(*Card); c == creature {
				sawCast = true
			}
		}
	}

	found := PerformDiscover(gs, 0, 3)
	if found != creature {
		t.Fatalf("expected to discover the CMC-3 creature, got %v", found)
	}
	if !sawCast {
		t.Fatal("free-cast discover must fire cast-triggers (spell_cast) — canonical pipeline was bypassed")
	}
	onBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == creature {
			onBF = true
		}
	}
	if !onBF {
		t.Fatal("discovered free-cast creature must resolve onto the battlefield, not sit on the stack")
	}
}

// (5) Discover off a discover chains: when the discovered free-cast card
// itself has the discover keyword, casting it runs its OWN discover, which
// finds and free-casts the next card. Both resolve onto the battlefield.
func TestPerformDiscover_ChainsThroughFreeCast(t *testing.T) {
	gs := newMiscGame(t)
	// Top: a CMC-3 creature that itself has "discover 3".
	chainer := addMiscLibrary(gs, 0, "Chain Discoverer", 3, "creature")
	chainer.BasePower, chainer.BaseToughness = 3, 3
	chainer.AST = &gameast.CardAST{
		Name: "Chain Discoverer",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "discover", Args: []interface{}{float64(3)}},
		},
	}
	// Below it: a CMC-3 creature the chained discover will find and free-cast.
	inner := addMiscLibrary(gs, 0, "Inner Beast", 3, "creature")
	inner.BasePower, inner.BaseToughness = 3, 3

	found := PerformDiscover(gs, 0, 3)
	if found != chainer {
		t.Fatalf("outer discover should hit Chain Discoverer, got %v", found)
	}

	bf := map[string]bool{}
	for _, p := range gs.Seats[0].Battlefield {
		bf[p.Card.Name] = true
	}
	if !bf["Chain Discoverer"] {
		t.Error("Chain Discoverer should resolve onto the battlefield")
	}
	if !bf["Inner Beast"] {
		t.Error("chained discover must free-cast Inner Beast onto the battlefield (discover-off-discover chaining)")
	}
}

// (2) The cast-or-hand disposition reaches BOTH branches: a low-MV hit goes to
// hand (not force-cast). Pins that the choice isn't hardcoded one way.
func TestPerformDiscover_LowMVHitGoesToHand(t *testing.T) {
	gs := newMiscGame(t)
	cheap := addMiscLibrary(gs, 0, "Cheap Spell", 1, "instant")

	found := PerformDiscover(gs, 0, 3)
	if found != cheap {
		t.Fatalf("expected to discover Cheap Spell, got %v", found)
	}
	inHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == cheap {
			inHand = true
		}
	}
	if !inHand {
		t.Fatal("low-MV discovered card should go to hand under the disposition heuristic")
	}
	// Not on the stack / battlefield.
	if len(gs.Stack) != 0 {
		t.Fatalf("to-hand disposition must not push a stack item, got %d", len(gs.Stack))
	}
}
