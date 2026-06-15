package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// CR §603.10 / §608.2g — Greven, Predator Captain ("draw cards equal to
// that creature's power and lose life equal to its toughness") must read
// the sacrificed creature's LAST-KNOWN P/T, including +1/+1 counters and
// other continuous effects, not its printed base. Regression for the r63
// LKI probe: grevenAttacks read Card.BasePower / Card.BaseToughness, so a
// +1/+1-countered sacrifice drew/lost only the base amount.

func greven_addCreature(gs *gameengine.GameState, seat int, name string, pow, tough int) *gameengine.Permanent {
	p := addPerm(gs, seat, name, "creature")
	p.Card.BasePower = pow
	p.Card.BaseToughness = tough
	return p
}

func TestGreven_SacrificeUsesLKIPower_WithCounters(t *testing.T) {
	gs := newGame(t, 2)
	greven := greven_addCreature(gs, 0, "Greven, Predator Captain", 4, 4)
	greven.Card.Types = append(greven.Card.Types, "legendary")

	// A 2/2 with TWO +1/+1 counters → effective 4/4. Base power is 2.
	victim := greven_addCreature(gs, 0, "Boosted Bear", 2, 2)
	victim.Counters["+1/+1"] = 2

	// Stock the library so draws are observable; high life so the loss is
	// survivable.
	addLibrary(gs, 0, "Card A", "Card B", "Card C", "Card D", "Card E")
	gs.Seats[0].Life = 40
	handBefore := len(gs.Seats[0].Hand)
	lifeBefore := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": greven,
		"attacker_seat": greven.Controller,
		"attacker_card": greven.Card,
	})

	drew := len(gs.Seats[0].Hand) - handBefore
	if drew != 4 {
		t.Errorf("Greven must draw equal to the sacrificed creature's LKI power "+
			"(2 base + 2 counters = 4); drew %d", drew)
	}
	if lost := lifeBefore - gs.Seats[0].Life; lost != 4 {
		t.Errorf("Greven must lose life equal to the sacrificed creature's LKI "+
			"toughness (4); lost %d", lost)
	}
}

// Control: a vanilla sacrifice (no counters) still works off its base P/T.
func TestGreven_SacrificeVanilla_UsesBase(t *testing.T) {
	gs := newGame(t, 2)
	greven := greven_addCreature(gs, 0, "Greven, Predator Captain", 4, 4)
	greven.Card.Types = append(greven.Card.Types, "legendary")
	greven_addCreature(gs, 0, "Plain Bear", 3, 1)
	addLibrary(gs, 0, "A", "B", "C", "D")
	gs.Seats[0].Life = 40
	handBefore := len(gs.Seats[0].Hand)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": greven,
		"attacker_seat": greven.Controller,
		"attacker_card": greven.Card,
	})
	if drew := len(gs.Seats[0].Hand) - handBefore; drew != 3 {
		t.Errorf("vanilla 3/1 sacrifice should draw 3, got %d", drew)
	}
}
