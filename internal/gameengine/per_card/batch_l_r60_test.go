package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch L (R60) — tests for 5 new handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Lotus Petal
// -----------------------------------------------------------------------------

func TestLotusPetal_TapSacAddsMana(t *testing.T) {
	gs := newGame(t, 2)
	petal := addPerm(gs, 0, "Lotus Petal", "artifact")
	preMana := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, petal, 0, nil)

	if gs.Seats[0].ManaPool != preMana+1 {
		t.Errorf("expected mana +1 from Lotus Petal, got pool=%d", gs.Seats[0].ManaPool)
	}
	// Petal should be in graveyard (sacrificed).
	for _, p := range gs.Seats[0].Battlefield {
		if p == petal {
			t.Errorf("Lotus Petal should be sacrificed off battlefield")
		}
	}
	foundInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == petal.Card {
			foundInGY = true
			break
		}
	}
	if !foundInGY {
		t.Errorf("Lotus Petal card should be in graveyard after sac")
	}
}

func TestLotusPetal_AlreadyTappedFails(t *testing.T) {
	gs := newGame(t, 2)
	petal := addPerm(gs, 0, "Lotus Petal", "artifact")
	petal.Tapped = true
	preMana := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, petal, 0, nil)

	if gs.Seats[0].ManaPool != preMana {
		t.Errorf("should not produce mana when already tapped")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when tapped")
	}
}

// -----------------------------------------------------------------------------
// Strip Mine
// -----------------------------------------------------------------------------

func TestStripMine_DestroysAnyOpponentLand(t *testing.T) {
	gs := newGame(t, 2)
	strip := addPerm(gs, 0, "Strip Mine", "land")
	oppLand := addPerm(gs, 1, "Mountain", "basic", "land")

	gameengine.InvokeActivatedHook(gs, strip, 0, nil)

	// Strip should be sacrificed.
	for _, p := range gs.Seats[0].Battlefield {
		if p == strip {
			t.Errorf("Strip Mine should be sacrificed after activation")
		}
	}
	// Opp Mountain should be destroyed.
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppLand {
			t.Errorf("Mountain should be destroyed by Strip Mine")
		}
	}
}

func TestStripMine_NoOpponentLandFails(t *testing.T) {
	gs := newGame(t, 2)
	strip := addPerm(gs, 0, "Strip Mine", "land")

	gameengine.InvokeActivatedHook(gs, strip, 0, nil)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no land target")
	}
	// Strip should still be on battlefield.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == strip {
			found = true
		}
	}
	if !found {
		t.Errorf("Strip Mine should remain on battlefield on failed activation")
	}
}

func TestWasteland_DestroysNonbasicOnly(t *testing.T) {
	gs := newGame(t, 2)
	waste := addPerm(gs, 0, "Wasteland", "land")
	oppNonbasic := addPerm(gs, 1, "Bayou", "land")
	oppBasic := addPerm(gs, 1, "Forest", "basic", "land")

	gameengine.InvokeActivatedHook(gs, waste, 0, nil)

	// Non-basic should be destroyed.
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppNonbasic {
			t.Errorf("Bayou should be destroyed by Wasteland")
		}
	}
	// Basic should survive.
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppBasic {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("Forest (basic) should survive Wasteland")
	}
}

func TestWasteland_RefusesBasicTargetWhenOnlyOption(t *testing.T) {
	gs := newGame(t, 2)
	waste := addPerm(gs, 0, "Wasteland", "land")
	addPerm(gs, 1, "Forest", "basic", "land")

	gameengine.InvokeActivatedHook(gs, waste, 0, nil)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("Wasteland should fail when only basic lands available")
	}
}

// -----------------------------------------------------------------------------
// Faithless Looting
// -----------------------------------------------------------------------------

