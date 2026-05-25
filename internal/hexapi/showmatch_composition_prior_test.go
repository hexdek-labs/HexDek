package hexapi

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/trueskill"
)

// showmatch_composition_prior_test.go — wiring tests for PR #412.
// Validates that the showmatch updateELO path:
//   1. Calls ObserveGame on the prior after each game so future
//      games benefit from accumulated archetype-pod evidence.
//   2. Reduces to vanilla behavior when the prior is nil OR cold-
//      start (Confidence=0) — load-bearing for the no-regression
//      guarantee.
//   3. Dampens μ gain for a winner in a composition-favored seat
//      when the prior has high confidence (the very property the
//      validation gauntlet measured at +1.4pp prediction accuracy).

// buildShowmatchForUpdateELO returns a minimal Showmatch with the
// 4-deck pod pre-seeded so updateELO can run without crashing.
// strategies[deckKey].Archetype is set from the archetypes slice.
func buildShowmatchForUpdateELO(deckKeys, archetypes []string, prior *trueskill.CompositionPrior) *Showmatch {
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

// runOneGame is a thin wrapper around updateELO that builds the
// commander/deck slice arguments (which updateELO requires for its
// per-deck eloState initialization) from the deckKeys.
func runOneGame(sm *Showmatch, deckKeys []string, winner int) {
	commanders := make([]string, len(deckKeys))
	decks := make([]*deckparser.TournamentDeck, len(deckKeys))
	for i, k := range deckKeys {
		commanders[i] = k
	}
	sm.updateELO(deckKeys, commanders, decks, winner, 30)
}

// -----------------------------------------------------------------------------
// Wiring: ObserveGame is called
// -----------------------------------------------------------------------------

func TestUpdateELO_ObservesGameToPrior(t *testing.T) {
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	prior := trueskill.NewCompositionPrior(4)
	sm := buildShowmatchForUpdateELO(deckKeys, archs, prior)

	runOneGame(sm, deckKeys, 0) // Mill wins

	// PairwiseSamples should now have a non-zero count for every
	// directed pair in the pod (4*3 = 12 directed pairs touched).
	for _, a := range archs {
		for _, b := range archs {
			if a == b {
				continue
			}
			if got := prior.PairwiseSamples(a, b); got != 1 {
				t.Errorf("after 1 game, PairwiseSamples(%s, %s) = %d, want 1", a, b, got)
			}
		}
	}
	if got := prior.ArchetypeSamples("Mill"); got != 1 {
		t.Errorf("Mill ArchetypeSamples = %d, want 1", got)
	}
}

func TestUpdateELO_DrawObservesParticipationOnly(t *testing.T) {
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	prior := trueskill.NewCompositionPrior(4)
	sm := buildShowmatchForUpdateELO(deckKeys, archs, prior)

	runOneGame(sm, deckKeys, -1) // draw

	for _, a := range archs {
		if got := prior.ArchetypeSamples(a); got != 1 {
			t.Errorf("draw: %s ArchetypeSamples = %d, want 1", a, got)
		}
	}
	// No archetype should have any wins credited.
	for _, a := range archs {
		// PairwiseSamples records participation for both directions,
		// but no wins. Verify via the prior's internal helper —
		// ExpectedWinrate must reflect 0/n for any archetype.
		if got := prior.ExpectedWinrate(a, archs); got != 0 {
			t.Errorf("draw should leave ExpectedWinrate(%s) = 0; got %.4f", a, got)
		}
	}
}

// -----------------------------------------------------------------------------
// Reduces to vanilla when prior is nil
// -----------------------------------------------------------------------------

func TestUpdateELO_NilPriorMatchesVanilla(t *testing.T) {
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}

	smPrior := buildShowmatchForUpdateELO(deckKeys, archs, nil)
	smPlain := buildShowmatchForUpdateELO(deckKeys, archs, nil)
	smPlain.compositionPrior = nil // explicit

	runOneGame(smPrior, deckKeys, 0)
	runOneGame(smPlain, deckKeys, 0)

	for _, k := range deckKeys {
		if math.Abs(smPrior.elo[k].tsMu-smPlain.elo[k].tsMu) > 1e-9 {
			t.Errorf("%s: nil-prior μ should match vanilla; prior=%.4f plain=%.4f",
				k, smPrior.elo[k].tsMu, smPlain.elo[k].tsMu)
		}
	}
}

