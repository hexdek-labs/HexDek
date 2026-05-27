package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch J (R60) — tests for 5 new handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Counterspell
// -----------------------------------------------------------------------------

func TestCounterspell_CountersAnySpell(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:  "Demonic Tutor",
			Owner: 1,
			Types: []string{"sorcery"},
		},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Counterspell", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Errorf("Counterspell should have countered the opp spell")
	}
}

func TestCounterspell_NoOpponentSpellFails(t *testing.T) {
	gs := newGame(t, 2)
	// Only our own spell on stack.
	ownSpell := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}},
	}
	gs.Stack = append(gs.Stack, ownSpell)

	card := addCard(gs, 0, "Counterspell", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if ownSpell.Countered {
		t.Errorf("Counterspell should not counter own spells")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no opp spell")
	}
}

// -----------------------------------------------------------------------------
// Skullclamp
// -----------------------------------------------------------------------------

func TestSkullclamp_DrawTwoWhenEquippedCreatureDies(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C", "D")
	clamp := addPerm(gs, 0, "Skullclamp", "artifact")
	creature := addPerm(gs, 0, "Grizzly Bears", "creature")
	creature.Card.BasePower = 2
	creature.Card.BaseToughness = 2

	// Invoke ETB so the would_die replacement is registered.
	gameengine.InvokeETBHook(gs, clamp)

	// Equip: set AttachedTo.
	clamp.AttachedTo = creature

	// Kill the creature — DestroyPermanent fires FireDieEvent which
	// triggers Skullclamp's stamp replacement; the dying-flag is then
	// read by the creature_dies trigger.
	if !gameengine.DestroyPermanent(gs, creature, nil) {
		t.Fatalf("destroy returned false")
	}

	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("Skullclamp should draw 2 when equipped creature dies, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

func TestSkullclamp_UnequippedCreatureDoesNotDraw(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C")
	clamp := addPerm(gs, 0, "Skullclamp", "artifact")
	creature := addPerm(gs, 0, "Llanowar Elves", "creature")
	creature.Card.BasePower = 1
	creature.Card.BaseToughness = 1

	gameengine.InvokeETBHook(gs, clamp)
	// Note: NOT setting AttachedTo. Clamp is not equipped.

	gameengine.DestroyPermanent(gs, creature, nil)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Skullclamp should not draw when creature dies unequipped, hand=%d",
			len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Solemn Simulacrum
// -----------------------------------------------------------------------------

func TestSolemnSimulacrum_ETBTutorsBasicLand(t *testing.T) {
	gs := newGame(t, 2)
	// Library: a spell, then a basic land.
	nonbasic := &gameengine.Card{Name: "Mountain Vista", Owner: 0, Types: []string{"land"}}
	basic := &gameengine.Card{Name: "Plains", Owner: 0, Types: []string{"basic", "land"}}
	gs.Seats[0].Library = []*gameengine.Card{nonbasic, basic}

	solemn := addPerm(gs, 0, "Solemn Simulacrum", "artifact", "creature")
	gameengine.InvokeETBHook(gs, solemn)

	// Basic land should be on battlefield tapped.
	foundLand := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Plains" {
			foundLand = true
			if !p.Tapped {
				t.Errorf("Plains should enter tapped via Solemn ETB")
			}
		}
	}
	if !foundLand {
		t.Errorf("Plains should be on battlefield after Solemn ETB")
	}
	// Non-basic still in library.
	if len(gs.Seats[0].Library) != 1 || gs.Seats[0].Library[0] != nonbasic {
		t.Errorf("non-basic should remain in library")
	}
}

func TestSolemnSimulacrum_DiesDrawsCard(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C")
	solemn := addPerm(gs, 0, "Solemn Simulacrum", "artifact", "creature")
	solemn.Card.BasePower = 2
	solemn.Card.BaseToughness = 2

	gameengine.InvokeETBHook(gs, solemn)
	preHand := len(gs.Seats[0].Hand)

	// Kill Solemn — would_die replacement fires the draw inline.
	gameengine.DestroyPermanent(gs, solemn, nil)

	postHand := len(gs.Seats[0].Hand)
	if postHand-preHand != 1 {
		t.Errorf("Solemn should draw 1 on death; pre=%d post=%d", preHand, postHand)
	}
}

// -----------------------------------------------------------------------------
// Hullbreacher
// -----------------------------------------------------------------------------

func TestHullbreacher_OpponentExtraDrawMakesTreasure(t *testing.T) {
	gs := newGame(t, 2)
	hb := addPerm(gs, 0, "Hullbreacher", "creature")
	gameengine.InvokeETBHook(gs, hb)

	// Stack the active-player flag so the "first draw" exemption is
	// already consumed. Set gs.Active to seat 1 (the opponent who'll
	// draw), pretend it's their turn N.
	gs.Active = 1
	if hb.Flags == nil {
		hb.Flags = map[string]int{}
	}
	hb.Flags["hullbreacher_normal_draw_seat_1"] = gs.Turn

	// Fire a would_draw event for seat 1.
	ev := gameengine.NewReplEvent("would_draw")
	ev.TargetSeat = 1
	ev.Payload["count"] = 1
	gameengine.FireEvent(gs, ev)

	if !ev.Cancelled {
		t.Errorf("Hullbreacher should cancel the extra draw event")
	}
	treasures := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Treasure Token" {
			treasures++
		}
	}
	if treasures != 1 {
		t.Errorf("expected 1 Treasure created for Hullbreacher controller, got %d", treasures)
	}
}