func TestFaithlessLooting_DrawsTwoDiscardsTwo(t *testing.T) {
	gs := newGame(t, 2)
	// Hand: one good card.
	good := &gameengine.Card{Name: "Wheel", Owner: 0, Types: []string{"sorcery", "cmc:5"}}
	gs.Seats[0].Hand = []*gameengine.Card{good}
	// Library: 3 cards (we'll draw 2; one stays).
	addLibrary(gs, 0, "fresh1", "fresh2", "fresh3")

	card := addCard(gs, 0, "Faithless Looting", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Hand size: started with 1, drew 2 = 3, discard 2 = 1.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected hand size 1 (1 + 2 - 2), got %d", len(gs.Seats[0].Hand))
	}
	// Graveyard should have 2 discarded cards.
	if len(gs.Seats[0].Graveyard) != 2 {
		t.Errorf("expected 2 discards in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
	// The high-CMC card should still be in hand.
	foundGood := false
	for _, c := range gs.Seats[0].Hand {
		if c == good {
			foundGood = true
		}
	}
	if !foundGood {
		t.Errorf("high-CMC card should remain in hand after Looting")
	}
}

// -----------------------------------------------------------------------------
// Dryad of the Ilysian Grove
// -----------------------------------------------------------------------------

func TestDryadIlysianGrove_StampsExtraLandDropAndLandTypes(t *testing.T) {
	gs := newGame(t, 2)
	land1 := addPerm(gs, 0, "Forest", "basic", "land")
	land2 := addPerm(gs, 0, "Mountain", "basic", "land")
	oppLand := addPerm(gs, 1, "Island", "basic", "land")

	dryad := addPerm(gs, 0, "Dryad of the Ilysian Grove", "creature", "enchantment")
	gameengine.InvokeETBHook(gs, dryad)

	if gs.Seats[0].Flags["extra_land_drops"] < 1 {
		t.Errorf("expected extra_land_drops bumped on Dryad ETB, got %d",
			gs.Seats[0].Flags["extra_land_drops"])
	}
	if land1.Flags["dryad_grove_all_types"] != 1 {
		t.Errorf("Forest should have all-types flag")
	}
	if land2.Flags["dryad_grove_all_types"] != 1 {
		t.Errorf("Mountain should have all-types flag")
	}
	if oppLand.Flags["dryad_grove_all_types"] == 1 {
		t.Errorf("opponent's land should NOT receive the type-grant flag")
	}
}

func TestDryadIlysianGrove_NewLandGetsStamp(t *testing.T) {
	gs := newGame(t, 2)
	dryad := addPerm(gs, 0, "Dryad of the Ilysian Grove", "creature", "enchantment")
	gameengine.InvokeETBHook(gs, dryad)

	// Now a new land enters under controller. Simulate via permanent_etb trigger.
	newLand := addPerm(gs, 0, "Swamp", "basic", "land")
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            newLand,
		"controller_seat": 0,
		"card":            newLand.Card,
	})

	if newLand.Flags["dryad_grove_all_types"] != 1 {
		t.Errorf("new land should receive the type-grant flag via refresh trigger")
	}
}

// -----------------------------------------------------------------------------
// Captain Sisay
// -----------------------------------------------------------------------------

func TestCaptainSisay_TutorsLegendaryCard(t *testing.T) {
	gs := newGame(t, 2)
	// Library mix: non-legendary, legendary, non-legendary.
	nonleg := &gameengine.Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}
	legend := &gameengine.Card{Name: "Yawgmoth, Thran Physician", Owner: 0, Types: []string{"creature", "legendary"}}
	other := &gameengine.Card{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = []*gameengine.Card{nonleg, legend, other}

	sisay := addPerm(gs, 0, "Captain Sisay", "creature", "legendary")
	gameengine.InvokeActivatedHook(gs, sisay, 0, nil)

	if !sisay.Tapped {
		t.Errorf("Sisay should be tapped after activation")
	}
	foundLegInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == legend {
			foundLegInHand = true
		}
	}
	if !foundLegInHand {
		t.Errorf("Yawgmoth should be tutored to hand")
	}
}

func TestCaptainSisay_NoLegendaryInLibrary(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Grizzly Bears", "Lightning Bolt")
	sisay := addPerm(gs, 0, "Captain Sisay", "creature", "legendary")

	gameengine.InvokeActivatedHook(gs, sisay, 0, nil)

	if !sisay.Tapped {
		t.Errorf("Sisay should still tap on a whiffed search")
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("hand should be empty on whiff, got %d", len(gs.Seats[0].Hand))
	}
}

func TestCaptainSisay_AlreadyTappedFails(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Anywhere")
	sisay := addPerm(gs, 0, "Captain Sisay", "creature", "legendary")
	sisay.Tapped = true

	gameengine.InvokeActivatedHook(gs, sisay, 0, nil)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when already tapped")
	}
}

// -----------------------------------------------------------------------------
// Registry smoke
// -----------------------------------------------------------------------------

func TestBatchLR60_AllRegistered(t *testing.T) {
	if !HasActivated("Lotus Petal") {
		t.Errorf("Lotus Petal Activated not registered")
	}
	if !HasActivated("Strip Mine") {
		t.Errorf("Strip Mine Activated not registered")
	}
	if !HasActivated("Wasteland") {
		t.Errorf("Wasteland Activated not registered")
	}
	if !HasResolve("Faithless Looting") {
		t.Errorf("Faithless Looting Resolve not registered")
	}
	if !HasETB("Dryad of the Ilysian Grove") {
		t.Errorf("Dryad ETB not registered")
	}
	if !HasTrigger("Dryad of the Ilysian Grove", "permanent_etb") {
		t.Errorf("Dryad permanent_etb trigger not registered")
	}
	if !HasActivated("Captain Sisay") {
		t.Errorf("Captain Sisay Activated not registered")
	}
}