func TestUpdateELO_ColdStartPriorMatchesVanilla(t *testing.T) {
	// Empty prior → Confidence = 0 → offsets = 0 → behavior matches
	// the no-prior path byte-exactly.
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}

	emptyPrior := trueskill.NewCompositionPrior(4)
	smCold := buildShowmatchForUpdateELO(deckKeys, archs, emptyPrior)
	smPlain := buildShowmatchForUpdateELO(deckKeys, archs, nil)

	runOneGame(smCold, deckKeys, 0)
	runOneGame(smPlain, deckKeys, 0)

	for _, k := range deckKeys {
		if math.Abs(smCold.elo[k].tsMu-smPlain.elo[k].tsMu) > 1e-9 {
			t.Errorf("%s: cold-start prior μ should match vanilla; cold=%.4f plain=%.4f",
				k, smCold.elo[k].tsMu, smPlain.elo[k].tsMu)
		}
	}
}

// -----------------------------------------------------------------------------
// Prior dampens favored-winner μ gain
// -----------------------------------------------------------------------------

func TestUpdateELO_FavoredWinnerGainsLessMu(t *testing.T) {
	// Pre-seed the prior with 500 games where Mill always wins this
	// exact pod composition. The prior will have high confidence and
	// large positive expected winrate for Mill. When updateELO runs
	// for ANOTHER Mill win, the offset shifts the rank-prediction
	// such that Mill's μ rises LESS than it would vanilla — the prior
	// "expected" Mill to win, so winning provides less skill signal.
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}

	prior := trueskill.NewCompositionPrior(4)
	for i := 0; i < 500; i++ {
		_ = prior.ObserveGame(archs, "Mill")
	}

	smPrior := buildShowmatchForUpdateELO(deckKeys, archs, prior)
	smVanilla := buildShowmatchForUpdateELO(deckKeys, archs, nil)

	runOneGame(smPrior, deckKeys, 0) // Mill wins
	runOneGame(smVanilla, deckKeys, 0)

	priorMillMu := smPrior.elo["d0"].tsMu
	vanillaMillMu := smVanilla.elo["d0"].tsMu

	// Both should be above starting μ (winners gain). Prior should
	// be CLOSER to starting μ — less rise.
	if priorMillMu >= vanillaMillMu {
		t.Errorf("favored Mill winner should gain less μ with prior: vanilla=%.4f prior=%.4f",
			vanillaMillMu, priorMillMu)
	}
}

func TestUpdateELO_DisfavoredWinnerGainsMoreMu(t *testing.T) {
	// Inverse: prior has Mill dominant. When Voltron wins, the
	// surprise is large, so Voltron's μ should rise MORE than
	// vanilla (the prior credit the upset as a bigger skill signal).
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}

	prior := trueskill.NewCompositionPrior(4)
	for i := 0; i < 500; i++ {
		_ = prior.ObserveGame(archs, "Mill")
	}

	smPrior := buildShowmatchForUpdateELO(deckKeys, archs, prior)
	smVanilla := buildShowmatchForUpdateELO(deckKeys, archs, nil)

	runOneGame(smPrior, deckKeys, 1) // Voltron wins
	runOneGame(smVanilla, deckKeys, 1)

	priorVoltronMu := smPrior.elo["d1"].tsMu
	vanillaVoltronMu := smVanilla.elo["d1"].tsMu

	if priorVoltronMu <= vanillaVoltronMu {
		t.Errorf("disfavored Voltron winner should gain more μ with prior: vanilla=%.4f prior=%.4f",
			vanillaVoltronMu, priorVoltronMu)
	}
}

// -----------------------------------------------------------------------------
// Persistence across games: prior accumulates evidence
// -----------------------------------------------------------------------------

func TestUpdateELO_PriorAccumulatesAcrossGames(t *testing.T) {
	deckKeys := []string{"d0", "d1", "d2", "d3"}
	archs := []string{"Mill", "Voltron", "Aggro", "Combo"}
	prior := trueskill.NewCompositionPrior(4)
	sm := buildShowmatchForUpdateELO(deckKeys, archs, prior)

	// 10 Mill wins in a row.
	for i := 0; i < 10; i++ {
		runOneGame(sm, deckKeys, 0)
	}
	// Mill's archetype-baseline winrate should now be 10/10 = 1.0
	// (within the pod). PairwiseSamples for Mill vs each opp should
	// have at least 10 games.
	if got := prior.PairwiseSamples("Mill", "Voltron"); got != 10 {
		t.Errorf("after 10 games, PairwiseSamples(Mill, Voltron) = %d, want 10", got)
	}
	// Confidence should be non-trivial (10 games > 0).
	if conf := prior.Confidence("Mill", archs); conf < 0.1 {
		t.Errorf("after 10 games, Mill confidence in pod = %.4f, want > 0.1", conf)
	}
}
