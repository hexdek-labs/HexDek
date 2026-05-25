package gameengine

import (
	"math/rand"
	"testing"
)

// Loki r60 extended-seeds / seed 2718 game 3428 turn 39 surfaced:
//
//   WinCondition: seat 0 lost via poison but has only 2 poison
//   counters (< 10)
//
// Pod: Genevieve Conniving Dragon / Bartolomé del Presidio / Zndrsplt
// Eye of Wisdom / Hapatra Vizier of Poisons. Hapatra's deck runs an
// infect package (Blightwidow, Dakmor Scorpion, Broodguard Elite).
//
// What happened: seat 0 took 10+ poison counters from infect combat
// damage, SBA 704.5c fired, markSeatLost stamped LossReason="ten or
// more poison counters (CR 704.5c)" and set Lost=true. Then a
// post-loss effect (Hapatra-pod counter-swap / "remove all poison
// counters" / etc.) decremented seat 0's PoisonCounters to 2. The
// invariant's substring-match-against-LossReason check fired the
// false positive — the loss was correct, the current state is just
// post-cleanup.
//
// Per CR §704.5c, losing the game is permanent. Once Lost via the
// SBA, subsequent counter adjustments are audit-preserved but do
// not undo the loss.
//
// The existing `life` check already has this guard:
//   if s.Life > 0 && !s.LeftGame { fire }
// — post-elimination triggers (Blood Artist, lifelink, etc.) can
// modify life afterward, so the check is scoped to seats not yet
// cleaned up. Fix: extend the same `!s.LeftGame` guard to the
// poison and commander-damage checks.

// TestWinCondition_PostElimPoisonReductionDoesNotFalsePositive pins
// the bit-stable seed-2718 game-3428 shape: a seat lost via poison,
// LeftGame=true, current PoisonCounters < 10 (was decremented
// post-loss). The invariant must NOT fire.
func TestWinCondition_PostElimPoisonReductionDoesNotFalsePositive(t *testing.T) {
	rng := rand.New(rand.NewSource(2718))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.Flags = map[string]int{"ended": 1, "winner": 3}
	gs.Seats[3].Won = true

	// Seat 0 lost via poison earlier in the game; LeftGame=true; current
	// counter count is 2 (post-cleanup / counter-swap).
	gs.Seats[0].Lost = true
	gs.Seats[0].LeftGame = true
	gs.Seats[0].LossReason = "ten or more poison counters (CR 704.5c)"
	gs.Seats[0].PoisonCounters = 2
	// Seats 1, 2 also Lost for cleanliness (game ended).
	gs.Seats[1].Lost = true
	gs.Seats[1].LeftGame = true
	gs.Seats[1].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Seats[1].Life = 0
	gs.Seats[2].Lost = true
	gs.Seats[2].LeftGame = true
	gs.Seats[2].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Seats[2].Life = 0

	if err := checkWinCondition(gs); err != nil {
		t.Fatalf("WinCondition must NOT fire on post-elim poison reduction (LeftGame=true); got: %v", err)
	}
}

// TestWinCondition_PrePoisonReductionStillFiresForActiveSeat pins
// the other direction: if a seat is Lost via poison but LeftGame is
// FALSE (still mid-cleanup, e.g. an invariant check between SBA
// passes), and the current poison count is < 10, that's a real bug
// — the invariant must still fire. Defends against narrowing the
// gate too far.
func TestWinCondition_PrePoisonReductionStillFiresForActiveSeat(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.Flags = map[string]int{"ended": 1, "winner": 1}
	gs.Seats[1].Won = true

	// Seat 0 marked Lost via poison BUT LeftGame is still false AND
	// PoisonCounters is too low — this is the bug shape the original
	// invariant was designed to catch.
	gs.Seats[0].Lost = true
	gs.Seats[0].LeftGame = false
	gs.Seats[0].LossReason = "ten or more poison counters (CR 704.5c)"
	gs.Seats[0].PoisonCounters = 2

	err := checkWinCondition(gs)
	if err == nil {
		t.Fatal("WinCondition MUST fire on poison-loss reason with low counters AND LeftGame=false")
	}
}

// TestWinCondition_LegitimatePoisonLossPasses confirms the
// straightforward valid case: seat lost via poison with 10+ counters
// still on record (e.g. SBA just fired, no counter-removal happened).
func TestWinCondition_LegitimatePoisonLossPasses(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.Flags = map[string]int{"ended": 1, "winner": 1}
	gs.Seats[1].Won = true

	gs.Seats[0].Lost = true
	gs.Seats[0].LeftGame = true
	gs.Seats[0].LossReason = "ten or more poison counters (CR 704.5c)"
	gs.Seats[0].PoisonCounters = 12

	if err := checkWinCondition(gs); err != nil {
		t.Fatalf("clean poison-loss state must pass; got: %v", err)
	}
}

// TestWinCondition_PostElimCommanderDamageReductionDoesNotFalsePositive
// mirrors the poison guard for the commander-damage path. Same shape:
// loss via commander damage occurred, then post-elim cleanup reduced
// the recorded damage (e.g. CommanderDamage map entries were touched
// by §800.4a cleanup or other effects). The invariant must trust the
// loss reason once LeftGame=true.
func TestWinCondition_PostElimCommanderDamageReductionDoesNotFalsePositive(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	gs := NewGameState(2, rng, nil)
	gs.CommanderFormat = true
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.Flags = map[string]int{"ended": 1, "winner": 1}
	gs.Seats[1].Won = true

	gs.Seats[0].Lost = true
	gs.Seats[0].LeftGame = true
	gs.Seats[0].LossReason = "21+ commander damage from Voltron Commander (CR 704.6c)"
	// CommanderDamage shows only a residual value (post-cleanup).
	gs.Seats[0].CommanderDamage = map[int]map[string]int{
		1: {"Voltron Commander": 3},
	}

	if err := checkWinCondition(gs); err != nil {
		t.Fatalf("WinCondition must not fire on post-elim commander damage reduction; got: %v", err)
	}
}

// TestWinCondition_PreLeftGameCommanderDamageStillFires confirms the
// commander-damage guard is symmetric with poison: still fires when
// LeftGame=false.
func TestWinCondition_PreLeftGameCommanderDamageStillFires(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	gs := NewGameState(2, rng, nil)
	gs.CommanderFormat = true
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.Flags = map[string]int{"ended": 1, "winner": 1}
	gs.Seats[1].Won = true

	gs.Seats[0].Lost = true
	gs.Seats[0].LeftGame = false
	gs.Seats[0].LossReason = "21+ commander damage from Voltron Commander (CR 704.6c)"
	gs.Seats[0].CommanderDamage = map[int]map[string]int{
		1: {"Voltron Commander": 3},
	}

	err := checkWinCondition(gs)
	if err == nil {
		t.Fatal("WinCondition MUST fire on low commander damage with LeftGame=false")
	}
}
