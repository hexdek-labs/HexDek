package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// CR §702.94 — Miracle audit / regression suite (dev/miracle-702-94-r63).
//
// Verifies: (a) the first-card-drawn-this-turn condition (tracked per turn,
// reset each turn, only the FIRST draw qualifies), (b) reveal-on-draw opens
// the window, (c) the triggered ability grants the alternative miracle cost
// via the canonical alt-cost cast path, and (d) it does NOT trigger on cards
// drawn beyond the first or via non-draw means — including the opponent-turn
// instant-speed first-draw case.

func miracleSpell(name string, cost int) *Card {
	return &Card{
		Name:  name,
		CMC:   6,
		Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "miracle", Args: []interface{}{float64(cost)}},
			},
		},
	}
}

func plainSpell(name string) *Card {
	return &Card{
		Name:  name,
		CMC:   3,
		Types: []string{"sorcery"},
		AST:   &gameast.CardAST{Name: name},
	}
}

func newMiracleGame(t *testing.T, seats int) *GameState {
	t.Helper()
	gs := NewGameState(seats, rand.New(rand.NewSource(42)), nil)
	gs.Turn = 1
	gs.Active = 0
	gs.Phase = "main"
	return gs
}

func miracleCountEvents(gs *GameState, kind string) int {
	n := 0
	for _, e := range gs.EventLog {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// (a)+(b)+(c): a miracle card drawn as the FIRST card this turn opens the
// window (reveal) and is castable for its miracle cost (not its mana cost).
func TestMiracle_FirstDrawCastableForMiracleCost(t *testing.T) {
	gs := newMiracleGame(t, 2)
	gs.Seats[0].ManaPool = 5
	card := miracleSpell("Terminus", 1)
	gs.Seats[0].Library = []*Card{card}

	drawn, ok := gs.drawOne(0)
	if !ok || drawn != card {
		t.Fatalf("expected to draw the miracle card, ok=%v", ok)
	}
	if !MiracleWindowOpen(gs, card) {
		t.Fatal("first-drawn miracle card should open its miracle window (reveal, §702.94a/b)")
	}
	if miracleCountEvents(gs, "miracle_revealed") != 1 {
		t.Errorf("expected one miracle_revealed event, got %d", miracleCountEvents(gs, "miracle_revealed"))
	}
	if !CanCastMiracle(gs, 0, card) {
		t.Fatal("CanCastMiracle should be true for first-drawn miracle card in hand")
	}

	if err := CastWithMiracle(gs, 0, card, nil); err != nil {
		t.Fatalf("miracle cast should succeed: %v", err)
	}
	// Paid the miracle cost (1), not the mana cost (CMC 6).
	if gs.Seats[0].ManaPool != 4 {
		t.Errorf("should have paid 1 (miracle), mana pool=%d (want 4)", gs.Seats[0].ManaPool)
	}
	// CastSpellWithCosts is the engine's cast+resolve path (PushStackItem →
	// PriorityRound → DrainStack), so the sorcery resolves and lands in the
	// graveyard rather than sitting on the stack.
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Error("miracle card should have left hand on cast")
		}
	}
	if miracleCountEvents(gs, "miracle") != 1 {
		t.Errorf("expected one miracle event, got %d", miracleCountEvents(gs, "miracle"))
	}
	inGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			inGY = true
		}
	}
	if !inGY {
		t.Error("resolved miracle sorcery should be in the graveyard")
	}
	// Window must close after casting so the same draw can't be miracled twice.
	if CanCastMiracle(gs, 0, card) {
		t.Error("window should be closed after a successful miracle cast")
	}
}

// (d) — a miracle card drawn SECOND is not eligible.
func TestMiracle_SecondDrawNotEligible(t *testing.T) {
	gs := newMiracleGame(t, 2)
	gs.Seats[0].ManaPool = 5
	miracle := miracleSpell("Bonfire of the Damned", 2)
	// Top of library drawn first: a plain card, then the miracle card.
	gs.Seats[0].Library = []*Card{plainSpell("Opt"), miracle}

	if _, ok := gs.drawOne(0); !ok { // first draw: plain
		t.Fatal("first draw failed")
	}
	if _, ok := gs.drawOne(0); !ok { // second draw: miracle
		t.Fatal("second draw failed")
	}
	if MiracleWindowOpen(gs, miracle) {
		t.Error("miracle window must NOT open for a card drawn second this turn")
	}
	if CanCastMiracle(gs, 0, miracle) {
		t.Error("CanCastMiracle must be false for a second-drawn miracle card")
	}
	if err := CastWithMiracle(gs, 0, miracle, nil); err == nil {
		t.Error("CastWithMiracle should fail for an ineligible (second-drawn) card")
	}
}

