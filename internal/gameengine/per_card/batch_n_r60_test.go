package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch N (R60) — tests for 5 new handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Krark-Clan Ironworks
// -----------------------------------------------------------------------------

func TestKCI_SacArtifactAddsColorlessMana(t *testing.T) {
	gs := newGame(t, 2)
	kci := addPerm(gs, 0, "Krark-Clan Ironworks", "artifact")
	sacFodder := addPerm(gs, 0, "Lotus Petal", "artifact")
	preMana := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, kci, 0, map[string]interface{}{
		"target_perm": sacFodder,
	})

	if gs.Seats[0].ManaPool != preMana+2 {
		t.Errorf("expected mana +2 from KCI activation, got pool=%d", gs.Seats[0].ManaPool)
	}
	// sacFodder should be in graveyard.
	for _, p := range gs.Seats[0].Battlefield {
		if p == sacFodder {
			t.Errorf("sac fodder should be sacrificed off battlefield")
		}
	}
}

func TestKCI_AutoPicksTokenArtifactFirst(t *testing.T) {
	gs := newGame(t, 2)
	kci := addPerm(gs, 0, "Krark-Clan Ironworks", "artifact")
	// Non-token mana rock.
	rock := addPerm(gs, 0, "Sol Ring", "artifact")
	// Token treasure — should be picked first.
	treasure := addPerm(gs, 0, "Treasure Token", "artifact", "token")
	treasure.Card.Owner = 0

	gameengine.InvokeActivatedHook(gs, kci, 0, nil)

	// Treasure should be sac'd.
	for _, p := range gs.Seats[0].Battlefield {
		if p == treasure {
			t.Errorf("KCI should prefer token artifact, treasure still on battlefield")
		}
		if p == rock {
			// expected: rock survives
		}
	}
}

func TestKCI_NoArtifactsFails(t *testing.T) {
	gs := newGame(t, 2)
	kci := addPerm(gs, 0, "Krark-Clan Ironworks", "artifact")
	// No other artifacts.
	preMana := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, kci, 0, nil)

	if gs.Seats[0].ManaPool != preMana {
		t.Errorf("KCI should not produce mana when no sac target; pool=%d", gs.Seats[0].ManaPool)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no artifact to sac")
	}
}

// -----------------------------------------------------------------------------
// Mystic Confluence
// -----------------------------------------------------------------------------

func TestMysticConfluence_CountersBouncesAndDraws(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C", "D")
	// Opp spell on stack (uncounterable target is fine).
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card:       &gameengine.Card{Name: "Brainstorm", Owner: 1, Types: []string{"instant"}},
	}
	gs.Stack = append(gs.Stack, oppSpell)
	gs.Seats[1].ManaPool = 0 // can't pay the {3} tax
	// Opp creature available to bounce.
	bouncee := addPerm(gs, 1, "Sheoldred", "creature")
	bouncee.Card.BasePower = 4
	bouncee.Card.BaseToughness = 4

	card := addCard(gs, 0, "Mystic Confluence", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Errorf("Mystic Confluence should counter the unpaid spell")
	}
	// Sheoldred should have been bounced.
	for _, p := range gs.Seats[1].Battlefield {
		if p == bouncee {
			t.Errorf("opp creature should have been bounced")
		}
	}
	// At least one card drawn.
	if len(gs.Seats[0].Hand) < 1 {
		t.Errorf("expected at least one card drawn, got %d", len(gs.Seats[0].Hand))
	}
}

