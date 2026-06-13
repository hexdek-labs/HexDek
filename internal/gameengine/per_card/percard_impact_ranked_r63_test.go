package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Impact-ranked dead-on-arrival coverage (dev/percard-impact-ranked-r63):
// high-deck-frequency staples whose abilities parsed to inert scaffolds and
// silently did nothing. Each test pins the now-implemented behavior.

func countCreatures(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.IsCreature() {
			n++
		}
	}
	return n
}

func TestRealityShift_ExilesAndControllerManifests(t *testing.T) {
	if !HasResolve("Reality Shift") {
		t.Fatal("Reality Shift not registered")
	}
	gs := newGame(t, 2)
	victim := addPerm(gs, 1, "Big Threat", "creature")
	addLibrary(gs, 1, "Top Card", "Lib2", "Lib3")
	libBefore := len(gs.Seats[1].Library)

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       addCard(gs, 0, "Reality Shift", "instant"),
		Targets:    []gameengine.Target{{Kind: gameengine.TargetKindPermanent, Permanent: victim}},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Victim exiled from its controller's battlefield.
	for _, p := range gs.Seats[1].Battlefield {
		if p == victim {
			t.Error("target creature should have been exiled")
		}
	}
	// Its controller (seat 1) manifested the top card: library down one, a
	// face-down 2/2 creature now on seat 1's battlefield.
	if len(gs.Seats[1].Library) != libBefore-1 {
		t.Errorf("controller's library should drop by 1 (manifest); got %d (was %d)", len(gs.Seats[1].Library), libBefore)
	}
	if countCreatures(gs, 1) != 1 {
		t.Errorf("controller should have exactly 1 creature (the manifest), got %d", countCreatures(gs, 1))
	}
}

func TestCullingRitual_DestroysLowMVAndAddsMana(t *testing.T) {
	if !HasResolve("Culling Ritual") {
		t.Fatal("Culling Ritual not registered")
	}
	gs := newGame(t, 2)
	gs.Seats[0].ManaPool = 0
	cheap := addPerm(gs, 0, "Cheap Dork", "creature")
	cheap.Card.CMC = 1
	expensive := addPerm(gs, 0, "Big Bomb", "creature")
	expensive.Card.CMC = 5
	land := addPerm(gs, 0, "Forest", "land")
	land.Card.CMC = 0
	oppArt := addPerm(gs, 1, "Small Rock", "artifact")
	oppArt.Card.CMC = 2

	item := &gameengine.StackItem{Controller: 0, Card: addCard(gs, 0, "Culling Ritual", "sorcery")}
	gameengine.InvokeResolveHook(gs, item)

	// Two nonland MV<=2 permanents destroyed (cheap creature + opp artifact);
	// land and the MV-5 creature survive.
	stillThere := func(p *gameengine.Permanent, seat int) bool {
		for _, x := range gs.Seats[seat].Battlefield {
			if x == p {
				return true
			}
		}
		return false
	}
	if stillThere(cheap, 0) {
		t.Error("MV1 creature should be destroyed")
	}
	if stillThere(oppArt, 1) {
		t.Error("MV2 opponent artifact should be destroyed")
	}
	if !stillThere(expensive, 0) {
		t.Error("MV5 creature should survive")
	}
	if !stillThere(land, 0) {
		t.Error("land should survive (nonland clause)")
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("should add 1 mana per destroyed (2), mana pool=%d", gs.Seats[0].ManaPool)
	}
}

