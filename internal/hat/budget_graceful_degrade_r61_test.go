package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// budget_graceful_degrade_r61_test.go — r61 PR-8 Fix A.
//
// Pre-r61, effectiveBudget cliffed to 0 (pure heuristic — no evaluator,
// no rollout) the instant total permanents crossed the complexity
// threshold (~60), which is a routine turn-8+ 4-player Commander board.
// The hat went brain-dead exactly when decisions are hardest. PR-8
// replaces the hard cliff with a linear taper toward a non-zero floor,
// and makes the combo carve-out explicit in the live gate so the hat
// never degrades on a turn it is assembling/executing its own combo.

// stuffBoard appends `n` zero-power filler permanents per seat.
func stuffBoardPerSeat(gs *gameengine.GameState, perSeat int) {
	for _, s := range gs.Seats {
		for i := 0; i < perSeat; i++ {
			s.Battlefield = append(s.Battlefield, &gameengine.Permanent{
				Card:       &gameengine.Card{Name: "filler"},
				Owner:      s.Idx,
				Controller: s.Idx,
			})
		}
	}
}

// TestEffectiveBudget_NonZeroOn70PermanentBoard is the headline Fix A
// pin: a ~70-permanent board with NO high-stakes signal must keep a
// non-zero (tapered) evaluation budget rather than dropping to 0.
func TestEffectiveBudget_NonZeroOn70PermanentBoard(t *testing.T) {
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	// ~72 permanents total (18 per seat) — well past the flat 60 threshold
	// at an early turn, with no low-life / deep-stack / pressure signal.
	stuffBoardPerSeat(gs, 18)

	h := newTierHat(100)
	got := h.effectiveBudget(gs)
	if got <= 0 {
		t.Fatalf("~72-permanent board must keep SOME evaluation budget (no thinking-cliff); got %d", got)
	}
	if got >= 100 {
		t.Fatalf("~72-permanent board should degrade BELOW the base budget; got %d (base 100)", got)
	}
	// over = 72 - 60 = 12 → factor 1 - 12*0.02 = 0.76 → 100*0.76 = 76.
	if got != 76 {
		t.Fatalf("taper math: want 76 (100 * (1 - 12*0.02)); got %d", got)
	}
}

// TestEffectiveBudget_FloorHoldsOnPathologicalBoard pins the floor: even
// a 120-permanent board never returns 0, and never blows past the floor
// factor (perf bound — the taper keeps per-decision cost small).
func TestEffectiveBudget_FloorHoldsOnPathologicalBoard(t *testing.T) {
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	stuffBoardPerSeat(gs, 40) // 160 permanents

	h := newTierHat(100)
	got := h.effectiveBudget(gs)
	if got <= 0 {
		t.Fatalf("pathological board must still do SOME evaluation; got %d", got)
	}
	// Floor factor 0.15 → 100*0.15 = 15. The taper bottoms out well before
	// this board size, so the floor (not the linear taper) governs.
	if got != 15 {
		t.Fatalf("floor-factor pin: want 15 (100 * 0.15); got %d", got)
	}
}

// TestEffectiveBudget_RolloutBudgetTapersBelowRolloutThreshold pins the
// perf bound for rollout-capable hats: a base rollout budget (>= 200) on
// a big board tapers BELOW rolloutBudgetGe, so rollouts cut out first and
// only the cheaper evaluator path runs — bounding per-decision cost.
func TestEffectiveBudget_RolloutBudgetTapersBelowRolloutThreshold(t *testing.T) {
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	stuffBoardPerSeat(gs, 25) // 100 permanents

	h := newTierHat(rolloutBudgetGe) // 200
	got := h.effectiveBudget(gs)
	if got <= 0 {
		t.Fatalf("rollout hat on big board must keep SOME budget; got %d", got)
	}
	if got >= rolloutBudgetGe {
		t.Fatalf("rollout budget must taper BELOW rolloutBudgetGe (%d) on a big board so rollouts cut out first; got %d",
			rolloutBudgetGe, got)
	}
}

// TestEffectiveBudget_ComboTurnNeverDegrades is the combo carve-out pin:
// the hat is assembling its own combo on a complex board. effectiveBudget
// must NOT degrade — it returns the full (combo-lifted) budget.
func TestEffectiveBudget_ComboTurnNeverDegrades(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Active = 0
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	// Complex board: ~72 permanents (would degrade a non-combo turn).
	stuffBoardPerSeat(gs, 18)

	// Seat 0 is assembling: 1 anchoring piece + 2 tutors for a 3-piece
	// combo (the canonical Assembling shape from the sequencer suite).
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		sequencerTutor("Demonic Tutor", 2),
		sequencerTutor("Vampiric Tutor", 1),
	)

	h := NewYggdrasilHat(makeThreeCardSequencerProfile(), 0)
	h.Budget = 100
	h.Noise = 0

	// Sanity: the carve-out predicate fires for this state.
	if !h.comboAssembling(gs) {
		t.Fatalf("test setup: seat 0 should be combo-assembling")
	}

	got := h.effectiveBudget(gs)
	if got <= 0 {
		t.Fatalf("combo-assembly turn must NOT degrade to 0 on a complex board; got %d", got)
	}
	// Combo-assembly turn is high-stakes → full budget with the PlanAssemble
	// lift can apply, but at minimum it must be >= the base budget (NOT a
	// degraded value below it). The degrade taper must be bypassed entirely.
	if got < 100 {
		t.Fatalf("combo-assembly turn must keep at least the full base budget (no degrade); got %d (base 100)", got)
	}
}

// TestEffectiveBudget_LowPressureCleanBoardUnchanged guards against over-
// reach: a simple, low-permanent board still returns exactly the base
// budget (no taper, no pressure lift). The degrade path must only fire
// past the complexity threshold.
func TestEffectiveBudget_LowPressureCleanBoardUnchanged(t *testing.T) {
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	stuffBoardPerSeat(gs, 5) // 20 permanents — well below threshold

	h := newTierHat(100)
	if got := h.effectiveBudget(gs); got != 100 {
		t.Fatalf("clean low-pressure board: want exact base budget 100, got %d", got)
	}
}
