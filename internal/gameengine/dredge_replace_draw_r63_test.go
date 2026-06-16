package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// dredge_replace_draw_r63_test.go — mechanic-probe (DREDGE, CR §702.52).
// The ActivateDredge primitive existed but had ZERO callers — dredge was
// never offered on a draw, so every dredge card sat inert in the graveyard.
// Fixed by MaybeDredgeReplaceDraw wired into all draw entry points (turn
// draw step, DrawN, resolveDraw) + a DredgeChooser decision hook. These pin
// each property of the draw-replacement.

// dredgeChooserHat is a full Hat (via GreedyHatStub) that also implements
// DredgeChooser. It records the candidates it was offered and either picks
// a named card, the first candidate, or declines.
type dredgeChooserHat struct {
	GreedyHatStub
	pickName   string
	decline    bool
	lastOffered int
}

func (h *dredgeChooserHat) ChooseDredge(gs *GameState, seatIdx int, candidates []*Card) *Card {
	h.lastOffered = len(candidates)
	if h.decline {
		return nil
	}
	if h.pickName != "" {
		for _, c := range candidates {
			if c != nil && c.Name == h.pickName {
				return c
			}
		}
		return nil
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return nil
}

func mkDredgeCard(name string, n int) *Card {
	c := &Card{Name: name, Owner: 0, Types: []string{"creature"}}
	c.AST = &gameast.CardAST{Name: name, Abilities: []gameast.Ability{
		&gameast.Keyword{Name: "dredge", Args: []interface{}{float64(n)}},
	}}
	return c
}

func mkLibrary(seat, count int) []*Card {
	lib := make([]*Card, count)
	for i := range lib {
		lib[i] = &Card{Name: "Lib Card", Owner: seat, Types: []string{"land"}}
	}
	return lib
}

func dredgeGame(t *testing.T, libCount int, gy []*Card, hat Hat) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)
	gs.Seats[0].Library = mkLibrary(0, libCount)
	gs.Seats[0].Graveyard = gy
	gs.Seats[0].Hat = hat
	return gs
}

func dredgeInZone(zone []*Card, c *Card) bool {
	for _, x := range zone {
		if x == c {
			return true
		}
	}
	return false
}

// (a)+(c)+(e)+(f): dredge REPLACES the draw — card returns to hand, N cards
// milled, draw count not incremented (the draw didn't happen).
func TestDredge_ReplacesDraw(t *testing.T) {
	imp := mkDredgeCard("Stinkweed Imp", 5)
	hat := &dredgeChooserHat{pickName: "Stinkweed Imp"}
	gs := dredgeGame(t, 10, []*Card{imp}, hat)

	drawn := DrawN(gs, 0, 1, nil)

	if drawn != 0 {
		t.Fatalf("dredge must REPLACE the draw: DrawN reported %d cards drawn, want 0", drawn)
	}
	if !dredgeInZone(gs.Seats[0].Hand, imp) {
		t.Fatal("(e) dredge card must return to hand")
	}
	if dredgeInZone(gs.Seats[0].Graveyard, imp) {
		t.Fatal("dredge card must have left the graveyard")
	}
	if len(gs.Seats[0].Library) != 5 {
		t.Fatalf("(c) must mill 5 from a 10-card library; library=%d want 5", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Graveyard) != 5 {
		t.Fatalf("(c) 5 milled cards must be in the graveyard; graveyard=%d want 5", len(gs.Seats[0].Graveyard))
	}
	if gs.Seats[0].Turn.CardsDrawn != 0 {
		t.Fatalf("(f) a replaced draw is NOT a draw: CardsDrawn=%d want 0", gs.Seats[0].Turn.CardsDrawn)
	}
}

// (b): can't dredge with fewer than N cards in library — draw normally.
func TestDredge_CannotIfLibraryTooSmall(t *testing.T) {
	imp := mkDredgeCard("Stinkweed Imp", 5)
	hat := &dredgeChooserHat{pickName: "Stinkweed Imp"}
	gs := dredgeGame(t, 4, []*Card{imp}, hat) // 4 < 5

	drawn := DrawN(gs, 0, 1, nil)
	if drawn != 1 {
		t.Fatalf("(b) library < N: must draw normally, got %d drawn want 1", drawn)
	}
	if !dredgeInZone(gs.Seats[0].Graveyard, imp) {
		t.Fatal("(b) dredge card must stay in graveyard when it can't be dredged")
	}
	if gs.Seats[0].Turn.CardsDrawn != 1 {
		t.Fatalf("(b) normal draw must count: CardsDrawn=%d want 1", gs.Seats[0].Turn.CardsDrawn)
	}
}

// (d): multiple dredgers each offer the choice on a draw.
func TestDredge_MultipleDredgersOffered(t *testing.T) {
	imp := mkDredgeCard("Stinkweed Imp", 5)
	troll := mkDredgeCard("Golgari Grave-Troll", 6)
	hat := &dredgeChooserHat{decline: true} // observe candidates, draw normally
	gs := dredgeGame(t, 10, []*Card{imp, troll}, hat)

	DrawN(gs, 0, 1, nil)
	if hat.lastOffered != 2 {
		t.Fatalf("(d) both dredgers must be offered; chooser saw %d candidates want 2", hat.lastOffered)
	}
}

// "you MAY": declining a dredge draws normally.
func TestDredge_DeclineDrawsNormally(t *testing.T) {
	imp := mkDredgeCard("Stinkweed Imp", 5)
	hat := &dredgeChooserHat{decline: true}
	gs := dredgeGame(t, 10, []*Card{imp}, hat)

	drawn := DrawN(gs, 0, 1, nil)
	if drawn != 1 {
		t.Fatalf("declining dredge must draw normally, got %d want 1", drawn)
	}
	if !dredgeInZone(gs.Seats[0].Graveyard, imp) {
		t.Fatal("declined dredge card must stay in graveyard")
	}
}

// A hat WITHOUT the DredgeChooser capability never dredges (safe default).
func TestDredge_NoChooserDrawsNormally(t *testing.T) {
	imp := mkDredgeCard("Stinkweed Imp", 5)
	gs := dredgeGame(t, 10, []*Card{imp}, &GreedyHatStub{}) // no ChooseDredge

	drawn := DrawN(gs, 0, 1, nil)
	if drawn != 1 {
		t.Fatalf("no DredgeChooser: must draw normally, got %d want 1", drawn)
	}
}

// Each draw independently offers dredge — dredging on draw 1 still leaves the
// other dredger to offer on draw 2 (per-draw replacement).
func TestDredge_OfferedPerDraw(t *testing.T) {
	imp := mkDredgeCard("Stinkweed Imp", 5)
	troll := mkDredgeCard("Golgari Grave-Troll", 6)
	hat := &dredgeChooserHat{} // picks first candidate each time
	gs := dredgeGame(t, 20, []*Card{imp, troll}, hat)

	drawn := DrawN(gs, 0, 2, nil)
	if drawn != 0 {
		t.Fatalf("both draws should be dredge-replaced, got %d drawn want 0", drawn)
	}
	if !dredgeInZone(gs.Seats[0].Hand, imp) || !dredgeInZone(gs.Seats[0].Hand, troll) {
		t.Fatal("both dredgers should have returned to hand across the two draws")
	}
}
