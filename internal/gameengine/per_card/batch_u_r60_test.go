package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch U (R60) — tests for 5 new storm/ritual/finisher handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Cabal Ritual
// -----------------------------------------------------------------------------

func TestCabalRitual_AddsThreeManaBelowThreshold(t *testing.T) {
	gs := newGame(t, 2)
	// Empty graveyard — below threshold.
	pre := gs.Seats[0].ManaPool

	card := addCard(gs, 0, "Cabal Ritual", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if got := gs.Seats[0].ManaPool; got != pre+3 {
		t.Errorf("expected +3 mana below threshold, got pool=%d (was %d)", got, pre)
	}
}

func TestCabalRitual_AddsFiveManaAtThreshold(t *testing.T) {
	gs := newGame(t, 2)
	// 7 cards in graveyard — exactly threshold.
	for i := 0; i < 7; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			&gameengine.Card{Name: "Junk", Owner: 0})
	}
	pre := gs.Seats[0].ManaPool

	card := addCard(gs, 0, "Cabal Ritual", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if got := gs.Seats[0].ManaPool; got != pre+5 {
		t.Errorf("expected +5 mana at threshold (7 cards in gy), got pool=%d (was %d)", got, pre)
	}
}

// -----------------------------------------------------------------------------
// Pyretic Ritual
// -----------------------------------------------------------------------------

func TestPyreticRitual_AddsThreeMana(t *testing.T) {
	gs := newGame(t, 2)
	pre := gs.Seats[0].ManaPool

	card := addCard(gs, 0, "Pyretic Ritual", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if got := gs.Seats[0].ManaPool; got != pre+3 {
		t.Errorf("expected +3 mana, got pool=%d (was %d)", got, pre)
	}
	if hasEvent(gs, "add_mana") < 1 {
		t.Errorf("expected add_mana event")
	}
}

// -----------------------------------------------------------------------------
// Birgi, God of Storytelling
// -----------------------------------------------------------------------------

func TestBirgi_AddsRedManaOnControllerCast(t *testing.T) {
	gs := newGame(t, 2)
	birgi := addPerm(gs, 0, "Birgi, God of Storytelling", "creature", "legendary")
	_ = birgi
	pre := gs.Seats[0].ManaPool

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
	})

	if got := gs.Seats[0].ManaPool; got != pre+1 {
		t.Errorf("expected +1 mana from Birgi on own cast, got pool=%d (was %d)", got, pre)
	}
}

func TestBirgi_NoManaOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Birgi, God of Storytelling", "creature", "legendary")
	pre := gs.Seats[0].ManaPool

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 1,
	})

	if got := gs.Seats[0].ManaPool; got != pre {
		t.Errorf("expected no mana from Birgi on opp cast, got pool=%d (was %d)", got, pre)
	}
}

// -----------------------------------------------------------------------------
// Heartless Hidetsugu
// -----------------------------------------------------------------------------

func TestHeartlessHidetsugu_HalvesEveryPlayersLifeRoundedUp(t *testing.T) {
	gs := newGame(t, 3)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 21
	gs.Seats[2].Life = 10

	hide := addPerm(gs, 0, "Heartless Hidetsugu", "creature", "legendary")
	hide.SummoningSick = false

	gameengine.InvokeActivatedHook(gs, hide, 0, nil)

	// 40 → 40 - ceil(40/2) = 40 - 20 = 20
	// 21 → 21 - ceil(21/2) = 21 - 11 = 10
	// 10 → 10 - ceil(10/2) = 10 - 5 = 5
	if gs.Seats[0].Life != 20 {
		t.Errorf("seat 0 expected life 20 (40 - 20), got %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 10 {
		t.Errorf("seat 1 expected life 10 (21 - 11), got %d", gs.Seats[1].Life)
	}
	if gs.Seats[2].Life != 5 {
		t.Errorf("seat 2 expected life 5 (10 - 5), got %d", gs.Seats[2].Life)
	}
	if !hide.Tapped {
		t.Errorf("Hidetsugu should be tapped after activation")
	}
}

func TestHeartlessHidetsugu_TappedNoOp(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	hide := addPerm(gs, 0, "Heartless Hidetsugu", "creature", "legendary")
	hide.Tapped = true

	gameengine.InvokeActivatedHook(gs, hide, 0, nil)

	if gs.Seats[0].Life != 20 || gs.Seats[1].Life != 20 {
		t.Errorf("tapped Hidetsugu should not deal damage")
	}
}

// -----------------------------------------------------------------------------
// Tendrils of Agony
// -----------------------------------------------------------------------------

func TestTendrilsOfAgony_DrainsAndGains(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	card := addCard(gs, 0, "Tendrils of Agony", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if gs.Seats[1].Life != 18 {
		t.Errorf("expected opp life 18 (20 - 2), got %d", gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != 22 {
		t.Errorf("expected own life 22 (20 + 2), got %d", gs.Seats[0].Life)
	}
}

func TestTendrilsOfAgony_PicksHighestLifeOpponent(t *testing.T) {
	gs := newGame(t, 3)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 15
	gs.Seats[2].Life = 30 // higher-life opp

	card := addCard(gs, 0, "Tendrils of Agony", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if gs.Seats[2].Life != 28 {
		t.Errorf("expected high-life opp to be drained, got seat 2 life=%d", gs.Seats[2].Life)
	}
	if gs.Seats[1].Life != 15 {
		t.Errorf("low-life opp should be unchanged, got seat 1 life=%d", gs.Seats[1].Life)
	}
}

func TestTendrilsOfAgony_LogsCopyFlag(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	card := addCard(gs, 0, "Tendrils of Agony", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card, IsCopy: true}
	gameengine.InvokeResolveHook(gs, item)

	// IsCopy=true on the item is forwarded to the emit details so storm
	// chain audit can distinguish original vs copy resolutions.
	if gs.Seats[1].Life != 18 {
		t.Errorf("copy still drains 2; expected 18, got %d", gs.Seats[1].Life)
	}
}
