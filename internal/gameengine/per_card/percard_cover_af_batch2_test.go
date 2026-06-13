package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// percard_cover_af_batch2_test.go — regression pins for A–F batch 2:
// Chain of Vapor, Fractured Sanity, Deliver Unto Evil.

func afCardInZone(zone []*gameengine.Card, c *gameengine.Card) bool {
	for _, x := range zone {
		if x == c {
			return true
		}
	}
	return false
}

func permWithCardOnBF(gs *gameengine.GameState, seat int, c *gameengine.Card) bool {
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card == c {
			return true
		}
	}
	return false
}

func stackItem(name string, controller int) *gameengine.StackItem {
	return &gameengine.StackItem{
		Kind:       "spell",
		Controller: controller,
		Card:       &gameengine.Card{Name: name, Owner: controller, Types: []string{"instant"}},
	}
}

// ---------------------------------------------------------------------------
// Chain of Vapor
// ---------------------------------------------------------------------------

func TestChainOfVapor_BouncesBestOpponentNonland(t *testing.T) {
	gs := newGame(t, 2)
	// Opponent's nonland permanent (high CMC) + a land that must be ignored.
	solRing := addPerm(gs, 1, "Sol Ring", "artifact", "cmc:1")
	bigArt := addPerm(gs, 1, "Gilded Lotus", "artifact", "cmc:5")
	addPerm(gs, 1, "Island", "land")
	_ = solRing

	chainOfVaporResolve(gs, stackItem("Chain of Vapor", 0))

	// The highest-CMC nonland (Gilded Lotus) is bounced to its owner's hand.
	if permWithCardOnBF(gs, 1, bigArt.Card) {
		t.Fatal("Gilded Lotus should have been bounced off the battlefield")
	}
	if !afCardInZone(gs.Seats[1].Hand, bigArt.Card) {
		t.Fatal("bounced permanent should be in its owner's hand")
	}
	// The land is untouched.
	landStill := false
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsLand() {
			landStill = true
		}
	}
	if !landStill {
		t.Error("Chain of Vapor must target a nonland permanent; the land should remain")
	}
}

func TestChainOfVapor_NoTarget_NoPanic(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 1, "Island", "land") // only a land — no legal nonland target
	chainOfVaporResolve(gs, stackItem("Chain of Vapor", 0))
}

// ---------------------------------------------------------------------------
// Fractured Sanity
// ---------------------------------------------------------------------------

func TestFracturedSanity_EachOpponentMills14(t *testing.T) {
	gs := newGame(t, 3)
	for _, opp := range []int{1, 2} {
		for i := 0; i < 20; i++ {
			gs.Seats[opp].Library = append(gs.Seats[opp].Library, addCard(gs, opp, "Filler", "creature"))
		}
	}
	fracturedSanityResolve(gs, stackItem("Fractured Sanity", 0))

	for _, opp := range []int{1, 2} {
		if len(gs.Seats[opp].Library) != 6 {
			t.Errorf("seat %d library should be 20-14=6, got %d", opp, len(gs.Seats[opp].Library))
		}
		if len(gs.Seats[opp].Graveyard) != 14 {
			t.Errorf("seat %d graveyard should hold 14 milled cards, got %d", opp, len(gs.Seats[opp].Graveyard))
		}
	}
}

func TestFracturedSanity_ShortLibrary_MillsWhatRemains(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 5; i++ {
		gs.Seats[1].Library = append(gs.Seats[1].Library, addCard(gs, 1, "Filler", "creature"))
	}
	fracturedSanityResolve(gs, stackItem("Fractured Sanity", 0))
	if len(gs.Seats[1].Library) != 0 {
		t.Errorf("short library should be fully milled, got %d left", len(gs.Seats[1].Library))
	}
	if len(gs.Seats[1].Graveyard) != 5 {
		t.Errorf("graveyard should hold 5, got %d", len(gs.Seats[1].Graveyard))
	}
}

// ---------------------------------------------------------------------------
// Deliver Unto Evil
// ---------------------------------------------------------------------------

func TestDeliverUntoEvil_ReturnsTwoBestFromGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	low := addCard(gs, 0, "Lightning Bolt", "instant", "cmc:1")
	mid := addCard(gs, 0, "Mind Stone", "artifact", "cmc:2")
	high1 := addCard(gs, 0, "Grave Titan", "creature", "cmc:6")
	high2 := addCard(gs, 0, "Massacre Wurm", "creature", "cmc:6")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, low, mid, high1, high2)

	deliverUntoEvilResolve(gs, stackItem("Deliver Unto Evil", 0))

	// The two highest-CMC cards return to hand.
	inHand := 0
	for _, c := range []*gameengine.Card{high1, high2} {
		if afCardInZone(gs.Seats[0].Hand, c) {
			inHand++
		}
	}
	if inHand != 2 {
		t.Fatalf("expected the two CMC-6 cards returned to hand, got %d", inHand)
	}
	// The two low-value cards remain in the graveyard.
	if !afCardInZone(gs.Seats[0].Graveyard, low) || !afCardInZone(gs.Seats[0].Graveyard, mid) {
		t.Error("the two least valuable cards should remain in the graveyard")
	}
}

func TestDeliverUntoEvil_FewerThanTwo_ReturnsWhatExists(t *testing.T) {
	gs := newGame(t, 2)
	only := addCard(gs, 0, "Sol Ring", "artifact", "cmc:1")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, only)
	deliverUntoEvilResolve(gs, stackItem("Deliver Unto Evil", 0))
	if !afCardInZone(gs.Seats[0].Hand, only) {
		t.Error("the single graveyard card should be returned to hand")
	}
}
