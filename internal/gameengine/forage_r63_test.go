package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 — Forage (CR §701.61, Bloomburrow). Before this pass every forage
// surface was inert: the generic AST forage modification, the
// paid_optional_cost("forage") conditional ("you may forage. If you do,
// X"), and the "{cost}, Forage:" activated-ability cost all logged a
// no-op, so no cards left the graveyard and the gated effect never fired.

// fillGraveyard (shared helper, keywords_threshold_test.go) appends n
// synthetic cards to seat's graveyard.

func countForageEvents(gs *GameState) int {
	c := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "forage" {
			c++
		}
	}
	return c
}

// (4a) exile-three path: exactly three cards move graveyard → exile
// (zone conservation — they are NOT lost), the forage event fires.
func TestForage_ExileThreePath_ZoneConservation(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	fillGraveyard(gs, 0, 5)

	before := len(gs.Seats[0].Graveyard) + len(gs.Seats[0].Exile)
	if !Forage(gs, 0, "Test Source") {
		t.Fatal("Forage should succeed with 5 graveyard cards and no Food")
	}
	if got := len(gs.Seats[0].Graveyard); got != 2 {
		t.Fatalf("graveyard: want 2 after exiling 3, got %d", got)
	}
	if got := len(gs.Seats[0].Exile); got != 3 {
		t.Fatalf("exile: want 3 foraged cards, got %d", got)
	}
	if after := len(gs.Seats[0].Graveyard) + len(gs.Seats[0].Exile); after != before {
		t.Fatalf("zone conservation: gy+exile changed %d -> %d (cards lost)", before, after)
	}
	if countForageEvents(gs) != 1 {
		t.Fatalf("want exactly one forage event, got %d", countForageEvents(gs))
	}
}

// (4b) sacrifice-a-Food path: with a Food present it is preferred, the
// graveyard is untouched, the Food leaves the battlefield.
func TestForage_SacrificeFoodPath(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	fillGraveyard(gs, 0, 5)
	CreateFoodToken(gs, 0)
	if findForageFood(gs, 0) == nil {
		t.Fatal("setup: Food token should be on the battlefield")
	}
	bfBefore := len(gs.Seats[0].Battlefield)

	if !Forage(gs, 0, "Test Source") {
		t.Fatal("Forage should succeed with a Food present")
	}
	if findForageFood(gs, 0) != nil {
		t.Fatal("Food should have been sacrificed")
	}
	if len(gs.Seats[0].Battlefield) != bfBefore-1 {
		t.Fatalf("battlefield should shrink by the sacrificed Food: %d -> %d", bfBefore, len(gs.Seats[0].Battlefield))
	}
	// Graveyard untouched: sac-Food was preferred over exiling cards.
	if got := len(gs.Seats[0].Graveyard); got != 5 {
		t.Fatalf("graveyard should be untouched on the Food path, want 5 got %d", got)
	}
	if countForageEvents(gs) != 1 {
		t.Fatalf("want exactly one forage event, got %d", countForageEvents(gs))
	}
}

// (3) the OR choice is enforced: fewer than three graveyard cards AND no
// Food → cannot forage; Forage pays nothing and returns false.
func TestForage_CannotPayEitherWay(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	fillGraveyard(gs, 0, 2) // < 3, no Food

	if CanForage(gs, 0) {
		t.Fatal("CanForage should be false with 2 gy cards and no Food")
	}
	if Forage(gs, 0, "Test Source") {
		t.Fatal("Forage should fail (return false) when unpayable")
	}
	if got := len(gs.Seats[0].Graveyard); got != 2 {
		t.Fatalf("graveyard must be untouched when forage fails, got %d", got)
	}
	if len(gs.Seats[0].Exile) != 0 {
		t.Fatal("nothing should be exiled when forage fails")
	}
	if countForageEvents(gs) != 0 {
		t.Fatal("no forage event should fire when forage is unpayable")
	}
}

