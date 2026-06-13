package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// glcover_damage_life_r63_test.go — regression pins for the r63 shard
// G-L DOA coverage batch: four "deals damage equal to a count" burn
// spells (Goblin War Strike, Gaze of Adamaro, Ire of Kaminari,
// Hidetsugu's Second Rite) plus one ETB life-gain (Luminollusk). Each
// previously parsed to an inert raw-text node and did NOTHING.

// --- Goblin War Strike ------------------------------------------------

func TestGoblinWarStrike_DamageEqualsGoblinCount(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 40
	addPerm(gs, 0, "Goblin Token A", "creature", "goblin")
	addPerm(gs, 0, "Goblin Token B", "creature", "goblin")
	addPerm(gs, 0, "Mountain Yeti", "creature") // not a goblin
	card := addCard(gs, 0, "Goblin War Strike", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if gs.Seats[1].Life != 38 {
		t.Errorf("opponent life %d, want 38 (-2 = two goblins)", gs.Seats[1].Life)
	}
}

func TestGoblinWarStrike_NoGoblinsNoDamage(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 40
	card := addCard(gs, 0, "Goblin War Strike", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if gs.Seats[1].Life != 40 {
		t.Errorf("opponent life %d, want 40 (no goblins → no damage)", gs.Seats[1].Life)
	}
}

// --- Gaze of Adamaro --------------------------------------------------

func TestGazeOfAdamaro_DamageEqualsTargetHand(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 40
	gs.Seats[1].Hand = []*gameengine.Card{
		{Name: "A", Owner: 1}, {Name: "B", Owner: 1},
		{Name: "C", Owner: 1}, {Name: "D", Owner: 1},
	}
	card := addCard(gs, 0, "Gaze of Adamaro", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if gs.Seats[1].Life != 36 {
		t.Errorf("opponent life %d, want 36 (-4 = hand size)", gs.Seats[1].Life)
	}
}

// --- Ire of Kaminari --------------------------------------------------

func TestIreOfKaminari_DamageEqualsArcaneInGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 40
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Glacial Ray", Owner: 0, Types: []string{"instant", "arcane"}},
		{Name: "Kodama's Reach", Owner: 0, Types: []string{"sorcery", "arcane"}},
		{Name: "Plains", Owner: 0, Types: []string{"land"}}, // not arcane
	}
	card := addCard(gs, 0, "Ire of Kaminari", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if gs.Seats[1].Life != 38 {
		t.Errorf("opponent life %d, want 38 (-2 = arcane in graveyard)", gs.Seats[1].Life)
	}
}

func TestIreOfKaminari_DetectsArcaneFromTypeLine(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 40
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Glacial Ray", Owner: 0, TypeLine: "Instant — Arcane"},
	}
	card := addCard(gs, 0, "Ire of Kaminari", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if gs.Seats[1].Life != 39 {
		t.Errorf("opponent life %d, want 39 (-1 arcane via type line)", gs.Seats[1].Life)
	}
}

// --- Hidetsugu's Second Rite ------------------------------------------

func TestHidetsugusSecondRite_DealsTenAtExactlyTen(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 10
	card := addCard(gs, 0, "Hidetsugu's Second Rite", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if gs.Seats[1].Life != 0 {
		t.Errorf("opponent life %d, want 0 (10 damage at exactly 10 life)", gs.Seats[1].Life)
	}
}

func TestHidetsugusSecondRite_NoEffectOffTen(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 11
	card := addCard(gs, 0, "Hidetsugu's Second Rite", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if gs.Seats[1].Life != 11 {
		t.Errorf("opponent life %d, want 11 (not exactly 10 → no effect)", gs.Seats[1].Life)
	}
}

// --- Luminollusk ------------------------------------------------------

func TestLuminollusk_GainsLifePerColor(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40
	// Three distinct colors among controlled permanents (W, U, plus R twice).
	wCard := addPerm(gs, 0, "White Thing", "creature")
	wCard.Card.Colors = []string{"W"}
	uCard := addPerm(gs, 0, "Blue Thing", "creature")
	uCard.Card.Colors = []string{"U"}
	r1 := addPerm(gs, 0, "Red Thing", "creature")
	r1.Card.Colors = []string{"R"}
	r2 := addPerm(gs, 0, "Red Thing 2", "creature")
	r2.Card.Colors = []string{"R"}
	colorless := addPerm(gs, 0, "Rock", "artifact") // no colors
	_ = colorless

	perm := addPerm(gs, 0, "Luminollusk", "artifact", "creature")
	gameengine.InvokeETBHook(gs, perm)
	if gs.Seats[0].Life != 43 {
		t.Errorf("caster life %d, want 43 (+3 distinct colors W/U/R)", gs.Seats[0].Life)
	}
}

func TestLuminollusk_NoColorsNoGain(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40
	rock := addPerm(gs, 0, "Rock", "artifact")
	rock.Card.Colors = nil
	perm := addPerm(gs, 0, "Luminollusk", "artifact", "creature")
	gameengine.InvokeETBHook(gs, perm)
	if gs.Seats[0].Life != 40 {
		t.Errorf("caster life %d, want 40 (no colored permanents → no gain)", gs.Seats[0].Life)
	}
}