// (d) — a miracle card that enters hand by a NON-draw means is not eligible.
func TestMiracle_NonDrawHandEntryNotEligible(t *testing.T) {
	gs := newMiracleGame(t, 2)
	card := miracleSpell("Temporal Mastery", 2)
	// Placed directly into hand (e.g. tutor/return-to-hand) — never drawn.
	gs.Seats[0].Hand = []*Card{card}

	if MiracleWindowOpen(gs, card) {
		t.Error("non-drawn miracle card must not have an open window")
	}
	if CanCastMiracle(gs, 0, card) {
		t.Error("CanCastMiracle must be false for a non-drawn card (§702.94a — only on draw)")
	}
}

// The opponent-turn case: a player's FIRST card drawn during the current
// turn qualifies even when it's an opponent's turn (instant-speed draw).
// This pins the root fix — the per-seat miracle counter is reset for ALL
// seats at every turn start, not just the active player's.
func TestMiracle_OpponentTurnFirstDrawEligible(t *testing.T) {
	gs := newMiracleGame(t, 2)
	gs.Seats[1].ManaPool = 5
	card := miracleSpell("Entreat the Angels", 3)
	gs.Seats[1].Library = []*Card{card}

	// Seat 1 already drew during their own previous turn — stale counter.
	if gs.Seats[1].Flags == nil {
		gs.Seats[1].Flags = map[string]int{}
	}
	gs.Seats[1].Flags["miracle_draws_this_turn"] = 4

	// Opponent (seat 0) begins their turn: UntapAll resets the miracle
	// counter for EVERY seat, including seat 1.
	gs.Turn = 2
	gs.Active = 0
	UntapAll(gs, 0)
	if gs.Seats[1].Flags["miracle_draws_this_turn"] != 0 {
		t.Fatalf("turn start should reset seat 1's miracle counter to 0, got %d",
			gs.Seats[1].Flags["miracle_draws_this_turn"])
	}

	// Seat 1 draws its first card of the (opponent's) turn at instant speed.
	if _, ok := gs.drawOne(1); !ok {
		t.Fatal("instant-speed draw failed")
	}
	if !MiracleWindowOpen(gs, card) {
		t.Fatal("first card drawn this turn should open the window even on an opponent's turn")
	}
	if !CanCastMiracle(gs, 1, card) {
		t.Fatal("non-active seat should be able to miracle its first card drawn this turn")
	}
}

// The window resets/expires each turn: a card drawn first on turn N is no
// longer miracle-eligible once the game advances to a later turn.
func TestMiracle_WindowExpiresNextTurn(t *testing.T) {
	gs := newMiracleGame(t, 2)
	card := miracleSpell("Devastation Tide", 3)
	gs.Seats[0].Library = []*Card{card}

	if _, ok := gs.drawOne(0); !ok {
		t.Fatal("draw failed")
	}
	if !CanCastMiracle(gs, 0, card) {
		t.Fatal("should be eligible the turn it was drawn")
	}
	// Advance to the next turn (seat 0's untap resets, gs.Turn advances).
	gs.Turn = 2
	UntapAll(gs, 0)
	if CanCastMiracle(gs, 0, card) {
		t.Error("miracle window must expire at end of the turn it was drawn (§702.94c)")
	}
}

// The per-turn counter resets so a fresh first-draw next turn qualifies even
// after multiple draws the prior turn.
func TestMiracle_ResetsAllowNextTurnFirstDraw(t *testing.T) {
	gs := newMiracleGame(t, 2)
	// Two draws this turn (no miracle).
	gs.Seats[0].Library = []*Card{plainSpell("a"), plainSpell("b")}
	gs.drawOne(0)
	gs.drawOne(0)
	if gs.Seats[0].Flags["miracle_draws_this_turn"] != 2 {
		t.Fatalf("expected counter 2, got %d", gs.Seats[0].Flags["miracle_draws_this_turn"])
	}
	// New turn: counter resets; a miracle card drawn first now qualifies.
	gs.Turn = 2
	UntapAll(gs, 0)
	card := miracleSpell("Thunderous Wrath", 1)
	gs.Seats[0].Library = []*Card{card}
	gs.drawOne(0)
	if !CanCastMiracle(gs, 0, card) {
		t.Error("first card drawn on a new turn should be miracle-eligible after the reset")
	}
}
