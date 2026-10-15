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
// Characterization only: the positive control is asserted so an all-pass run
// cannot masquerade as a result; the colour rows LOG.
// NOTE the shape of this constructor — it IS the finding. The runtime
// gameengine.Card has NO mana-cost string: only `CMC int` and
// `Colors []string`. There is nowhere to put "{2}{R}" versus "{2}{U}", so
// the cast path structurally cannot compare pips against a pool. The
// `cost` argument below is a comment to the reader, not data the engine
// can see.
func r117Card(name, cost string, cmc int, colors ...string) *gameengine.Card {
	_ = cost
	return &gameengine.Card{
		Name: name, CMC: cmc, Colors: colors,
		Types: []string{"sorcery"},
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
		verdict := "offered"
		if !got {
			verdict = "not offered"
		}
		t.Logf("%-22s %-16s on {R}{R}{R} -> %-11s  (colour-legal: %v)",
			r.name, r.cost, verdict, r.legal)
	}
}
