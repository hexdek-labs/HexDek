package hexapi

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/trueskill"
)

// showmatch_composition_monitoring_test.go — regressions for the
// PR #418 monitoring instrumentation. updateELO now returns a
// []heimdall.CompositionPriorEffect that captures the prior's
// per-seat effect on this game's TrueSkill update — including the
// MuDeltaVsBaseline gold metric (prior-aware μ minus vanilla-shadow
// μ) which downstream Heimdall analytics aggregate.

func buildShowmatchForMonitoring(deckKeys, archetypes []string, prior *trueskill.CompositionPrior) *Showmatch {
	sm := &Showmatch{
		elo:          map[string]*eloState{},
		bracketCache: map[string]int{},
		strategies:   map[string]*hat.StrategyProfile{},
	}
	for i, key := range deckKeys {
		sm.strategies[key] = &hat.StrategyProfile{Archetype: archetypes[i]}
	}
	if prior != nil {
		sm.compositionPrior = prior
		sm.compositionPriorCfg = trueskill.DefaultCompositionUpdateConfig(prior)
	}
	return sm
}

func runMonitoredGame(sm *Showmatch, deckKeys []string, winner int) []interface{} {
	commanders := make([]string, len(deckKeys))
	decks := make([]*deckparser.TournamentDeck, len(deckKeys))
	for i, k := range deckKeys {
		commanders[i] = k
	}
	effects := sm.updateELO(deckKeys, commanders, decks, winner, 30)
	// Box to interface{} so the test helper signature stays generic.
	out := make([]interface{}, len(effects))
	for i, e := range effects {
		out[i] = e
	}
	return out
}

// -----------------------------------------------------------------------------
// updateELO returns one effect entry per seat
// -----------------------------------------------------------------------------

func TestUpdateELO_ReturnsOneEffectPerSeat(t *testing.T) {
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	sm := buildShowmatchForMonitoring(deckKeys, archs, trueskill.NewCompositionPrior(4))

	commanders := []string{"d0", "d1", "d2", "d3"}
	decks := make([]*deckparser.TournamentDeck, 4)
	effects := sm.updateELO(deckKeys, commanders, decks, 0, 30)

	if len(effects) != 4 {
		t.Fatalf("updateELO returned %d effects, want 4", len(effects))
	}
	for i, eff := range effects {
		if eff.Seat != i {
			t.Errorf("effect[%d].Seat = %d, want %d", i, eff.Seat, i)
		}
		if eff.Archetype != archs[i] {
			t.Errorf("effect[%d].Archetype = %q, want %q", i, eff.Archetype, archs[i])
		}
	}
}

// -----------------------------------------------------------------------------
// Cold-start: MuDeltaVsBaseline = 0 for all seats (offsets are zero)
// -----------------------------------------------------------------------------

func TestUpdateELO_ColdStartHasZeroMuDelta(t *testing.T) {
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	sm := buildShowmatchForMonitoring(deckKeys, archs, trueskill.NewCompositionPrior(4))

	commanders := []string{"d0", "d1", "d2", "d3"}
	decks := make([]*deckparser.TournamentDeck, 4)
	effects := sm.updateELO(deckKeys, commanders, decks, 0, 30)

	for i, eff := range effects {
		if math.Abs(eff.MuDeltaVsBaseline) > 1e-9 {
			t.Errorf("cold-start effect[%d]: MuDeltaVsBaseline = %.6f, want 0",
				i, eff.MuDeltaVsBaseline)
		}
		if math.Abs(eff.Offset) > 1e-9 {
			t.Errorf("cold-start effect[%d]: Offset = %.6f, want 0", i, eff.Offset)
		}
		if eff.Confidence != 0 {
			t.Errorf("cold-start effect[%d]: Confidence = %.4f, want 0", i, eff.Confidence)
		}
	}
}

func TestUpdateELO_NilPriorHasZeroMuDelta(t *testing.T) {
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	sm := buildShowmatchForMonitoring(deckKeys, archs, nil) // nil prior

	commanders := []string{"d0", "d1", "d2", "d3"}
	decks := make([]*deckparser.TournamentDeck, 4)
	effects := sm.updateELO(deckKeys, commanders, decks, 0, 30)

	for i, eff := range effects {
		if math.Abs(eff.MuDeltaVsBaseline) > 1e-9 {
			t.Errorf("nil-prior effect[%d]: MuDeltaVsBaseline = %.6f, want 0",
				i, eff.MuDeltaVsBaseline)
		}
	}
}

// -----------------------------------------------------------------------------
// Active prior: MuDeltaVsBaseline reflects the shift direction
// -----------------------------------------------------------------------------

func TestUpdateELO_FavoredWinnerHasNegativeMuDelta(t *testing.T) {
	// Prior strongly favors Mill. When Mill wins, the prior dampens
	// the μ gain → priorAfter < vanillaAfter → MuDeltaVsBaseline < 0
	// for the winning seat.
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	prior := trueskill.NewCompositionPrior(4)
	for i := 0; i < 500; i++ {
		_ = prior.ObserveGame(archs, "Mill")
	}
	sm := buildShowmatchForMonitoring(deckKeys, archs, prior)

	commanders := []string{"d0", "d1", "d2", "d3"}
	decks := make([]*deckparser.TournamentDeck, 4)
	effects := sm.updateELO(deckKeys, commanders, decks, 0, 30) // Mill wins

	millEff := effects[0]
	if millEff.MuDeltaVsBaseline >= 0 {
		t.Errorf("favored Mill winner: MuDeltaVsBaseline should be < 0 (prior dampened gain); got %.4f",
			millEff.MuDeltaVsBaseline)
	}
	if millEff.Offset <= 0 {
		t.Errorf("favored Mill: Offset should be > 0 (prior shifted Mill's μ up before update); got %.4f",
			millEff.Offset)
	}
	if millEff.Confidence <= 0 {
		t.Errorf("after 500 prior observations, Mill Confidence should be > 0; got %.4f",
			millEff.Confidence)
	}
}

