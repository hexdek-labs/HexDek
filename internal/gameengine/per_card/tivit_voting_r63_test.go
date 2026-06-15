package per_card

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func countTokensByName(gs *gameengine.GameState, seat int, name string) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			n++
		}
	}
	return n
}

func newTivitPerm(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	p := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Tivit, Seller of Secrets", Owner: seat, Types: []string{"creature", "artifact"}},
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// CR §701.20 + Tivit's "you get a vote for each vote": in an N-player game the
// payout scales with the table — every player votes (one token each, all to the
// controller), then the controller gets N more votes. So 4 players → ~8 tokens.
// Before r63 Tivit hardcoded a fixed 2 Clue + 2 Treasure (a 2-player result).
func TestTivit_PayoutScalesWithFourPlayers(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(11)), nil)
	tivit := newTivitPerm(gs, 0)

	tivitPayout(gs, tivit)

	clues := countTokensByName(gs, 0, "Clue Token")
	treasures := countTokensByName(gs, 0, "Treasure Token")
	total := clues + treasures
	// 4 living players → 4 base votes + 4 "you get a vote for each vote" = 8.
	if total != 8 {
		t.Fatalf("Tivit in a 4-player game should make 8 tokens (2× living players), got %d (clues=%d treasures=%d)", total, clues, treasures)
	}
}

// Two-player: 2 base + 2 extra = 4 tokens.
func TestTivit_PayoutScalesWithTwoPlayers(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(7)), nil)
	tivit := newTivitPerm(gs, 0)

	tivitPayout(gs, tivit)

	total := countTokensByName(gs, 0, "Clue Token") + countTokensByName(gs, 0, "Treasure Token")
	if total != 4 {
		t.Fatalf("Tivit in a 2-player game should make 4 tokens, got %d", total)
	}
}

// A dead opponent doesn't vote (CR §800.4): 4 seats, one eliminated → 3 voters
// → 6 tokens.
func TestTivit_DeadOpponentDoesNotVote(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(13)), nil)
	gs.Seats[2].Lost = true
	tivit := newTivitPerm(gs, 0)

	tivitPayout(gs, tivit)

	total := countTokensByName(gs, 0, "Clue Token") + countTokensByName(gs, 0, "Treasure Token")
	if total != 6 {
		t.Fatalf("with one of four players eliminated, Tivit should make 6 tokens (2×3 living), got %d", total)
	}
}
