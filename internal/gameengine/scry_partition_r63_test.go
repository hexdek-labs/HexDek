package gameengine

import "testing"

// scry_partition_r63_test.go — regression for the r63 within-zone CardIdentity
// dup (live grinder cluster: Plains/Swamp twice on battlefield, Nature's Lore /
// Mystic Remora / Talisman twice in library). Root cause: a Hat's ChooseScry /
// ChooseSurveil top-empty fallback returned the first looked card in BOTH the
// top and bottom (it forced cards[0] onto top but removed a DIFFERENT card from
// bottom), and Scry/Surveil only validated the COUNT, so the rebuild appended
// that card into the library TWICE. The duplicated library card could then be
// drawn and played, producing the battlefield dup too. These pin the engine
// chokepoint guard: a malformed split can never duplicate a card.

// badSplitHat returns a malformed scry/surveil split — the first looked card in
// BOTH halves (count matches `n` but it's not a partition), modeling the buggy
// Hat fallback.
type badSplitHat struct{ GreedyHatStub }

func (*badSplitHat) ChooseScry(gs *GameState, seatIdx int, cards []*Card) (top, bottom []*Card) {
	if len(cards) < 2 {
		return cards, nil
	}
	// top=[cards[0]], bottom=[cards[0]] — count 2 == n, but cards[1] dropped
	// and cards[0] duplicated across both halves.
	return []*Card{cards[0]}, []*Card{cards[0]}
}

func (*badSplitHat) ChooseSurveil(gs *GameState, seatIdx int, cards []*Card) (graveyard, top []*Card) {
	if len(cards) < 2 {
		return nil, cards
	}
	return []*Card{cards[0]}, []*Card{cards[0]}
}

func libDupCount(cards []*Card) int {
	seen := map[*Card]bool{}
	dups := 0
	for _, c := range cards {
		if c == nil {
			continue
		}
		if seen[c] {
			dups++
		}
		seen[c] = true
	}
	return dups
}

func scryTestGame(t *testing.T) (*GameState, *Card, *Card, *Card) {
	t.Helper()
	gs := newKWCombatGame(t)
	a := &Card{Name: "Nature's Lore", Owner: 0, InstanceID: "h0OGVG200001", Types: []string{"sorcery"}}
	b := &Card{Name: "Plains", Owner: 0, InstanceID: "h0OGVC000002", Types: []string{"land"}}
	c := &Card{Name: "Island", Owner: 0, InstanceID: "h0OGVU000003", Types: []string{"land"}}
	gs.Seats[0].Library = []*Card{a, b, c}
	return gs, a, b, c
}

// A malformed scry split must NOT duplicate a card in the library.
func TestScry_MalformedSplitDoesNotDuplicate(t *testing.T) {
	gs, a, b, c := scryTestGame(t)
	gs.Seats[0].Hat = &badSplitHat{}

	Scry(gs, 0, 2)

	lib := gs.Seats[0].Library
	if d := libDupCount(lib); d != 0 {
		t.Errorf("scry must never duplicate a card in the library; got %d dup(s) in %v", d, libNames(lib))
	}
	// All three distinct cards still present exactly once (conservation).
	if len(lib) != 3 {
		t.Errorf("library size must be conserved at 3, got %d (%v)", len(lib), libNames(lib))
	}
	for _, want := range []*Card{a, b, c} {
		n := 0
		for _, x := range lib {
			if x == want {
				n++
			}
		}
		if n != 1 {
			t.Errorf("card %q must appear exactly once, got %d", want.Name, n)
		}
	}
}

// A malformed surveil split must NOT duplicate a card across library+graveyard.
func TestSurveil_MalformedSplitDoesNotDuplicate(t *testing.T) {
	gs, a, b, c := scryTestGame(t)
	gs.Seats[0].Hat = &badSplitHat{}

	Surveil(gs, 0, 2)

	// No card appears twice in the library, nor in both library and graveyard.
	lib := gs.Seats[0].Library
	gy := gs.Seats[0].Graveyard
	if d := libDupCount(lib); d != 0 {
		t.Errorf("surveil must not duplicate within the library; got %d dup(s)", d)
	}
	all := append(append([]*Card{}, lib...), gy...)
	if d := libDupCount(all); d != 0 {
		t.Errorf("surveil must not leave a card in BOTH library and graveyard; got %d cross-zone dup(s)", d)
	}
	if len(all) != 3 {
		t.Errorf("conservation: library+graveyard must total 3, got %d", len(all))
	}
	_, _, _ = a, b, c
}

func libNames(cards []*Card) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		if c != nil {
			out = append(out, c.Name)
		}
	}
	return out
}
