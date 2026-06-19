package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// graft_r63_test.go — Graft (CR §702.58, Ravnica). ApplyGraftETB /
// ApplyGraftTransfer existed but had NO caller — graft was fully inert (no ETB
// counters, the another-creature-enters move never offered). These pin both
// halves now that ApplyGraft is wired into the ETB paths.

func graftAST(name string, n int) *gameast.CardAST {
	return &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "graft", Args: []interface{}{n}}},
	}
}

// (1) A graft N creature enters with N +1/+1 counters.
func TestGraft_EntersWithCounters(t *testing.T) {
	gs := newCombatGame(t)
	g := addBattlefieldWithAST(gs, 0, "Simic Basilisk", 0, 0, graftAST("Simic Basilisk", 3), "creature")
	ApplyGraft(gs, g)
	if g.Counters["+1/+1"] != 3 {
		t.Fatalf("graft 3 should enter with 3 +1/+1 counters, got %d", g.Counters["+1/+1"])
	}
}

// (2) ETB counters route through the doubler chain (Doubling Season → 2N),
// proving ApplyGraftETB uses PutCountersTriggered, not raw AddCounter.
func TestGraft_EntersWithCountersDoubled(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	g := addBattlefieldWithAST(gs, 0, "Cytoplast Root-Kin", 0, 0, graftAST("Cytoplast Root-Kin", 4), "creature")
	ApplyGraft(gs, g)
	if g.Counters["+1/+1"] != 8 {
		t.Fatalf("graft 4 with Doubling Season should be 8 counters, got %d", g.Counters["+1/+1"])
	}
}

// (3) When another creature you control enters, the graft creature moves one
// counter onto it (graft decrements, newcomer increments).
func TestGraft_MovesCounterOntoNewCreature(t *testing.T) {
	gs := newCombatGame(t)
	g := addBattlefieldWithAST(gs, 0, "Grafter", 0, 0, graftAST("Grafter", 3), "creature")
	g.AddCounter("+1/+1", 3) // already on the battlefield with its graft counters

	newbie := addBattlefield(gs, 0, "Newcomer", 2, 2, "creature")
	ApplyGraft(gs, newbie) // newbie entering triggers the graft move

	if g.Counters["+1/+1"] != 2 {
		t.Fatalf("graft creature should drop to 2 counters after moving one, got %d", g.Counters["+1/+1"])
	}
	if newbie.Counters["+1/+1"] != 1 {
		t.Fatalf("the entering creature should gain 1 +1/+1 counter, got %d", newbie.Counters["+1/+1"])
	}
}

// (4) Auto-play does NOT feed an opponent's entering creature.
func TestGraft_DoesNotMoveOntoOpponent(t *testing.T) {
	gs := newCombatGame(t)
	g := addBattlefieldWithAST(gs, 0, "Grafter", 0, 0, graftAST("Grafter", 3), "creature")
	g.AddCounter("+1/+1", 3)

	oppNewbie := addBattlefield(gs, 1, "Opp Newcomer", 2, 2, "creature")
	ApplyGraft(gs, oppNewbie)

	if g.Counters["+1/+1"] != 3 {
		t.Fatalf("graft creature must not feed an opponent's creature, counters dropped to %d", g.Counters["+1/+1"])
	}
	if oppNewbie.Counters["+1/+1"] != 0 {
		t.Fatalf("opponent's creature should not receive a graft counter, got %d", oppNewbie.Counters["+1/+1"])
	}
}

// (5) A graft creature with no counters left offers no move.
func TestGraft_NoCountersNoMove(t *testing.T) {
	gs := newCombatGame(t)
	addBattlefieldWithAST(gs, 0, "Empty Grafter", 0, 0, graftAST("Empty Grafter", 1), "creature")
	// (no counters added — graft creature is depleted)
	newbie := addBattlefield(gs, 0, "Newcomer", 2, 2, "creature")
	ApplyGraft(gs, newbie)
	if newbie.Counters["+1/+1"] != 0 {
		t.Fatalf("a depleted graft creature must not move a counter, newcomer got %d", newbie.Counters["+1/+1"])
	}
}

// (6) Integration: a graft creature entering via the non-cast ETB path
// (FirePermanentETBTriggers) gets its counters — proving the hook is wired.
func TestGraft_WiredIntoETBPath(t *testing.T) {
	gs := newCombatGame(t)
	g := addBattlefieldWithAST(gs, 0, "Plaxcaster Frogling", 0, 0, graftAST("Plaxcaster Frogling", 3), "creature")
	FirePermanentETBTriggers(gs, g)
	if g.Counters["+1/+1"] != 3 {
		t.Fatalf("graft creature entering via FirePermanentETBTriggers should have 3 counters, got %d", g.Counters["+1/+1"])
	}
}
