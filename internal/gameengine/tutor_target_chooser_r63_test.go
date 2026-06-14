package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// libCard builds a typed library card for these tests.
func libCard(name string, owner, cmc int, types ...string) *Card {
	return &Card{Name: name, Owner: owner, Types: types, CMC: cmc}
}

// ----------------------------------------------------------------------------
// Regression 1: the generic tutor must gather ALL matching candidates, not
// stop at the first one in library order. Before the r63 fix the scan broke
// out of the loop as soon as it had `count` matches, so for the common
// count==1 case it grabbed whichever matching card happened to sit earliest
// in (post-shuffle) library order and never consulted the highest-CMC
// baseline. With no Hat chooser attached, the engine should now select the
// highest-CMC match.
// ----------------------------------------------------------------------------

func TestTutorGeneric_NoChooser_PicksHighestCMC(t *testing.T) {
	gs := newFixtureGame(t)

	// Order matters: the WEAK creature is first in library order. The naive
	// pre-fix scan would have grabbed it; the fixed path scans all matches
	// and the baseline picks the biggest threat.
	gs.Seats[0].Library = append(gs.Seats[0].Library,
		libCard("Llanowar Elves", 0, 1, "creature"),
		libCard("Grizzly Bears", 0, 2, "creature"),
		libCard("Craterhoof Behemoth", 0, 8, "creature"),
		libCard("Lightning Bolt", 0, 1, "instant"),
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "creature"},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	if found := ResolveTutorGeneric(gs, 0, tutor); found != 1 {
		t.Fatalf("expected 1 found, got %d", found)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Fatalf("expected 1 card in hand, got %d", len(gs.Seats[0].Hand))
	}
	if got := gs.Seats[0].Hand[0].Name; got != "Craterhoof Behemoth" {
		t.Errorf("expected highest-CMC creature Craterhoof Behemoth, got %s", got)
	}
}

// ----------------------------------------------------------------------------
// Regression 2: when the searcher's Hat implements TutorTargetChooser, the
// engine must route the selection through it and honor the choice — even when
// the chosen card is NOT the highest CMC. This is the seam that lets strategy
// (combo piece / answer / land) override the blunt CMC baseline.
// ----------------------------------------------------------------------------

// comboTutorHat always tutors for a specific named card, ignoring CMC.
type comboTutorHat struct {
	GreedyHatStub
	want    string
	called  bool
	sawCands int
}

func (h *comboTutorHat) ChooseTutorTargets(gs *GameState, seatIdx int, candidates []*Card, count int, query gameast.Filter, destination string) []*Card {
	h.called = true
	h.sawCands = len(candidates)
	for _, c := range candidates {
		if c != nil && c.Name == h.want {
			return []*Card{c}
		}
	}
	return nil
}

func TestTutorGeneric_ChooserPicksComboPiece(t *testing.T) {
	gs := newFixtureGame(t)
	hat := &comboTutorHat{want: "Thassa's Oracle"}
	gs.Seats[0].Hat = hat

	gs.Seats[0].Library = append(gs.Seats[0].Library,
		libCard("Craterhoof Behemoth", 0, 8, "creature"),  // biggest CMC, the baseline pick
		libCard("Thassa's Oracle", 0, 2, "creature"),       // the combo piece we want
		libCard("Grizzly Bears", 0, 2, "creature"),
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "creature"},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	if found := ResolveTutorGeneric(gs, 0, tutor); found != 1 {
		t.Fatalf("expected 1 found, got %d", found)
	}
	if !hat.called {
		t.Fatal("expected the Hat's ChooseTutorTargets to be consulted")
	}
	if hat.sawCands != 3 {
		t.Errorf("expected chooser to see all 3 matching candidates, saw %d", hat.sawCands)
	}
	if got := gs.Seats[0].Hand[0].Name; got != "Thassa's Oracle" {
		t.Errorf("expected combo piece Thassa's Oracle, got %s (CMC baseline leaked through)", got)
	}
}

// ----------------------------------------------------------------------------
// Regression 3: a malformed chooser result (card not among candidates) must
// fall back to the CMC baseline rather than corrupt the search.
// ----------------------------------------------------------------------------

type badTutorHat struct{ GreedyHatStub }

func (*badTutorHat) ChooseTutorTargets(gs *GameState, seatIdx int, candidates []*Card, count int, query gameast.Filter, destination string) []*Card {
	// Return a card that is not in the candidate set — the engine must reject
	// it and fall back.
	return []*Card{libCard("Black Lotus", 0, 0, "artifact")}
}

func TestTutorGeneric_ChooserMalformed_FallsBack(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Hat = &badTutorHat{}

	gs.Seats[0].Library = append(gs.Seats[0].Library,
		libCard("Grizzly Bears", 0, 2, "creature"),
		libCard("Craterhoof Behemoth", 0, 8, "creature"),
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "creature"},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	if found := ResolveTutorGeneric(gs, 0, tutor); found != 1 {
		t.Fatalf("expected 1 found, got %d", found)
	}
	if got := gs.Seats[0].Hand[0].Name; got != "Craterhoof Behemoth" {
		t.Errorf("malformed chooser should fall back to highest-CMC, got %s", got)
	}
}