func TestMysticConfluence_FallsBackToDrawWhenNoTargets(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C", "D")
	// No opp spells, no opp creatures.

	card := addCard(gs, 0, "Mystic Confluence", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Should draw 3 cards.
	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("expected 3 cards drawn via draw fallback, got %d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Treasure Cruise
// -----------------------------------------------------------------------------

func TestTreasureCruise_DrawsThree(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C", "D", "E")

	card := addCard(gs, 0, "Treasure Cruise", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("expected 3 cards drawn, got %d", len(gs.Seats[0].Hand))
	}
}

func TestTreasureCruise_ShortLibraryDrawsAvailable(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B")

	card := addCard(gs, 0, "Treasure Cruise", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected 2 cards drawn from short library, got %d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Stifle / Trickbind
// -----------------------------------------------------------------------------

func TestStifle_CountersTriggeredAbility(t *testing.T) {
	gs := newGame(t, 2)
	// Opp fetchland trigger on stack.
	fetchTrig := &gameengine.StackItem{
		Controller: 1,
		Kind:       "triggered",
		Source:     addPerm(gs, 1, "Polluted Delta", "land"),
	}
	gs.Stack = append(gs.Stack, fetchTrig)

	card := addCard(gs, 0, "Stifle", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !fetchTrig.Countered {
		t.Errorf("Stifle should counter the triggered ability")
	}
}

func TestStifle_CountersActivatedAbility(t *testing.T) {
	gs := newGame(t, 2)
	pwActivation := &gameengine.StackItem{
		Controller: 1,
		Kind:       "activated",
		Source:     addPerm(gs, 1, "Liliana of the Veil", "planeswalker"),
	}
	gs.Stack = append(gs.Stack, pwActivation)

	card := addCard(gs, 0, "Stifle", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !pwActivation.Countered {
		t.Errorf("Stifle should counter the activated ability")
	}
}

func TestStifle_IgnoresSpells(t *testing.T) {
	gs := newGame(t, 2)
	spell := &gameengine.StackItem{
		Controller: 1,
		Kind:       "spell",
		Card:       &gameengine.Card{Name: "Brainstorm", Owner: 1, Types: []string{"instant"}},
	}
	gs.Stack = append(gs.Stack, spell)

	card := addCard(gs, 0, "Stifle", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if spell.Countered {
		t.Errorf("Stifle should not counter a spell")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when only spells on stack")
	}
}

func TestTrickbind_SharesHandler(t *testing.T) {
	gs := newGame(t, 2)
	trig := &gameengine.StackItem{
		Controller: 1,
		Kind:       "triggered",
		Source:     addPerm(gs, 1, "Sensei's Divining Top", "artifact"),
	}
	gs.Stack = append(gs.Stack, trig)

	card := addCard(gs, 0, "Trickbind", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !trig.Countered {
		t.Errorf("Trickbind should counter the triggered ability")
	}
}

// -----------------------------------------------------------------------------
// Yisan, the Wanderer Bard
// -----------------------------------------------------------------------------

func TestYisan_TutorsCreatureMatchingVerses(t *testing.T) {
	gs := newGame(t, 2)
	yisan := addPerm(gs, 0, "Yisan, the Wanderer Bard", "creature", "legendary")
	gs.Seats[0].ManaPool = 3

	// Library: a 1-CMC creature (matches verse 1 after activation).
	cmc1 := &gameengine.Card{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature", "cmc:1"}}
	cmc3 := &gameengine.Card{Name: "Reclamation Sage", Owner: 0, Types: []string{"creature", "cmc:3"}}
	gs.Seats[0].Library = []*gameengine.Card{cmc1, cmc3}

	gameengine.InvokeActivatedHook(gs, yisan, 0, nil)

	if !yisan.Tapped {
		t.Errorf("Yisan should be tapped after activation")
	}
	if yisan.Counters["verse"] != 1 {
		t.Errorf("expected 1 verse counter, got %d", yisan.Counters["verse"])
	}
	// Llanowar Elves should be on battlefield.
	foundOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card == cmc1 {
			foundOnBF = true
		}
	}
	if !foundOnBF {
		t.Errorf("Llanowar Elves should be tutored onto battlefield")
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana spent, pool=%d", gs.Seats[0].ManaPool)
	}
}

func TestYisan_NoMatchingCMCWhiffs(t *testing.T) {
	gs := newGame(t, 2)
	yisan := addPerm(gs, 0, "Yisan, the Wanderer Bard", "creature", "legendary")
	gs.Seats[0].ManaPool = 3
	// Only a 5-CMC creature; activation looks for CMC 1.
	cmc5 := &gameengine.Card{Name: "Big Beast", Owner: 0, Types: []string{"creature", "cmc:5"}}
	gs.Seats[0].Library = []*gameengine.Card{cmc5}

	gameengine.InvokeActivatedHook(gs, yisan, 0, nil)

	if yisan.Counters["verse"] != 1 {
		t.Errorf("verse counter still added even on whiff, got %d", yisan.Counters["verse"])
	}
	if !yisan.Tapped {
		t.Errorf("Yisan still taps on whiff")
	}
	// Library should be intact (just shuffled).
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("library should still have the unsuitable creature, got %d", len(gs.Seats[0].Library))
	}
}

func TestYisan_InsufficientManaFails(t *testing.T) {
	gs := newGame(t, 2)
	yisan := addPerm(gs, 0, "Yisan, the Wanderer Bard", "creature", "legendary")
	gs.Seats[0].ManaPool = 2 // need 3
	addLibrary(gs, 0, "anything")

	gameengine.InvokeActivatedHook(gs, yisan, 0, nil)

	if yisan.Tapped {
		t.Errorf("Yisan should NOT tap on insufficient-mana fail")
	}
	if yisan.Counters["verse"] != 0 {
		t.Errorf("no verse counter on failed activation")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event")
	}
}

// -----------------------------------------------------------------------------
// Registry smoke
// -----------------------------------------------------------------------------

func TestBatchNR60_AllRegistered(t *testing.T) {
	if !HasActivated("Krark-Clan Ironworks") {
		t.Errorf("KCI Activated not registered")
	}
	if !HasResolve("Mystic Confluence") {
		t.Errorf("Mystic Confluence Resolve not registered")
	}
	if !HasResolve("Treasure Cruise") {
		t.Errorf("Treasure Cruise Resolve not registered")
	}
	if !HasResolve("Stifle") {
		t.Errorf("Stifle Resolve not registered")
	}
	if !HasResolve("Trickbind") {
		t.Errorf("Trickbind Resolve not registered")
	}
	if !HasActivated("Yisan, the Wanderer Bard") {
		t.Errorf("Yisan Activated not registered")
	}
}
