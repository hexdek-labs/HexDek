package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch T (R60) — tests for 5 new tutor handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Diabolic Tutor
// -----------------------------------------------------------------------------

func TestDiabolicTutor_FindsAnyCardToHand(t *testing.T) {
	gs := newGame(t, 2)
	junk := &gameengine.Card{Name: "Junk", Owner: 0, Types: []string{"land"}}
	bomb := &gameengine.Card{Name: "Bomb", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = []*gameengine.Card{junk, bomb}

	card := addCard(gs, 0, "Diabolic Tutor", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// First library card matches the no-filter predicate; junk goes to hand.
	foundJunk := false
	for _, c := range gs.Seats[0].Hand {
		if c == junk {
			foundJunk = true
		}
	}
	if !foundJunk {
		t.Errorf("Diabolic Tutor should have moved the first library card (Junk) to hand")
	}
}

func TestDiabolicTutor_EmptyLibraryShuffleNoOp(t *testing.T) {
	gs := newGame(t, 2)
	// Empty library.
	card := addCard(gs, 0, "Diabolic Tutor", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected empty hand on no-find, got %d cards", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Eldritch Evolution
// -----------------------------------------------------------------------------

func TestEldritchEvolution_TutorsCreatureWithinCapCMC(t *testing.T) {
	gs := newGame(t, 2)
	// Sac creature CMC 2 → cap is 4. Library has CMC 3 and CMC 5 creatures
	// — picker should choose CMC 3 (highest <= 4).
	cmc3 := &gameengine.Card{Name: "Mid", Owner: 0, Types: []string{"creature", "cmc:3"}}
	cmc5 := &gameengine.Card{Name: "Big", Owner: 0, Types: []string{"creature", "cmc:5"}}
	gs.Seats[0].Library = []*gameengine.Card{cmc5, cmc3}

	card := addCard(gs, 0, "Eldritch Evolution", "sorcery")
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       card,
		CostMeta:   map[string]interface{}{"sac_cmc": 2},
	}
	gameengine.InvokeResolveHook(gs, item)

	// CMC 3 creature should be on the battlefield.
	foundMid := false
	foundBig := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == cmc3 {
			foundMid = true
		}
		if p != nil && p.Card == cmc5 {
			foundBig = true
		}
	}
	if !foundMid {
		t.Errorf("expected CMC 3 creature on battlefield (cap=4)")
	}
	if foundBig {
		t.Errorf("CMC 5 creature should NOT be on battlefield (over cap)")
	}
}

func TestEldritchEvolution_StampsExileOnResolve(t *testing.T) {
	gs := newGame(t, 2)
	creature := &gameengine.Card{Name: "Filler", Owner: 0, Types: []string{"creature", "cmc:2"}}
	gs.Seats[0].Library = []*gameengine.Card{creature}

	card := addCard(gs, 0, "Eldritch Evolution", "sorcery")
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       card,
		CostMeta:   map[string]interface{}{"sac_cmc": 1},
	}
	gameengine.InvokeResolveHook(gs, item)

	if !gameengine.ShouldExileOnResolve(item) {
		t.Errorf("Eldritch Evolution should stamp exile_on_resolve")
	}
}

func TestEldritchEvolution_NoLegalTargetUnderCapFails(t *testing.T) {
	gs := newGame(t, 2)
	// Cap is 1+2 = 3; library has only a CMC 6 creature.
	big := &gameengine.Card{Name: "Big", Owner: 0, Types: []string{"creature", "cmc:6"}}
	gs.Seats[0].Library = []*gameengine.Card{big}

	card := addCard(gs, 0, "Eldritch Evolution", "sorcery")
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       card,
		CostMeta:   map[string]interface{}{"sac_cmc": 1},
	}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no creature under cap")
	}
}

// -----------------------------------------------------------------------------
// Green Sun's Zenith
// -----------------------------------------------------------------------------

func TestGreenSunsZenith_FindsGreenCreatureUpToX(t *testing.T) {
	gs := newGame(t, 2)
	// X=3. Library has a green CMC 3 creature and a non-green CMC 3
	// creature and a green CMC 5 creature.
	greenMid := &gameengine.Card{Name: "Acidic Slime", Owner: 0, Types: []string{"creature", "cmc:3", "pip:G"}}
	blueMid := &gameengine.Card{Name: "Snapcaster", Owner: 0, Types: []string{"creature", "cmc:3", "pip:U"}}
	greenBig := &gameengine.Card{Name: "Craterhoof", Owner: 0, Types: []string{"creature", "cmc:5", "pip:G"}}
	gs.Seats[0].Library = []*gameengine.Card{blueMid, greenBig, greenMid}

	card := addCard(gs, 0, "Green Sun's Zenith", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card, ChosenX: 3}
	gameengine.InvokeResolveHook(gs, item)

	// Acidic Slime (green CMC 3) should be on battlefield; others stay.
	foundSlime := false
	foundSnap := false
	foundHoof := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil {
			continue
		}
		switch p.Card {
		case greenMid:
			foundSlime = true
		case blueMid:
			foundSnap = true
		case greenBig:
			foundHoof = true
		}
	}
	if !foundSlime {
		t.Errorf("expected green CMC 3 on battlefield")
	}
	if foundSnap {
		t.Errorf("blue creature should not be fetched by GSZ")
	}
	if foundHoof {
		t.Errorf("green CMC 5 should not be fetched at X=3")
	}
}

