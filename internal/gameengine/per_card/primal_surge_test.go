package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func newCardWithTypes(name string, owner int, types ...string) *gameengine.Card {
	return &gameengine.Card{
		Name:  name,
		Owner: owner,
		Types: append([]string{}, types...),
	}
}

func makePrimalSurgeStackItem(seat int) *gameengine.StackItem {
	return &gameengine.StackItem{
		Kind:       "spell",
		Card:       &gameengine.Card{Name: "Primal Surge", Owner: seat, Types: []string{"sorcery"}},
		Controller: seat,
	}
}

func TestPrimalSurge_PutsConsecutivePermanentsOnBattlefield(t *testing.T) {
	gs := newGame(t, 2)
	// Library top-down: artifact, creature, enchantment, [stop here: sorcery], land
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Sol Ring", 0, "artifact"),
		newCardWithTypes("Llanowar Elves", 0, "creature"),
		newCardWithTypes("Sylvan Library", 0, "enchantment"),
		newCardWithTypes("Lightning Bolt", 0, "sorcery"),
		newCardWithTypes("Forest", 0, "land"),
	}
	primalSurgeResolve(gs, makePrimalSurgeStackItem(0))

	if len(gs.Seats[0].Battlefield) != 3 {
		t.Fatalf("expected 3 permanents on battlefield, got %d (%v)",
			len(gs.Seats[0].Battlefield), permNames(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Exile) != 1 {
		t.Errorf("expected 1 card (the sorcery) in exile, got %d", len(gs.Seats[0].Exile))
	}
	if gs.Seats[0].Exile[0].DisplayName() != "Lightning Bolt" {
		t.Errorf("expected Lightning Bolt in exile, got %q", gs.Seats[0].Exile[0].DisplayName())
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected 1 card (the Forest) still in library, got %d", len(gs.Seats[0].Library))
	}
}

func TestPrimalSurge_StopsOnFirstNonpermanent(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Counterspell", 0, "instant"),
		newCardWithTypes("Wurmcoil Engine", 0, "artifact", "creature"),
	}
	primalSurgeResolve(gs, makePrimalSurgeStackItem(0))

	if len(gs.Seats[0].Battlefield) != 0 {
		t.Errorf("expected no battlefield additions, got %d", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Exile) != 1 || gs.Seats[0].Exile[0].DisplayName() != "Counterspell" {
		t.Errorf("expected Counterspell exiled and loop stopped, exile=%v", exileNames(gs.Seats[0]))
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected Wurmcoil still in library (loop stopped), got %d cards", len(gs.Seats[0].Library))
	}
}

func TestPrimalSurge_StopsOnEmptyLibrary(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Sol Ring", 0, "artifact"),
	}
	primalSurgeResolve(gs, makePrimalSurgeStackItem(0))

	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("expected 1 permanent, got %d", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("expected empty library, got %d", len(gs.Seats[0].Library))
	}
}

func TestPrimalSurge_DoesNothingOnLostSeat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Lost = true
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("Sol Ring", 0, "artifact")}
	primalSurgeResolve(gs, makePrimalSurgeStackItem(0))
	if len(gs.Seats[0].Battlefield) != 0 || len(gs.Seats[0].Library) != 1 {
		t.Error("Primal Surge should no-op on a Lost seat")
	}
}

func permNames(perms []*gameengine.Permanent) []string {
	out := []string{}
	for _, p := range perms {
		if p != nil && p.Card != nil {
			out = append(out, p.Card.DisplayName())
		}
	}
	return out
}

func exileNames(s *gameengine.Seat) []string {
	out := []string{}
	for _, c := range s.Exile {
		if c != nil {
			out = append(out, c.DisplayName())
		}
	}
	return out
}
