package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r117 — does the production cast path enforce COLOURED mana costs?
//
// Taking §107.4c (the `Any`-pays-`{C}` hole in `CanPayColored`) turned up
// something that voids it: `CanPayColored` and `PayColoredPip` have ZERO
// production callers. A caller count excluding comments and _test.go, and
// confirming each symbol is defined:
//
//	CanPayColored     defined 1   prod calls  0
//	PayColoredPip     defined 1   prod calls  0
//	AddMana           defined 1   prod calls 56
//	EnsureTypedPool   defined 1   prod calls 36
//	PayGenericCost    defined 1   prod calls 10   (generic-only by name)
//	PayManaCost       DOES NOT EXIST
//
// So the typed pool is written heavily and its colour predicates are never
// read. `buildCastableList` — the single production cast-candidate
// enumerator — compares `CalculateTotalCost` (an int) against
// `seat.Mana.Total()` (an int). This probe asks the entry point directly
// rather than concluding from the read (lesson 1).
//
// r126 — THIS PROBE WAS A CHARACTERIZATION AND IS NOW A REGRESSION, AND
// ITS ORIGINAL HEADER CONTAINED A CLAIM THAT WAS FALSE WHEN WRITTEN.
//
// It used to say: "The runtime gameengine.Card has NO mana-cost string:
// only CMC int and Colors []string. There is nowhere to put {2}{R} versus
// {2}{U}, so the cast path structurally cannot compare pips against a
// pool." That was wrong. Card.ManaCostString already existed; r124 found
// that no card builder ever ASSIGNED it, which is a different and much
// smaller problem. The constructor below discarded its `cost` argument on
// the strength of that wrong claim, which is why this probe could only
// ever log.
//
// It now populates the cost, and the colour rows ASSERT.
func r117Card(name, cost string, cmc int, colors ...string) *gameengine.Card {
	return &gameengine.Card{
		Name: name, CMC: cmc, Colors: colors,
		ManaCostString: cost,
		Types:          []string{"sorcery"},
	}
}

func TestColourEnforcementProbe_r117(t *testing.T) {
	mk := func(redMana int, hand ...*gameengine.Card) (*gameengine.GameState, int) {
		gs := canonGame(2)
		seat := gs.Seats[0]
		seat.Hand = hand
		seat.Mana = &gameengine.ColoredManaPool{}
		if redMana > 0 {
			gameengine.AddMana(gs, seat, "R", redMana, "r117 Mountain")
		}
		return gs, 0
	}

	offered := func(list []*gameengine.Card, name string) bool {
		for _, c := range list {
			if c != nil && c.Name == name {
				return true
			}
		}
		return false
	}

	// POSITIVE CONTROL: a card that costs MORE than the total available
	// must be excluded. If this is offered the probe is dead and every
	// "offered" below means nothing.
	gs, s := mk(3, r117Card("r117 Too Expensive", "{9}", 9))
	if offered(buildCastableList(gs, s), "r117 Too Expensive") {
		t.Fatal("control: a 9-mana card was offered on 3 mana — probe is dead")
	}
	t.Log("CONTROL  cost 9 vs 3 mana        -> not offered   (the total gate works)")

	// Three mana, all RED. Each of these costs exactly 3 and is castable
	// on total. Only the mono-red one is castable on COLOUR.
	rows := []struct {
		name, cost string
		cmc        int
		colors     []string
		legal      bool
	}{
		{"r117 Red Spell", "{2}{R}", 3, []string{"R"}, true},
		{"r117 Blue Spell", "{2}{U}", 3, []string{"U"}, false},
		{"r117 Triple Blue", "{U}{U}{U}", 3, []string{"U"}, false},
		{"r117 Five Colour", "{W}{U}{B}{R}{G}", 5, []string{"W", "U", "B", "R", "G"}, false},
	}
	for _, r := range rows {
		gs, s := mk(3, r117Card(r.name, r.cost, r.cmc, r.colors...))
		got := offered(buildCastableList(gs, s), r.name)
		if got != r.legal {
			verdict := "offered"
			if !got {
				verdict = "NOT offered"
			}
			t.Errorf("%-22s %-16s on {R}{R}{R} -> %s, want colour-legal=%v (§202.1a)",
				r.name, r.cost, verdict, r.legal)
		}
	}
}
