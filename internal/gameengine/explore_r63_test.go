package gameengine

import (
	"math/rand"
	"testing"
)

func newExploreGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(7)), nil)
}

func exploreCreature(gs *GameState, seat int, name string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Owner: seat, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: seat,
		Owner:      seat,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// (a)/(b) Land on top → goes to the EXPLORING CONTROLLER's hand, no counter.
func TestExplore_LandToHandNoCounter(t *testing.T) {
	gs := newExploreGame(t)
	c := exploreCreature(gs, 0, "Explorer")
	gs.Seats[0].Library = []*Card{{Name: "Forest", Types: []string{"land"}}}

	wasLand, _ := PerformExplore(gs, c)
	if !wasLand {
		t.Fatal("revealing a land should report wasLand=true")
	}
	if c.Counters["+1/+1"] != 0 {
		t.Fatalf("land explore must NOT add a +1/+1 counter, got %d", c.Counters["+1/+1"])
	}
	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0].Name != "Forest" {
		t.Fatalf("revealed land must go to the controller's hand, hand=%v", gs.Seats[0].Hand)
	}
}

// (c)/(d) Nonland on top → +1/+1 counter on the SPECIFIC exploring creature; card binned.
func TestExplore_NonlandCounterOnExploringCreature(t *testing.T) {
	gs := newExploreGame(t)
	c1 := exploreCreature(gs, 0, "Explorer 1")
	c2 := exploreCreature(gs, 0, "Explorer 2")
	gs.Seats[0].Library = []*Card{{Name: "Bolt", Types: []string{"instant"}}}

	wasLand, _ := PerformExplore(gs, c2)
	if wasLand {
		t.Fatal("revealing a nonland should report wasLand=false")
	}
	if c2.Counters["+1/+1"] != 1 {
		t.Fatalf("the exploring creature (c2) must get the +1/+1 counter, got %d", c2.Counters["+1/+1"])
	}
	if c1.Counters["+1/+1"] != 0 {
		t.Fatalf("a non-exploring creature (c1) must NOT get a counter, got %d", c1.Counters["+1/+1"])
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Fatalf("nonland should be binned, graveyard=%v", gs.Seats[0].Graveyard)
	}
}

// (e) "whenever a creature you control explores" fires ONCE per explore.
func TestExplore_FiresTriggerOncePerExplore(t *testing.T) {
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	exploreCount := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "explore" {
			exploreCount++
		}
	}

	gs := newExploreGame(t)
	c := exploreCreature(gs, 0, "Explorer")
	gs.Seats[0].Library = []*Card{
		{Name: "Bolt", Types: []string{"instant"}},
		{Name: "Forest", Types: []string{"land"}},
	}
	PerformExplore(gs, c) // nonland
	PerformExplore(gs, c) // land
	if exploreCount != 2 {
		t.Fatalf("explore trigger must fire once per explore (2 total), fired %d", exploreCount)
	}
}

// (f) Empty library → no crash, no counter, no hand/graveyard change; trigger still fires.
func TestExplore_EmptyLibraryHarmless(t *testing.T) {
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "explore" {
			fired++
		}
	}

	gs := newExploreGame(t)
	c := exploreCreature(gs, 0, "Explorer")
	gs.Seats[0].Library = nil

	wasLand, revealed := PerformExplore(gs, c)
	if wasLand || revealed != nil {
		t.Fatal("empty-library explore should reveal nothing")
	}
	if c.Counters["+1/+1"] != 0 || len(gs.Seats[0].Hand) != 0 || len(gs.Seats[0].Graveyard) != 0 {
		t.Fatal("empty-library explore must not change counters/hand/graveyard")
	}
	if fired != 1 {
		t.Fatalf("the creature still explored on an empty library — trigger should fire once, fired %d", fired)
	}
}

// (a) Explore reveals the EXPLORING CONTROLLER's library, not another seat's.
func TestExplore_UsesControllersLibrary(t *testing.T) {
	gs := newExploreGame(t)
	c := exploreCreature(gs, 1, "Explorer") // seat 1's creature
	gs.Seats[0].Library = []*Card{{Name: "Island", Types: []string{"land"}}}
	gs.Seats[1].Library = []*Card{{Name: "Mountain", Types: []string{"land"}}}

	PerformExplore(gs, c)
	if len(gs.Seats[1].Hand) != 1 || gs.Seats[1].Hand[0].Name != "Mountain" {
		t.Fatalf("explore must use seat 1's (controller's) library, seat1 hand=%v", gs.Seats[1].Hand)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Fatalf("seat 0's library/hand must be untouched, seat0 hand=%v", gs.Seats[0].Hand)
	}
}