func TestHullbreacher_OwnDrawNotReplaced(t *testing.T) {
	gs := newGame(t, 2)
	hb := addPerm(gs, 0, "Hullbreacher", "creature")
	gameengine.InvokeETBHook(gs, hb)

	ev := gameengine.NewReplEvent("would_draw")
	ev.TargetSeat = 0 // controller's own draw
	ev.Payload["count"] = 1
	gameengine.FireEvent(gs, ev)

	if ev.Cancelled {
		t.Errorf("Hullbreacher should NOT replace own draws")
	}
}

func TestHullbreacher_FirstDrawPerTurnExempt(t *testing.T) {
	gs := newGame(t, 2)
	hb := addPerm(gs, 0, "Hullbreacher", "creature")
	gameengine.InvokeETBHook(gs, hb)

	// Seat 1 is active; this is their FIRST draw this turn.
	gs.Active = 1
	gs.Turn = 5

	ev := gameengine.NewReplEvent("would_draw")
	ev.TargetSeat = 1
	ev.Payload["count"] = 1
	gameengine.FireEvent(gs, ev)

	if ev.Cancelled {
		t.Errorf("first draw of active player's draw step should pass through")
	}
	treasures := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Treasure Token" {
			treasures++
		}
	}
	if treasures != 0 {
		t.Errorf("first draw exempt — no treasure should mint, got %d", treasures)
	}
}

// -----------------------------------------------------------------------------
// Pyroblast
// -----------------------------------------------------------------------------

func TestPyroblast_CountersBlueSpell(t *testing.T) {
	gs := newGame(t, 2)
	blueSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:   "Brainstorm",
			Owner:  1,
			Types:  []string{"instant"},
			Colors: []string{"U"},
		},
	}
	gs.Stack = append(gs.Stack, blueSpell)

	card := addCard(gs, 0, "Pyroblast", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !blueSpell.Countered {
		t.Errorf("Pyroblast should counter blue spell")
	}
}

func TestPyroblast_IgnoresNonBlueSpell(t *testing.T) {
	gs := newGame(t, 2)
	redSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:   "Lava Spike",
			Owner:  1,
			Types:  []string{"sorcery"},
			Colors: []string{"R"},
		},
	}
	gs.Stack = append(gs.Stack, redSpell)

	card := addCard(gs, 0, "Pyroblast", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if redSpell.Countered {
		t.Errorf("Pyroblast should not counter a red spell")
	}
}

func TestPyroblast_DestroysBluePermanentWhenNoBlueSpell(t *testing.T) {
	gs := newGame(t, 2)
	blueArtifact := addPerm(gs, 1, "Counterspell Trinket", "artifact")
	blueArtifact.Card.Colors = []string{"U"}

	card := addCard(gs, 0, "Pyroblast", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	for _, p := range gs.Seats[1].Battlefield {
		if p == blueArtifact {
			t.Errorf("blue permanent should be destroyed by Pyroblast")
		}
	}
}

func TestPyroblast_NoOpWhenNothingBlue(t *testing.T) {
	gs := newGame(t, 2)
	// Only non-blue permanents on board.
	addPerm(gs, 1, "Mountain", "land")

	card := addCard(gs, 0, "Pyroblast", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no blue target")
	}
}

func TestRedElementalBlast_SharesPyroblastHandler(t *testing.T) {
	gs := newGame(t, 2)
	blueSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:   "Counterspell",
			Owner:  1,
			Types:  []string{"instant"},
			Colors: []string{"U"},
		},
	}
	gs.Stack = append(gs.Stack, blueSpell)

	card := addCard(gs, 0, "Red Elemental Blast", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !blueSpell.Countered {
		t.Errorf("Red Elemental Blast should counter blue spell")
	}
}

// -----------------------------------------------------------------------------
// Registry smoke
// -----------------------------------------------------------------------------

func TestBatchJR60_AllRegistered(t *testing.T) {
	if !HasResolve("Counterspell") {
		t.Errorf("Counterspell resolve not registered")
	}
	if !HasETB("Skullclamp") {
		t.Errorf("Skullclamp ETB not registered")
	}
	if !HasTrigger("Skullclamp", "creature_dies") {
		t.Errorf("Skullclamp dies trigger not registered")
	}
	if !HasETB("Solemn Simulacrum") {
		t.Errorf("Solemn Simulacrum ETB not registered")
	}
	if !HasETB("Hullbreacher") {
		t.Errorf("Hullbreacher ETB not registered")
	}
	if !HasResolve("Pyroblast") {
		t.Errorf("Pyroblast resolve not registered")
	}
	if !HasResolve("Red Elemental Blast") {
		t.Errorf("Red Elemental Blast resolve not registered")
	}
}
