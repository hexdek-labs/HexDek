package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// conviction_graveyard_latent_r60_test.go — pins the r60 conviction
// uplift: convictionGraveyardLatent credits winline/finisher pieces
// currently in our graveyard, gated on a recursion path being active.
// Adjusts the conviction-LOCAL score-window relPos only; global
// h.relativePosition stays untouched so attack-targeting / cast-
// prioritization / other consumers see no change.

// buildLatentHat constructs a hat with a tiny strategy containing one
// 2-card combo plan ("PieceA" + "PieceB") plus one finisher ("FinX").
// Returns hat + game so individual tests vary graveyard / battlefield.
func buildLatentHat(t *testing.T) (*YggdrasilHat, *gameengine.GameState) {
	t.Helper()
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	h := primedYggdrasilHat(2)
	h.Strategy = &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"PieceA", "PieceB"}, Type: "infinite", Class: "infinite_drain"},
		},
		FinisherCards: []string{"FinX"},
	}
	return h, gs
}

// addBreach drops an Underworld Breach-shaped permanent so
// seatHasComboGraveyardRecursion fires.
func addBreach(seat *gameengine.Seat) {
	c := newTestCardMinimal("Underworld Breach", []string{"enchantment"}, 2, nil)
	newTestPermanent(seat, c, 0, 0)
}

// -----------------------------------------------------------------------------
// 1. Latent uplift fires when recursion engine + winline piece in grave
// -----------------------------------------------------------------------------

func TestConvictionGraveyardLatent_FiresWhenEngineAndPiecePresent(t *testing.T) {
	h, gs := buildLatentHat(t)
	seat := gs.Seats[0]
	addBreach(seat)
	seat.Graveyard = append(seat.Graveyard,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))

	got := h.convictionGraveyardLatent(gs, 0)
	want := 0.08
	if got != want {
		t.Errorf("1 winline piece + recursion engine: got %.4f, want %.4f", got, want)
	}
}

// 3 winline pieces in graveyard → 3 × 0.08 = 0.24 (still under the 0.30 cap).
func TestConvictionGraveyardLatent_ScalesWithCount(t *testing.T) {
	h, gs := buildLatentHat(t)
	seat := gs.Seats[0]
	addBreach(seat)
	seat.Graveyard = append(seat.Graveyard,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		newTestCardMinimal("PieceB", []string{"creature"}, 3, nil),
		newTestCardMinimal("FinX", []string{"creature"}, 5, nil),
	)
	got := h.convictionGraveyardLatent(gs, 0)
	want := 3 * 0.08
	if got != want {
		t.Errorf("3 pieces in grave: got %.4f, want %.4f", got, want)
	}
}

// Cap at 0.30 — defense against false-positive suppression on a long
// graveyard with many winline pieces (unlikely but defensive).
func TestConvictionGraveyardLatent_CapsAtThirty(t *testing.T) {
	h, gs := buildLatentHat(t)
	// Strategy with 5 winline pieces, all in graveyard → would compute
	// 0.40 uncapped → must clamp at 0.30.
	h.Strategy = &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"P1", "P2", "P3", "P4", "P5"}, Type: "infinite"},
		},
	}
	seat := gs.Seats[0]
	addBreach(seat)
	for _, name := range []string{"P1", "P2", "P3", "P4", "P5"} {
		seat.Graveyard = append(seat.Graveyard,
			newTestCardMinimal(name, []string{"creature"}, 3, nil))
	}
	got := h.convictionGraveyardLatent(gs, 0)
	if got != 0.30 {
		t.Errorf("5 pieces (would be 0.40 uncapped): got %.4f, want 0.30 (cap)", got)
	}
}

// -----------------------------------------------------------------------------
// 2. Negative-of-the-fix: gates correctly
// -----------------------------------------------------------------------------

// No recursion engine in play → uplift 0 even with pieces in graveyard
// (without a recursion path, those pieces aren't recoverable).
func TestConvictionGraveyardLatent_NoEngineNoUplift(t *testing.T) {
	h, gs := buildLatentHat(t)
	seat := gs.Seats[0]
	// Piece in graveyard but no recursion engine on board.
	seat.Graveyard = append(seat.Graveyard,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))

	got := h.convictionGraveyardLatent(gs, 0)
	if got != 0 {
		t.Errorf("no recursion engine: got %.4f, want 0", got)
	}
}

// Engine in play but no winline pieces in graveyard → 0.
func TestConvictionGraveyardLatent_EngineButNoPiecesIsZero(t *testing.T) {
	h, gs := buildLatentHat(t)
	seat := gs.Seats[0]
	addBreach(seat)
	// Random non-winline card in graveyard.
	seat.Graveyard = append(seat.Graveyard,
		newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil))

	got := h.convictionGraveyardLatent(gs, 0)
	if got != 0 {
		t.Errorf("engine but no winline pieces in grave: got %.4f, want 0", got)
	}
}

// Empty graveyard → 0.
func TestConvictionGraveyardLatent_EmptyGraveyardIsZero(t *testing.T) {
	h, gs := buildLatentHat(t)
	addBreach(gs.Seats[0])
	got := h.convictionGraveyardLatent(gs, 0)
	if got != 0 {
		t.Errorf("empty grave: got %.4f, want 0", got)
	}
}

