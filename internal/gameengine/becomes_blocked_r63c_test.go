package gameengine

// r63c PROGRESSION phase-3c: FireBecomesBlockedTriggers is the batch-
// wrapped entry point for "whenever ~ becomes blocked," dispatch (CR
// §509.4). The combat flow fires the same triggers inline under the
// declare-blockers priority batch (fireBlockTriggers); this entry point
// resolves them immediately for the PROGRESSION trigger-correctness harness.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func becomesBlockedDrawBearer(gs *GameState, seat int, raw string) *Permanent {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "becomes_blocked"},
		Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}},
		Raw:     raw,
	}
	p := addBattlefield(gs, seat, "Blocked Drawer", 2, 2, "creature")
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{tr}}
	return p
}

func TestBecomesBlocked_FiresForAttacker(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A", "Card B")
	atk := becomesBlockedDrawBearer(gs, 0, "whenever this creature becomes blocked, draw a card")

	before := len(gs.Seats[0].Hand)
	FireBecomesBlockedTriggers(gs, atk)
	if got := len(gs.Seats[0].Hand) - before; got != 1 {
		t.Fatalf("becomes-blocked trigger must fire: drew %d cards, want 1", got)
	}
}

func TestBecomesBlocked_SilentWithoutTrigger(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A")
	// A vanilla attacker with no becomes-blocked trigger.
	v := addBattlefield(gs, 0, "Vanilla Attacker", 2, 2, "creature")

	before := len(gs.Seats[0].Hand)
	FireBecomesBlockedTriggers(gs, v)
	if got := len(gs.Seats[0].Hand) - before; got != 0 {
		t.Fatalf("a creature with no becomes-blocked trigger must stay silent: drew %d", got)
	}
}
