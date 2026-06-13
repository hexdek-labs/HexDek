package gameengine

import (
	"testing"
)

func stockLibrary(gs *GameState, seat, n int) {
	for i := 0; i < n; i++ {
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, &Card{Name: "Filler", Owner: seat})
	}
}

// "each player mills N cards" — previously inert parsed_tail; now resolves a
// symmetric mill via resolveResidualByText.

func TestEachPlayerMill_AllPlayers(t *testing.T) {
	gs := newTestGame(t, 3)
	for s := 0; s < 3; s++ {
		stockLibrary(gs, s, 10)
	}
	src := addTestPerm(gs, 0, "Chill of Foreboding", "sorcery")

	if !resolveResidualByText(gs, src, "each player mills five cards") {
		t.Fatalf("expected the each-player mill tail to be recognized")
	}
	for s := 0; s < 3; s++ {
		if len(gs.Seats[s].Graveyard) != 5 {
			t.Fatalf("seat %d should have milled 5 (gy=%d)", s, len(gs.Seats[s].Graveyard))
		}
		if len(gs.Seats[s].Library) != 5 {
			t.Fatalf("seat %d library should be 10-5=5, got %d", s, len(gs.Seats[s].Library))
		}
	}
}

func TestEachOpponentMill_OnlyOpponents(t *testing.T) {
	gs := newTestGame(t, 3)
	for s := 0; s < 3; s++ {
		stockLibrary(gs, s, 10)
	}
	src := addTestPerm(gs, 0, "Fathom Feeder", "creature")

	if !resolveResidualByText(gs, src, "each opponent mills three cards") {
		t.Fatalf("expected recognized")
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Fatalf("controller should NOT be milled, gy=%d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[1].Graveyard) != 3 || len(gs.Seats[2].Graveyard) != 3 {
		t.Fatalf("opponents should mill 3 each, got %d/%d",
			len(gs.Seats[1].Graveyard), len(gs.Seats[2].Graveyard))
	}
}

func TestEachPlayerMill_ConditionalSkipped(t *testing.T) {
	gs := newTestGame(t, 2)
	stockLibrary(gs, 0, 5)
	stockLibrary(gs, 1, 5)
	src := addTestPerm(gs, 0, "Test", "sorcery")
	// "who discarded a card" conditional → must NOT be handled here.
	if resolveResidualByText(gs, src, "each player who discarded a card mills two cards") {
		// It may be handled by some other recognizer, but our handler must
		// not have fired: verify nothing was milled by us.
	}
	if len(gs.Seats[0].Graveyard) != 0 || len(gs.Seats[1].Graveyard) != 0 {
		t.Fatalf("conditional 'who ...' variant must not be milled by the generic handler")
	}
}

func TestEachPlayerMill_VariableCountSkipped(t *testing.T) {
	gs := newTestGame(t, 2)
	stockLibrary(gs, 0, 5)
	stockLibrary(gs, 1, 5)
	src := addTestPerm(gs, 0, "Test", "sorcery")
	if EachPlayerMillFromTail(gs, src, "each player mills x cards") {
		t.Fatalf("variable-X count must not be handled")
	}
}
