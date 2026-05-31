package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func addLibraryWithType(gs *gameengine.GameState, seat int, name string, types ...string) *gameengine.Card {
	c := &gameengine.Card{Name: name, Owner: seat, Types: append([]string{}, types...)}
	gs.Seats[seat].Library = append(gs.Seats[seat].Library, c)
	return c
}

// TestRampantGrowth_TutorsBasicToBattlefieldTapped pins the canonical
// Rampant Growth resolution: search basic land → battlefield tapped.
func TestRampantGrowth_TutorsBasicToBattlefieldTapped(t *testing.T) {
	gs := newGame(t, 2)
	forest := addLibraryWithType(gs, 0, "Forest", "land", "basic", "forest")
	addLibraryWithType(gs, 0, "Grizzly Bears", "creature")

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Rampant Growth", Owner: 0, Types: []string{"sorcery"}},
		Kind:       "spell",
	}
	gameengine.InvokeResolveHook(gs, item)

	foundOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == forest {
			foundOnBF = true
			if !p.Tapped {
				t.Errorf("Rampant Growth land should enter tapped")
			}
			break
		}
	}
	if !foundOnBF {
		t.Errorf("expected Forest tutored onto battlefield; bf=%+v", gs.Seats[0].Battlefield)
	}
}

// TestCultivate_OneBasicToBFTappedOneToHand pins Cultivate's
// split-destination: 2 basics → 1 BF tapped + 1 to hand.
func TestCultivate_OneBasicToBFTappedOneToHand(t *testing.T) {
	gs := newGame(t, 2)
	forest1 := addLibraryWithType(gs, 0, "Forest", "land", "basic", "forest")
	forest2 := addLibraryWithType(gs, 0, "Forest 2", "land", "basic", "forest")
	addLibraryWithType(gs, 0, "Bear", "creature")

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Cultivate", Owner: 0, Types: []string{"sorcery"}},
		Kind:       "spell",
	}
	gameengine.InvokeResolveHook(gs, item)

	bfFound := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && (p.Card == forest1 || p.Card == forest2) {
			bfFound = true
			if !p.Tapped {
				t.Errorf("Cultivate land on BF should be tapped")
			}
			break
		}
	}
	if !bfFound {
		t.Errorf("expected one Forest on battlefield")
	}
	handFound := false
	for _, c := range gs.Seats[0].Hand {
		if c == forest1 || c == forest2 {
			handFound = true
			break
		}
	}
	if !handFound {
		t.Errorf("expected one Forest in hand")
	}
}

// TestKodamasReach_SameAsCultivate pins that Kodama's Reach uses the
// same shared resolve handler as Cultivate.
func TestKodamasReach_SameAsCultivate(t *testing.T) {
	gs := newGame(t, 2)
	addLibraryWithType(gs, 0, "Forest", "land", "basic", "forest")
	addLibraryWithType(gs, 0, "Forest 2", "land", "basic", "forest")

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Kodama's Reach", Owner: 0, Types: []string{"sorcery", "arcane"}},
		Kind:       "spell",
	}
	gameengine.InvokeResolveHook(gs, item)

	// Expect 1 land on battlefield + 1 in hand.
	bfCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsLand() {
			bfCount++
		}
	}
	if bfCount != 1 {
		t.Errorf("expected 1 land on bf after Kodama's Reach; got %d", bfCount)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 land in hand after Kodama's Reach; got %d", len(gs.Seats[0].Hand))
	}
}

// TestThreeVisits_TutorsForestUntapped pins Three Visits: search
// Forest → battlefield (NOT tapped, distinct from Rampant Growth).
func TestThreeVisits_TutorsForestUntapped(t *testing.T) {
	gs := newGame(t, 2)
	plains := addLibraryWithType(gs, 0, "Plains", "land", "basic", "plains")
	forest := addLibraryWithType(gs, 0, "Forest", "land", "basic", "forest")

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Three Visits", Owner: 0, Types: []string{"sorcery"}},
		Kind:       "spell",
	}
	gameengine.InvokeResolveHook(gs, item)

	foundForest := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == forest {
			foundForest = true
			if p.Tapped {
				t.Errorf("Three Visits land should enter UNTAPPED")
			}
			break
		}
	}
	if !foundForest {
		t.Errorf("expected Forest tutored")
	}
	// Plains should NOT be tutored (filter is Forest only).
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == plains {
			t.Errorf("Plains should not match Three Visits filter")
		}
	}
}

// TestNaturesLore_SameAsThreeVisits pins shared handler.
func TestNaturesLore_SameAsThreeVisits(t *testing.T) {
	gs := newGame(t, 2)
	addLibraryWithType(gs, 0, "Forest", "land", "basic", "forest")

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Nature's Lore", Owner: 0, Types: []string{"sorcery"}},
		Kind:       "spell",
	}
	gameengine.InvokeResolveHook(gs, item)

	bfCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsLand() {
			bfCount++
			if p.Tapped {
				t.Errorf("Nature's Lore land should enter untapped")
			}
		}
	}
	if bfCount != 1 {
		t.Errorf("expected 1 land on bf after Nature's Lore; got %d", bfCount)
	}
}

// TestFarseek_TutorsNonForestBasicTapped pins Farseek: search
// Plains/Island/Swamp/Mountain → BF tapped. Does NOT find Forest.
func TestFarseek_TutorsNonForestBasicTapped(t *testing.T) {
	gs := newGame(t, 2)
	forest := addLibraryWithType(gs, 0, "Forest", "land", "basic", "forest")
	island := addLibraryWithType(gs, 0, "Island", "land", "basic", "island")

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Farseek", Owner: 0, Types: []string{"sorcery"}},
		Kind:       "spell",
	}
	gameengine.InvokeResolveHook(gs, item)

	foundIsland := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == island {
			foundIsland = true
			if !p.Tapped {
				t.Errorf("Farseek land should enter tapped")
			}
			break
		}
	}
	if !foundIsland {
		t.Errorf("expected Island tutored by Farseek (skips Forest)")
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == forest {
			t.Errorf("Forest should NOT match Farseek (filter excludes Forest)")
		}
	}
}

// TestRampantGrowth_EmptyLibraryFizzlesGracefully pins the failure
// path: no basic in library, spell resolves cleanly (shuffle still
// happens implicitly).
func TestRampantGrowth_EmptyLibraryFizzlesGracefully(t *testing.T) {
	gs := newGame(t, 2)
	addLibraryWithType(gs, 0, "Bear", "creature")

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Rampant Growth", Owner: 0, Types: []string{"sorcery"}},
		Kind:       "spell",
	}
	gameengine.InvokeResolveHook(gs, item)

	bfLands := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsLand() {
			bfLands++
		}
	}
	if bfLands != 0 {
		t.Errorf("expected no lands on battlefield (no basic in library); got %d", bfLands)
	}
}