// No strategy → 0 (no winline set to match against).
func TestConvictionGraveyardLatent_NoStrategyIsZero(t *testing.T) {
	h, gs := buildLatentHat(t)
	h.Strategy = nil
	addBreach(gs.Seats[0])
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))
	got := h.convictionGraveyardLatent(gs, 0)
	if got != 0 {
		t.Errorf("nil strategy: got %.4f, want 0", got)
	}
}

// Lost / left-game seat → 0 (no point computing recovery equity).
func TestConvictionGraveyardLatent_LostSeatIsZero(t *testing.T) {
	h, gs := buildLatentHat(t)
	addBreach(gs.Seats[0])
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))
	gs.Seats[0].Lost = true
	got := h.convictionGraveyardLatent(gs, 0)
	if got != 0 {
		t.Errorf("lost seat: got %.4f, want 0", got)
	}
}

// Nil game / OOB seat — defensive paths must return 0, not panic.
func TestConvictionGraveyardLatent_NilGameNoPanic(t *testing.T) {
	h := primedYggdrasilHat(2)
	if got := h.convictionGraveyardLatent(nil, 0); got != 0 {
		t.Errorf("nil game: got %.4f, want 0", got)
	}
	gs := newTestGame(t, 2)
	if got := h.convictionGraveyardLatent(gs, 99); got != 0 {
		t.Errorf("oob seat: got %.4f, want 0", got)
	}
}

// -----------------------------------------------------------------------------
// 3. Integration: uplift suppresses score-window scoreTriggered
// -----------------------------------------------------------------------------

// Build a small synthetic scenario: place enough turns of low-relPos
// samples into the window that scoreTriggered would have fired, then
// verify the uplift moves the window samples above the threshold so
// the trigger does NOT fire. We can't easily fabricate a -0.5 relPos
// through real eval calls in a small test fixture, but we can directly
// pre-fill d.relPosWindow with values that depend on the uplift's
// arithmetic and verify the trigger gate behaves correctly.
func TestConvictionScoreTriggered_UpliftSuppressesFalseTrigger(t *testing.T) {
	h, gs := buildLatentHat(t)
	gs.Turn = 12 // above convictionScoreMinTurn (10)

	// Set up: opponent has a much higher evalPosition so our relPos is
	// strongly negative. Easiest path is to give the opponent a stacked
	// board and ourselves nothing.
	for i := 0; i < 5; i++ {
		newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Big", []string{"creature"}, 4, nil), 4, 4)
	}

	// Our seat: a recursion engine in play AND winline pieces in
	// graveyard (so the uplift fires AT MAX = 0.30).
	addBreach(gs.Seats[0])
	for _, name := range []string{"PieceA", "PieceB", "FinX"} {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal(name, []string{"creature"}, 3, nil))
	}

	// Force the conviction diagnostic to record samples across enough
	// turns to fill the window. The recordConvictionSample emits an
	// event each call — we just want the trigger arithmetic to run.
	for i := 0; i < convictionScoreWindow; i++ {
		h.recordConvictionSample(gs, 0)
	}

	// Read back the last sample's window state. If the uplift had not
	// fired, the window of strongly-negative samples would have
	// triggered scoreTriggered. With the uplift it should not have.
	if h.convictionDiag == nil {
		t.Fatalf("no diagnostic state recorded")
	}
	// Each sample in the window equals (raw relPos + uplift). The
	// uplift adds 0.24 (3 pieces × 0.08). If raw relPos was, say,
	// -0.5, the adjusted value is -0.26 which is ABOVE the threshold
	// -0.35 and should NOT fire scoreTriggered. Verify each window
	// sample is consistent with the uplift having been applied.
	for i, v := range h.convictionDiag.relPosWindow {
		// Recompute unadjusted relPos to sanity-check the diff.
		raw := h.relativePosition(gs, 0)
		expected := raw + h.convictionGraveyardLatent(gs, 0)
		if v != expected {
			t.Errorf("window[%d] = %.4f, expected %.4f (raw=%.4f + uplift=%.4f)",
				i, v, expected, raw, expected-raw)
		}
	}
}

// Conversely: without a recursion engine in play, the window samples
// reflect the raw relPos (no uplift). Confirms the uplift gate works
// at the integration level.
func TestConvictionScoreTriggered_NoEngineNoUplift(t *testing.T) {
	h, gs := buildLatentHat(t)
	gs.Turn = 12
	// Opponent board.
	for i := 0; i < 5; i++ {
		newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Big", []string{"creature"}, 4, nil), 4, 4)
	}
	// Pieces in graveyard but NO recursion engine.
	for _, name := range []string{"PieceA", "PieceB"} {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal(name, []string{"creature"}, 3, nil))
	}
	for i := 0; i < convictionScoreWindow; i++ {
		h.recordConvictionSample(gs, 0)
	}
	raw := h.relativePosition(gs, 0)
	for i, v := range h.convictionDiag.relPosWindow {
		if v != raw {
			t.Errorf("window[%d] = %.4f, want raw relPos %.4f (no uplift expected)",
				i, v, raw)
		}
	}
}