func TestUpdateELO_DisfavoredWinnerHasPositiveMuDelta(t *testing.T) {
	// Prior favors Mill, but Voltron wins. The prior shifted Voltron's
	// μ DOWN before the update (negative offset), so when the update
	// runs in shifted space and the delta is applied back, Voltron's
	// final μ rises MORE than vanilla would have allowed → positive
	// MuDeltaVsBaseline.
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	prior := trueskill.NewCompositionPrior(4)
	for i := 0; i < 500; i++ {
		_ = prior.ObserveGame(archs, "Mill")
	}
	sm := buildShowmatchForMonitoring(deckKeys, archs, prior)

	commanders := []string{"d0", "d1", "d2", "d3"}
	decks := make([]*deckparser.TournamentDeck, 4)
	effects := sm.updateELO(deckKeys, commanders, decks, 1, 30) // Voltron wins

	voltronEff := effects[1]
	if voltronEff.MuDeltaVsBaseline <= 0 {
		t.Errorf("disfavored Voltron winner: MuDeltaVsBaseline should be > 0 (prior amplified the upset); got %.4f",
			voltronEff.MuDeltaVsBaseline)
	}
	if voltronEff.Offset >= 0 {
		t.Errorf("disfavored Voltron: Offset should be < 0 (prior didn't expect Voltron to win); got %.4f",
			voltronEff.Offset)
	}
}

// -----------------------------------------------------------------------------
// Confidence + ExpectedWinrate populated from prior state
// -----------------------------------------------------------------------------

func TestUpdateELO_ExpectedWinrateMatchesPriorMean(t *testing.T) {
	// Seed prior such that Mill wins 80% of games in this pod. After
	// 100 games, ExpectedWinrate(Mill, pod) ≈ 0.80.
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	prior := trueskill.NewCompositionPrior(4)
	for i := 0; i < 100; i++ {
		winner := "Mill"
		if i >= 80 {
			winner = "Voltron"
		}
		_ = prior.ObserveGame(archs, winner)
	}
	sm := buildShowmatchForMonitoring(deckKeys, archs, prior)

	commanders := []string{"d0", "d1", "d2", "d3"}
	decks := make([]*deckparser.TournamentDeck, 4)
	effects := sm.updateELO(deckKeys, commanders, decks, 0, 30)

	millEff := effects[0]
	if math.Abs(millEff.ExpectedWinrate-0.80) > 0.05 {
		t.Errorf("Mill ExpectedWinrate = %.4f, want ≈ 0.80", millEff.ExpectedWinrate)
	}
	// Confidence at n≈300 pairwise samples per opponent (3 opps × 100
	// games) should be solidly above 0.5.
	if millEff.Confidence < 0.5 {
		t.Errorf("Mill Confidence after 100 games = %.4f, want > 0.5", millEff.Confidence)
	}
}

// -----------------------------------------------------------------------------
// Draw path returns effects too (early-return branch in updateELO)
// -----------------------------------------------------------------------------

func TestUpdateELO_DrawPathReturnsEffects(t *testing.T) {
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	sm := buildShowmatchForMonitoring(deckKeys, archs, trueskill.NewCompositionPrior(4))

	commanders := []string{"d0", "d1", "d2", "d3"}
	decks := make([]*deckparser.TournamentDeck, 4)
	effects := sm.updateELO(deckKeys, commanders, decks, -1, 30) // draw

	if len(effects) != 4 {
		t.Fatalf("draw path returned %d effects, want 4 (the no-winner early-return branch)",
			len(effects))
	}
	// Cold-start prior → all deltas zero.
	for i, eff := range effects {
		if math.Abs(eff.MuDeltaVsBaseline) > 1e-9 {
			t.Errorf("draw effect[%d]: MuDeltaVsBaseline = %.6f, want 0", i, eff.MuDeltaVsBaseline)
		}
	}
}

// -----------------------------------------------------------------------------
// Sum of MuDeltaVsBaseline across all seats ≈ 0 (rating conservation)
// -----------------------------------------------------------------------------

func TestUpdateELO_TotalMuDeltaApproximatelyZero(t *testing.T) {
	// TrueSkill conserves total μ (winner's gain ≈ losers' loss). The
	// prior shifts how that conserved total is allocated, so the SUM
	// of MuDeltaVsBaseline across all seats should be close to zero —
	// the prior redistributes, it doesn't inject or remove rating.
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	prior := trueskill.NewCompositionPrior(4)
	for i := 0; i < 500; i++ {
		_ = prior.ObserveGame(archs, "Mill")
	}
	sm := buildShowmatchForMonitoring(deckKeys, archs, prior)

	commanders := []string{"d0", "d1", "d2", "d3"}
	decks := make([]*deckparser.TournamentDeck, 4)
	effects := sm.updateELO(deckKeys, commanders, decks, 0, 30)

	total := 0.0
	for _, eff := range effects {
		total += eff.MuDeltaVsBaseline
	}
	// Within 0.5 μ — small drift OK due to σ-shrink interactions, but
	// no systematic injection.
	if math.Abs(total) > 0.5 {
		t.Errorf("sum of MuDeltaVsBaseline = %.4f, want ≈ 0 (prior should redistribute, not inject)",
			total)
	}
}