// (2) the gated effect happens only when forage is paid: the
// paid_optional_cost("forage") conditional ("you may forage. If you do,
// draw a card") draws when payable and does nothing when not.
func TestForage_PaidOptionalCostGatesEffect(t *testing.T) {
	mkConditional := func() *gameast.Conditional {
		return &gameast.Conditional{
			Condition: &gameast.Condition{
				Kind: "paid_optional_cost",
				Args: []interface{}{"forage"},
			},
			Body: &gameast.Draw{
				Count:  gameast.NumberOrRef{IsInt: true, Int: 1},
				Target: gameast.Filter{Base: "self"},
			},
		}
	}

	// Payable: 3 graveyard cards → forage paid → draw happens.
	t.Run("payable_draws", func(t *testing.T) {
		gs := NewGameState(2, nil, nil)
		fillGraveyard(gs, 0, 3)
		addLibrary(gs, 0, "Top Card")
		src := addBattlefield(gs, 0, "Treetop Sentries", 2, 2, "creature")

		ResolveEffect(gs, src, mkConditional())

		if len(gs.Seats[0].Hand) != 1 {
			t.Fatalf("expected to draw 1 card after foraging, hand=%d", len(gs.Seats[0].Hand))
		}
		if len(gs.Seats[0].Graveyard) != 0 {
			t.Fatalf("3 gy cards should have been foraged away, got %d", len(gs.Seats[0].Graveyard))
		}
	})

	// Unpayable: 1 graveyard card, no Food → forage not paid → no draw.
	t.Run("unpayable_no_draw", func(t *testing.T) {
		gs := NewGameState(2, nil, nil)
		fillGraveyard(gs, 0, 1)
		addLibrary(gs, 0, "Top Card")
		src := addBattlefield(gs, 0, "Treetop Sentries", 2, 2, "creature")

		ResolveEffect(gs, src, mkConditional())

		if len(gs.Seats[0].Hand) != 0 {
			t.Fatalf("no draw should happen when forage is unpayable, hand=%d", len(gs.Seats[0].Hand))
		}
		if len(gs.Seats[0].Graveyard) != 1 {
			t.Fatalf("graveyard must be untouched when forage is unpayable, got %d", len(gs.Seats[0].Graveyard))
		}
	})
}

// (1) forage wired as a real activated-ability cost with the OR choice:
// "Forage: draw" pays the forage cost (exiles three from gy) when
// activated, and rejects the activation when forage is unpayable.
func TestForage_ActivatedAbilityCost(t *testing.T) {
	mkForageCard := func(gs *GameState, seat int) *Permanent {
		card := &Card{
			Name:          "Thornvault Forager",
			Owner:         seat,
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 1, // non-zero so SBA §704.5f doesn't bin the source
			AST: &gameast.CardAST{
				Name: "Thornvault Forager",
				Abilities: []gameast.Ability{
					&gameast.Activated{
						Cost:   gameast.Cost{Extra: []string{"forage"}},
						Effect: &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "self"}},
					},
				},
			},
		}
		perm := &Permanent{
			Card:       card,
			Controller: seat,
			Owner:      seat,
			Timestamp:  gs.NextTimestamp(),
			Counters:   map[string]int{},
			Flags:      map[string]int{},
		}
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
		return perm
	}

	t.Run("pays_forage_when_affordable", func(t *testing.T) {
		gs := NewGameState(2, nil, nil)
		fillGraveyard(gs, 0, 5)
		src := mkForageCard(gs, 0)

		if err := ActivateAbility(gs, 0, src, 0, nil); err != nil {
			t.Fatalf("ActivateAbility should succeed, got %v", err)
		}
		// Forage cost paid during activation: 3 gy cards exiled.
		if got := len(gs.Seats[0].Graveyard); got != 2 {
			t.Fatalf("forage cost should have exiled 3 gy cards, gy=%d", got)
		}
		if got := len(gs.Seats[0].Exile); got != 3 {
			t.Fatalf("3 foraged cards should be in exile, got %d", got)
		}
		if countForageEvents(gs) != 1 {
			t.Fatalf("want one forage event from the activated cost, got %d", countForageEvents(gs))
		}
	})

	t.Run("rejected_when_unaffordable", func(t *testing.T) {
		gs := NewGameState(2, nil, nil)
		fillGraveyard(gs, 0, 2) // < 3, no Food
		src := mkForageCard(gs, 0)

		err := ActivateAbility(gs, 0, src, 0, nil)
		if err == nil {
			t.Fatal("ActivateAbility should reject when forage is unaffordable")
		}
		if got := len(gs.Seats[0].Graveyard); got != 2 {
			t.Fatalf("graveyard must be untouched on a rejected activation, gy=%d", got)
		}
		if len(gs.Seats[0].Exile) != 0 {
			t.Fatal("nothing should be exiled on a rejected activation")
		}
	})
}