func TestBlasphemousEdict_EachPlayerSacsUpTo13(t *testing.T) {
	if !HasResolve("Blasphemous Edict") {
		t.Fatal("Blasphemous Edict not registered")
	}
	gs := newGame(t, 2)
	// Seat 0: 2 creatures (fewer than 13 → all sacrificed).
	addPerm(gs, 0, "c0a", "creature")
	addPerm(gs, 0, "c0b", "creature")
	// Seat 1: 15 creatures; two strong ones must survive (sac the 13 weakest).
	for i := 0; i < 13; i++ {
		addPerm(gs, 1, "weak", "creature") // power 0
	}
	s1 := addPerm(gs, 1, "strong1", "creature")
	s1.Card.BasePower = 99
	s2 := addPerm(gs, 1, "strong2", "creature")
	s2.Card.BasePower = 99

	item := &gameengine.StackItem{Controller: 0, Card: addCard(gs, 0, "Blasphemous Edict", "sorcery")}
	gameengine.InvokeResolveHook(gs, item)

	if countCreatures(gs, 0) != 0 {
		t.Errorf("seat 0 (2 creatures) should sacrifice all, got %d left", countCreatures(gs, 0))
	}
	if countCreatures(gs, 1) != 2 {
		t.Errorf("seat 1 (15 creatures) should keep 2 after sac'ing 13, got %d left", countCreatures(gs, 1))
	}
	// The survivors must be the strong ones.
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsCreature() && p.Power() != 99 {
			t.Error("seat 1 should keep its strongest creatures, not the weak ones")
		}
	}
}

func TestPromiseOfLoyalty_EachPlayerKeepsStrongest(t *testing.T) {
	if !HasResolve("Promise of Loyalty") {
		t.Fatal("Promise of Loyalty not registered")
	}
	gs := newGame(t, 2)
	// Seat 0: three creatures, one clearly strongest.
	addPerm(gs, 0, "weak0a", "creature")
	addPerm(gs, 0, "weak0b", "creature")
	best0 := addPerm(gs, 0, "best0", "creature")
	best0.Card.BasePower = 7
	best0.Card.BaseToughness = 7
	// Seat 1: two creatures.
	addPerm(gs, 1, "weak1", "creature")
	best1 := addPerm(gs, 1, "best1", "creature")
	best1.Card.BasePower = 4

	item := &gameengine.StackItem{Controller: 0, Card: addCard(gs, 0, "Promise of Loyalty", "sorcery")}
	gameengine.InvokeResolveHook(gs, item)

	if countCreatures(gs, 0) != 1 {
		t.Errorf("seat 0 should keep exactly 1 creature, got %d", countCreatures(gs, 0))
	}
	if countCreatures(gs, 1) != 1 {
		t.Errorf("seat 1 should keep exactly 1 creature, got %d", countCreatures(gs, 1))
	}
	// The kept creatures are the strongest, and they carry a vow counter.
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsCreature() {
			if p != best0 {
				t.Error("seat 0 should keep its strongest creature")
			}
			if p.Counters["vow"] != 1 {
				t.Error("kept creature should have a vow counter")
			}
		}
	}
}

func TestTwinflame_CreatesHastyTokenCopies(t *testing.T) {
	if !HasResolve("Twinflame") {
		t.Fatal("Twinflame not registered")
	}
	gs := newGame(t, 2)
	orig := addPerm(gs, 0, "Hero", "creature")
	orig.Card.BasePower = 3
	orig.Card.BaseToughness = 3

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       addCard(gs, 0, "Twinflame", "sorcery"),
		Targets:    []gameengine.Target{{Kind: gameengine.TargetKindPermanent, Permanent: orig}},
	}
	gameengine.InvokeResolveHook(gs, item)

	if countCreatures(gs, 0) != 2 {
		t.Fatalf("expected original + 1 token copy = 2 creatures, got %d", countCreatures(gs, 0))
	}
	// The copy has haste (not summoning sick) and is flagged a copy.
	foundCopy := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["twinflame_copy"] == 1 {
			foundCopy = true
			if p.SummoningSick {
				t.Error("Twinflame token copy should have haste (not summoning sick)")
			}
		}
	}
	if !foundCopy {
		t.Error("expected a twinflame token copy on the battlefield")
	}
	// A delayed trigger should be scheduled to exile the copies at end step.
	if len(gs.DelayedTriggers) == 0 {
		t.Error("expected a next_end_step delayed trigger to exile the copies")
	}
}
