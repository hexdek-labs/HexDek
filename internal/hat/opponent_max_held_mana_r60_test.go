package hat

import (
	"testing"
)

// opponent_max_held_mana_r60_test.go — pins the r60 3rd Eye magnitude
// signal: opponentMaxHeldMana tracks the largest available-mana value
// ever observed at any opponent's upkeep, complementing the binary
// streak counter opponentHeldMana. Pre-r60, "opp consistently holds 2
// mana (Spell Pierce / Mana Leak rep)" and "opp consistently holds 4
// mana (Cryptic Command / Force of Negation rep)" both incremented the
// same streak counter — the magnitude dimension was invisible to the
// classifier. The new field + classifyOpponent branch on maxHeldMana>=4
// gives a Cryptic-class control read on its own.

// hatWithMaxHeld primes a test hat including the new opponentMaxHeldMana
// slice. The existing primedYggdrasilHat helper predates the field; we
// extend it inline rather than mutating the shared helper to avoid
// touching unrelated tests.
func hatWithMaxHeld(seats int) *YggdrasilHat {
	h := primedYggdrasilHat(seats)
	h.opponentMaxHeldMana = make([]int, seats)
	return h
}

// -----------------------------------------------------------------------------
// 1. OpponentMaxHeldMana accessor — bounds + happy path.
// -----------------------------------------------------------------------------

func TestOpponentMaxHeldMana_AccessorReturnsTrackedValue(t *testing.T) {
	h := hatWithMaxHeld(4)
	h.opponentMaxHeldMana[2] = 5
	if got := h.OpponentMaxHeldMana(2); got != 5 {
		t.Errorf("OpponentMaxHeldMana(2) = %d, want 5", got)
	}
}

func TestOpponentMaxHeldMana_OutOfBoundsReturnsZero(t *testing.T) {
	h := hatWithMaxHeld(2)
	if got := h.OpponentMaxHeldMana(-1); got != 0 {
		t.Errorf("negative seat: got %d, want 0", got)
	}
	if got := h.OpponentMaxHeldMana(99); got != 0 {
		t.Errorf("oob seat: got %d, want 0", got)
	}
}

func TestOpponentMaxHeldMana_NilSliceReturnsZero(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	// opponentMaxHeldMana isn't init'd until the first ObserveEvent /
	// game_start path runs — accessor must be safe before then.
	if got := h.OpponentMaxHeldMana(0); got != 0 {
		t.Errorf("pre-init slice: got %d, want 0", got)
	}
}

// -----------------------------------------------------------------------------
// 2. classifyOpponent — maxHeldMana >= 4 lights up "control" on its own.
// -----------------------------------------------------------------------------

func TestClassifyOpponent_HighMaxHeldManaSignalsControl(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 6
	h := hatWithMaxHeld(2)

	// Opponent has sustained 4-mana representation but no observed
	// tutors / removal / counters yet — the streak counter alone or the
	// removal/counter heuristic would leave them at "unknown". The
	// magnitude signal should pick them up as "control".
	h.opponentMaxHeldMana[1] = 4

	prof := h.classifyOpponent(gs, 1)
	if prof == nil {
		t.Fatalf("nil profile")
	}
	if prof.Archetype != "control" {
		t.Errorf("archetype=%q, want control (maxHeldMana=4 signal)", prof.Archetype)
	}
	if prof.Confidence < 0.55 {
		t.Errorf("confidence=%.2f, want >= 0.55 baseline", prof.Confidence)
	}
}

// Negative-of-the-fix: maxHeldMana=3 should NOT trip the control branch
// (the threshold is 4 for the Cryptic-class read).
func TestClassifyOpponent_MaxHeldManaBelowThresholdStaysUnknown(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 6
	h := hatWithMaxHeld(2)
	h.opponentMaxHeldMana[1] = 3 // below the 4 threshold

	prof := h.classifyOpponent(gs, 1)
	if prof.Archetype != "unknown" {
		t.Errorf("archetype=%q, want unknown (maxHeldMana=3 below 4 threshold)",
			prof.Archetype)
	}
}

// Negative-of-the-fix: with maxHeldMana >= 4 but a wide creature board,
// the existing aggro branch wins. The new branch is gated on light
// creature board (<= 3) so a Cyclops Electromancer + 4 mountains
// doesn't get mistaken for control.
func TestClassifyOpponent_HighMaxHeldManaWithWideBoardStaysAggro(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 4
	h := hatWithMaxHeld(2)
	h.opponentMaxHeldMana[1] = 5
	// Five creatures on the table by turn 4 — clearly aggro.
	for i := 0; i < 5; i++ {
		card := newTestCardMinimal("Bear", []string{"creature"}, 1, nil)
		h.recordOpponentPlay("cast", card.DisplayName(), 1, card, gs.Turn)
	}

	prof := h.classifyOpponent(gs, 1)
	if prof.Archetype != "aggro" {
		t.Errorf("archetype=%q, want aggro (board signal beats magnitude when board>=3)",
			prof.Archetype)
	}
}

// Existing combo path (tutor + held streak + light board) still wins
// over the new control path even with high maxHeldMana — combo's
// confidence baseline (0.7) is higher than the new control branch
// (0.55) and the combo check is evaluated first.
func TestClassifyOpponent_ComboPathStillWinsOverMagnitudeBranch(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := hatWithMaxHeld(2)
	h.recordOpponentPlay("tutor", "Demonic Tutor", 1, nil, gs.Turn)
	h.opponentHeldMana[1] = 3
	h.opponentMaxHeldMana[1] = 5 // would also satisfy the new branch

	prof := h.classifyOpponent(gs, 1)
	if prof.Archetype != "combo" {
		t.Errorf("archetype=%q, want combo (tutor+held wins over magnitude)",
			prof.Archetype)
	}
}