func TestGreenSunsZenith_ShufflesIntoLibrary(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = []*gameengine.Card{}

	card := addCard(gs, 0, "Green Sun's Zenith", "instant")
	card.Owner = 0
	item := &gameengine.StackItem{Controller: 0, Card: card, ChosenX: 0}
	gameengine.InvokeResolveHook(gs, item)

	// GSZ should be back in library, NOT in graveyard.
	inLibrary := false
	for _, c := range gs.Seats[0].Library {
		if c == card {
			inLibrary = true
		}
	}
	if !inLibrary {
		t.Errorf("GSZ should shuffle itself into library, not go to graveyard")
	}
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Errorf("GSZ should NOT be in graveyard")
		}
	}
}

// -----------------------------------------------------------------------------
// Buried Alive
// -----------------------------------------------------------------------------

func TestBuriedAlive_DumpsThreeCreaturesToGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	c1 := &gameengine.Card{Name: "A", Owner: 0, Types: []string{"creature", "cmc:6"}}
	c2 := &gameengine.Card{Name: "B", Owner: 0, Types: []string{"creature", "cmc:5"}}
	c3 := &gameengine.Card{Name: "C", Owner: 0, Types: []string{"creature", "cmc:4"}}
	c4 := &gameengine.Card{Name: "D", Owner: 0, Types: []string{"creature", "cmc:3"}}
	nonCreature := &gameengine.Card{Name: "Land", Owner: 0, Types: []string{"land"}}
	gs.Seats[0].Library = []*gameengine.Card{nonCreature, c4, c3, c2, c1}

	card := addCard(gs, 0, "Buried Alive", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Top 3 by CMC: c1, c2, c3.
	inGrave := map[*gameengine.Card]bool{}
	for _, c := range gs.Seats[0].Graveyard {
		inGrave[c] = true
	}
	if !inGrave[c1] || !inGrave[c2] || !inGrave[c3] {
		t.Errorf("top 3 creatures by CMC should be in graveyard")
	}
	if inGrave[c4] {
		t.Errorf("CMC 3 creature should not be in graveyard (only top 3 selected)")
	}
	if inGrave[nonCreature] {
		t.Errorf("non-creature should not be in graveyard")
	}
}

func TestBuriedAlive_FewerThanThreeCreaturesLegal(t *testing.T) {
	gs := newGame(t, 2)
	// Only 1 creature in library — "up to 3" allows this.
	c1 := &gameengine.Card{Name: "Solo", Owner: 0, Types: []string{"creature", "cmc:4"}}
	land := &gameengine.Card{Name: "Land", Owner: 0, Types: []string{"land"}}
	gs.Seats[0].Library = []*gameengine.Card{land, c1}

	card := addCard(gs, 0, "Buried Alive", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Graveyard) != 1 || gs.Seats[0].Graveyard[0] != c1 {
		t.Errorf("expected exactly 1 creature in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
}

// -----------------------------------------------------------------------------
// Final Parting
// -----------------------------------------------------------------------------

func TestFinalParting_SplitsHandAndGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	// Hand pick should prefer instant/sorcery; graveyard pick should
	// prefer highest-CMC creature.
	tutor := &gameengine.Card{Name: "Demonic Tutor", Owner: 0, Types: []string{"sorcery", "cmc:2"}}
	bigCreature := &gameengine.Card{Name: "Griselbrand", Owner: 0, Types: []string{"creature", "cmc:8"}}
	smallCreature := &gameengine.Card{Name: "Squire", Owner: 0, Types: []string{"creature", "cmc:1"}}
	gs.Seats[0].Library = []*gameengine.Card{smallCreature, bigCreature, tutor}

	card := addCard(gs, 0, "Final Parting", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Tutor → hand. Griselbrand → graveyard.
	foundTutorInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == tutor {
			foundTutorInHand = true
		}
	}
	foundBigInGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == bigCreature {
			foundBigInGrave = true
		}
	}
	if !foundTutorInHand {
		t.Errorf("expected Demonic Tutor in hand (sorcery beats creatures in hand picker)")
	}
	if !foundBigInGrave {
		t.Errorf("expected Griselbrand in graveyard (highest-CMC creature)")
	}
}

func TestFinalParting_EmptyLibraryFailsCleanly(t *testing.T) {
	gs := newGame(t, 2)
	// Empty library.
	card := addCard(gs, 0, "Final Parting", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on empty library")
	}
	if len(gs.Seats[0].Hand) != 0 || len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected no cards moved on empty library")
	}
}

func TestFinalParting_SingleCreatureGoesToGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	// Only one card in library — creature. Hand picker grades creatures
	// lowest among types, but with only one option it'll still pick
	// SOMETHING (the creature) into hand. Then graveyard picker has
	// nothing — that's the "fewer than 2" path.
	soloCreature := &gameengine.Card{Name: "Solo", Owner: 0, Types: []string{"creature", "cmc:3"}}
	gs.Seats[0].Library = []*gameengine.Card{soloCreature}

	card := addCard(gs, 0, "Final Parting", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0] != soloCreature {
		t.Errorf("expected solo creature in hand, hand=%d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected empty graveyard (no second target), got %d", len(gs.Seats[0].Graveyard))
	}
}
