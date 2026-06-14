package gameengine

// CR §702.66 — Delve: "Each card you exile from your graveyard while
// casting this spell pays for {1}." Regression for the r63 convoke/delve
// mechanic probe: delve was entirely unwired (HasDelve / DelveMaxReduction
// existed but no cost path consulted them), so every delve spell paid full
// price. Delve is a GENERIC-only reducer and must floor at the colored-pip
// requirement per CR §601.2f-h — it can never pay a colored/colorless pip.

import (
	"strconv"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func dl_delveCard(name, manaCost string, cmc int) *Card {
	return &Card{
		Name: name, Types: []string{"sorcery", "cost:" + strconv.Itoa(cmc)},
		ManaCostString: manaCost,
		AST: &gameast.CardAST{Name: name,
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "delve", Raw: "delve"}}},
	}
}

func dl_fillGraveyard(gs *GameState, seat, n int) {
	for i := 0; i < n; i++ {
		gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, &Card{Name: "GY", Owner: seat})
	}
}

// (c): each graveyard card reduces generic by 1.
func TestDelve_ReducesGenericByGraveyardCount(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	dl_fillGraveyard(gs, 0, 5)
	// Treasure Cruise {7}{U}, CMC 8, colored floor {U} = 1.
	tc := dl_delveCard("Treasure Cruise", "{7}{U}", 8)
	if got := CalculateTotalCost(gs, tc, 0); got != 3 {
		t.Errorf("delve with 5 GY cards: {7}{U} → want 3 (8 -5), got %d", got)
	}
	// Empty the graveyard → no reduction, full price.
	gs.Seats[0].Graveyard = nil
	if got := CalculateTotalCost(gs, tc, 0); got != 8 {
		t.Errorf("delve with empty GY: want full 8, got %d", got)
	}
}

// (c)/(d): delve is generic-only — it CANNOT pay the colored {U} pip even
// with a huge graveyard.
func TestDelve_CannotPayColoredPip(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	dl_fillGraveyard(gs, 0, 20) // more than enough to zero the generic
	tc := dl_delveCard("Treasure Cruise", "{7}{U}", 8)
	if got := CalculateTotalCost(gs, tc, 0); got != 1 {
		t.Errorf("delve floors at colored {U}=1, got %d", got)
	}
	// Murderous Cut {4}{B}: 20 GY → floors at {B} = 1.
	cut := dl_delveCard("Murderous Cut", "{4}{B}", 5)
	if got := CalculateTotalCost(gs, cut, 0); got != 1 {
		t.Errorf("Murderous Cut delve floors at {B}=1, got %d", got)
	}
}

// (d): delve composes with other cost changes per §601.2f — increases
// apply first, then the (generic) delve reduction floored at colored pips.
func TestDelve_ComposesWithIncreaseAndFloor(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	dl_fillGraveyard(gs, 0, 4)
	// Sphere of Resistance (+1 to all spells) controlled by an opponent.
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, &Permanent{
		Card: &Card{Name: "Sphere of Resistance", Types: []string{"artifact"}}, Controller: 1, Owner: 1,
	})
	// Tombstalker {6}{B}{B} (CMC 8, colored floor {B}{B}=2): +1 Sphere → 9,
	// -4 delve → 5.
	ts := dl_delveCard("Tombstalker", "{6}{B}{B}", 8)
	if got := CalculateTotalCost(gs, ts, 0); got != 5 {
		t.Errorf("Tombstalker +Sphere -4 delve: want 5, got %d", got)
	}
	// With a 10-card graveyard the generic is exhausted and delve floors at
	// the {B}{B} colored requirement (Sphere +1 then floored): 9 -7 generic
	// available, but floor is 2 → 2.
	gs.Seats[0].Graveyard = nil
	dl_fillGraveyard(gs, 0, 10)
	if got := CalculateTotalCost(gs, ts, 0); got != 2 {
		t.Errorf("Tombstalker +Sphere -10 delve floors at {B}{B}=2, got %d", got)
	}
}

// All-generic delve spell reduces fully to 0 (no colored pips to protect).
func TestDelve_AllGenericReducesToZero(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	dl_fillGraveyard(gs, 0, 6)
	// A hypothetical {5} delve sorcery.
	gen := dl_delveCard("Generic Delve", "{5}", 5)
	if got := CalculateTotalCost(gs, gen, 0); got != 0 {
		t.Errorf("all-generic {5} delve with 6 GY: want 0, got %d", got)
	}
}
