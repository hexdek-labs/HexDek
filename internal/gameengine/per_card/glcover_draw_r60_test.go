package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// glcover_draw_r60_test.go — regression pins for the shard G-L
// DrawN-powered draw cards (Lucid Dreams, Lock and Load, Inspired
// Ultimatum) plus a direct test of the shared gameengine.DrawN
// primitive. Each card previously parsed to an inert raw-text node and
// drew ZERO cards.

func fillLibrary(gs *gameengine.GameState, seat, n int) {
	for i := 0; i < n; i++ {
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, &gameengine.Card{
			Name: "Filler", Owner: seat,
		})
	}
}

// --- DrawN primitive --------------------------------------------------

func TestDrawN_DrawsRequestedCount(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 10)
	preHand := len(gs.Seats[0].Hand)

	drawn := gameengine.DrawN(gs, 0, 3, nil)

	if drawn != 3 {
		t.Errorf("DrawN returned %d, want 3", drawn)
	}
	if got := len(gs.Seats[0].Hand) - preHand; got != 3 {
		t.Errorf("hand grew by %d, want 3", got)
	}
	if got := len(gs.Seats[0].Library); got != 7 {
		t.Errorf("library has %d, want 7", got)
	}
}

func TestDrawN_StopsAtEmptyLibrary(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 2)
	drawn := gameengine.DrawN(gs, 0, 5, nil)
	if drawn != 2 {
		t.Errorf("DrawN returned %d, want 2 (library exhausted)", drawn)
	}
}

func TestDrawN_NonPositiveIsNoOp(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 5)
	if got := gameengine.DrawN(gs, 0, 0, nil); got != 0 {
		t.Errorf("DrawN(0) returned %d, want 0", got)
	}
	if len(gs.Seats[0].Library) != 5 {
		t.Errorf("library mutated on no-op draw")
	}
}

// --- Lucid Dreams -----------------------------------------------------

func TestLucidDreams_DrawsPerCardTypeInGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 10)
	// Graveyard with 4 distinct card types (and a duplicate creature).
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Bear", Owner: 0, Types: []string{"creature"}},
		{Name: "Bolt", Owner: 0, Types: []string{"instant"}},
		{Name: "Wrath", Owner: 0, Types: []string{"sorcery"}},
		{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}},
		{Name: "Elf", Owner: 0, Types: []string{"creature"}}, // dup type
		{Name: "Forest", Owner: 0, Types: []string{"basic", "land"}},
	}
	preHand := len(gs.Seats[0].Hand)

	card := addCard(gs, 0, "Lucid Dreams", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	// 5 distinct canonical types: creature, instant, sorcery, artifact, land.
	if got := len(gs.Seats[0].Hand) - preHand; got != 5 {
		t.Errorf("drew %d cards, want 5 (distinct card types)", got)
	}
}

// --- Lock and Load ----------------------------------------------------

func TestLockAndLoad_DrawsOnePlusOtherInstSorc(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 10)
	gs.Seats[0].Turn.Casts = []gameengine.CastRecord{
		{CardName: "Opt", Types: []string{"instant"}},
		{CardName: "Ponder", Types: []string{"sorcery"}},
		{CardName: "Llanowar Elves", Types: []string{"creature"}}, // not counted
		{CardName: "Lock and Load", Types: []string{"sorcery"}},   // self, excluded
	}
	preHand := len(gs.Seats[0].Hand)

	card := addCard(gs, 0, "Lock and Load", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	// 1 (base) + 2 other instant/sorcery = 3.
	if got := len(gs.Seats[0].Hand) - preHand; got != 3 {
		t.Errorf("drew %d cards, want 3 (1 + 2 other inst/sorc)", got)
	}
}

// --- Inspired Ultimatum -----------------------------------------------

func TestInspiredUltimatum_LifeDamageDraw(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 30
	gs.Seats[1].Life = 30
	fillLibrary(gs, 0, 10)
	preHand := len(gs.Seats[0].Hand)

	card := addCard(gs, 0, "Inspired Ultimatum", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	if gs.Seats[0].Life != 35 {
		t.Errorf("caster life %d, want 35 (+5)", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 25 {
		t.Errorf("opponent life %d, want 25 (-5 damage)", gs.Seats[1].Life)
	}
	if got := len(gs.Seats[0].Hand) - preHand; got != 5 {
		t.Errorf("drew %d cards, want 5", got)
	}
}
