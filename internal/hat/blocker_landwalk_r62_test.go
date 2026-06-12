package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r62 — legality phase-2 finding: every hat enumerated legal blockers
// through gameengine.CanBlock, the nil-GameState wrapper that SKIPS the
// landwalk gate ("requires game state for land checks"), so hats
// assigned blocks against islandwalk/forestwalk/swampwalk attackers
// whose walk applied — and DeclareBlockers' hat path applies the map
// unvalidated. 13/13 of the 509.1b violations in the loki phase-2
// discovery run (seeds 550043, 1130043, 1160043, 1440043, 1530043,
// 1540043, 1700043, 1840043, 1950043) were landwalk blocks. Hats now
// call CanBlockGS with the live state.

func landwalkFixture(t *testing.T) (*gameengine.GameState, *gameengine.Permanent, *gameengine.Permanent) {
	t.Helper()
	gs := newTestGame(t, 2)

	walker := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:          "River Merfolk",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     2,
			BaseToughness: 2,
			AST: &gameast.CardAST{
				Name:      "River Merfolk",
				Abilities: []gameast.Ability{&gameast.Keyword{Name: "islandwalk"}},
			},
		},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, walker)

	island := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Island", Owner: 1, Types: []string{"land", "island"}},
		Controller: 1,
		Owner:      1,
	}
	blocker := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:          "Coral Colony",
			Owner:         1,
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 6,
		},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, island, blocker)
	return gs, walker, blocker
}

func TestGreedy_AssignBlockers_RespectsLandwalk(t *testing.T) {
	gs, walker, blocker := landwalkFixture(t)
	h := &GreedyHat{}
	got := h.AssignBlockers(gs, 1, []*gameengine.Permanent{walker})
	if bs := got[walker]; len(bs) != 0 {
		t.Fatalf("islandwalker attacking an Island-controlling defender must be unblockable; greedy assigned %d blocker(s)", len(bs))
	}
	// Control: with no Island, the same block is legal.
	gs.Seats[1].Battlefield = gs.Seats[1].Battlefield[1:] // drop the Island
	got = h.AssignBlockers(gs, 1, []*gameengine.Permanent{walker})
	if bs := got[walker]; len(bs) != 1 || bs[0] != blocker {
		t.Fatalf("without the Island, the block is legal and expected; got %v", bs)
	}
}

func TestYggdrasil_AssignBlockers_RespectsLandwalk(t *testing.T) {
	gs, walker, _ := landwalkFixture(t)
	h := NewYggdrasilHat(nil, 0)
	got := h.AssignBlockers(gs, 1, []*gameengine.Permanent{walker})
	if bs := got[walker]; len(bs) != 0 {
		t.Fatalf("islandwalker attacking an Island-controlling defender must be unblockable; yggdrasil assigned %d blocker(s)", len(bs))
	}
}

// End-to-end: the full DeclareBlockers hat path with the legality
// validator attached produces ZERO 509.1b violations for the landwalk
// board — the exact loki cluster shape, now clean.
func TestDeclareBlockers_Landwalk_NoLegalityViolation(t *testing.T) {
	gs, walker, _ := landwalkFixture(t)
	v := gameengine.NewLegalityValidator(7)
	gs.Legality = v
	gs.Seats[1].Hat = &GreedyHat{}

	gameengine.DeclareBlockers(gs, []*gameengine.Permanent{walker}, 1)

	for _, viol := range v.Violations {
		t.Errorf("landwalk block flagged post-fix: %s", viol.String())
	}
}
