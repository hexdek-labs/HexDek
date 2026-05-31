package trueskill

import (
	"math"
	"testing"
)

// R60 4P-FFA tied-losers fix.
//
// Pre-fix `UpdateMultiplayer` walked rank-sorted adjacency, so for the
// dominant 4P Commander outcome shape — ranks [0, 1, 1, 1] (one winner,
// three tied losers because seat eliminations rarely play out to a strict
// 4-way ordering) — the winner only got a decisive bump against ONE
// loser and the other two losers received zero decisive-loss signal.
// The showmatch updateELO path papered that over by triple-running
// `Update2Player(winner, loser_i)` pairwise, but that gave each loser a
// FULL 1v1 decisive loss against an already-chained winner — losers
// collapsed ~3× faster than 4P FFA's 25% per-seat win expectation
// justifies.
//
// Fix: all-pairs decomposition. For [0, 1, 1, 1] that produces 3 winner-
// vs-loser decisive comparisons + 3 loser-vs-loser draws. The draws among
// equal-μ losers wash out; the winner accumulates ~3·Δ; each loser eats
// ~Δ. Signal mass is conserved (Σ Δμ ≈ 0), and per-loser loss is roughly
// Δ_winner / 3 — matching the rank-shape reality.

// TestUpdateMultiplayer_TiedLosers_4P pins the headline R60 invariants
// for the [0, 1, 1, 1] rank shape: winner mu UP, every loser mu DOWN,
// each loser-mu-shrink ≈ winner-mu-gain / 3 (signal mass conservation),
// equal losers shrink by equal amounts (symmetry).
func TestUpdateMultiplayer_TiedLosers_4P(t *testing.T) {
	cfg := DefaultFFAConfig()
	ratings := []Rating{
		DefaultRating(), // winner
		DefaultRating(), // tied loser 1
		DefaultRating(), // tied loser 2
		DefaultRating(), // tied loser 3
	}
	ranks := []int{0, 1, 1, 1}

	updated := UpdateMultiplayer(cfg, ratings, ranks)

	winnerGain := updated[0].Mu - ratings[0].Mu
	if winnerGain <= 0 {
		t.Fatalf("winner mu must increase, got Δ=%f", winnerGain)
	}

	loserDeltas := []float64{
		updated[1].Mu - ratings[1].Mu,
		updated[2].Mu - ratings[2].Mu,
		updated[3].Mu - ratings[3].Mu,
	}
	for i, d := range loserDeltas {
		if d >= 0 {
			t.Fatalf("loser %d mu must decrease, got Δ=%f", i+1, d)
		}
	}

	// Symmetry: identical starting ratings + identical rank → identical
	// post-update μ for every loser.
	for i := 1; i < len(loserDeltas); i++ {
		if math.Abs(loserDeltas[0]-loserDeltas[i]) > 1e-9 {
			t.Fatalf("equal-rank losers must shrink by equal amount: L1 Δ=%f, L%d Δ=%f",
				loserDeltas[0], i+1, loserDeltas[i])
		}
	}

	// Signal-mass conservation: ΣΔμ across all four seats ≈ 0 (the
	// 3 winner-vs-loser pairs contribute +x and -x each, the 3 draws
	// among equal losers contribute ≈0).
	totalDelta := winnerGain + loserDeltas[0] + loserDeltas[1] + loserDeltas[2]
	if math.Abs(totalDelta) > 0.05 {
		t.Errorf("signal mass not conserved: Σ Δμ = %f (expected ≈ 0)", totalDelta)
	}

	// Per-loser shrink ≈ winner gain / 3 — the headline R60 calibration
	// invariant. Allow 5% tolerance: dynamics-noise (tau) and the
	// truncated-Gaussian non-linearity introduce small asymmetry between
	// the +x winner side and -x summed loser side.
	expectedLoserDelta := -winnerGain / 3.0
	for i, d := range loserDeltas {
		ratio := d / expectedLoserDelta
		if ratio < 0.95 || ratio > 1.05 {
			t.Errorf("loser %d Δ=%f, expected ≈ %f (winner_gain/3), ratio=%f",
				i+1, d, expectedLoserDelta, ratio)
		}
	}
}

// TestUpdateMultiplayer_TiedLosers_VsPreFixHack_OrderIndependence pins
// the symmetry guarantee the decoupled-pairwise R60 fix delivers vs the
// pre-fix chained-pairwise hack. Under the hack, winner.σ shrinks after
// the first Update2Player, so subsequent (winner, L_j) pairs see a
// smaller c, producing different μ-deltas for L1 vs L2 vs L3 — losers
// who happen to be processed later eat asymmetric updates purely due to
// seat-iteration order. The corrected path uses starting-state for every
// pair, so iteration order has no effect.
func TestUpdateMultiplayer_TiedLosers_VsPreFixHack_OrderIndependence(t *testing.T) {
	cfg := DefaultFFAConfig()
	ratings := []Rating{
		DefaultRating(), DefaultRating(), DefaultRating(), DefaultRating(),
	}
	ranks := []int{0, 1, 1, 1}

	// Corrected path: all three losers must shrink by the same amount.
	correct := UpdateMultiplayer(cfg, ratings, ranks)
	d1 := ratings[1].Mu - correct[1].Mu
	d3 := ratings[3].Mu - correct[3].Mu
	if math.Abs(d1-d3) > 1e-9 {
		t.Fatalf("decoupled all-pairs must give equal-rank losers identical Δμ: L1=%f L3=%f", d1, d3)
	}

	// Pre-fix chained hack: process losers serially with a shared
	// winner-rating accumulator. Track each loser's resulting μ.
	wR := ratings[0]
	losers := []Rating{ratings[1], ratings[2], ratings[3]}
	for i := range losers {
		wNew, lNew := Update2Player(cfg, wR, losers[i])
		wR = wNew
		losers[i] = lNew
	}
	hackD1 := ratings[1].Mu - losers[0].Mu
	hackD3 := ratings[3].Mu - losers[2].Mu

	// The hack must give DIFFERENT shrink to L1 vs L3 — that's the
	// order-dependence bug the fix closes. Confirm it does, otherwise
	// the test isn't actually exercising the issue.
	if math.Abs(hackD1-hackD3) < 0.05 {
		t.Errorf("expected pre-fix chained-pairwise to give order-asymmetric loser shrink; "+
			"L1=%f L3=%f (Δ=%f, expected ≥0.05)", hackD1, hackD3, math.Abs(hackD1-hackD3))
	}
}

// TestUpdateMultiplayer_AdjacencyChainRetired pins the rank-shape that
// most exposed the adjacency-chain bug: [0, 1, 1, 1]. Under the old
// adjacency chain, the third loser saw ZERO decisive signal — only a
// draw against the second loser. Confirm the new path gives all three
// losers a real loss, not a draw artifact.
func TestUpdateMultiplayer_AdjacencyChainRetired(t *testing.T) {
	cfg := DefaultFFAConfig()
	ratings := []Rating{
		DefaultRating(), DefaultRating(), DefaultRating(), DefaultRating(),
	}
	ranks := []int{0, 1, 1, 1}
	updated := UpdateMultiplayer(cfg, ratings, ranks)

	// Compare loser-3 against a same-config draw between two
	// DefaultRating players — that's the only signal the old adjacency
	// chain would have given loser 3.
	a, _ := UpdateDraw(cfg, DefaultRating(), DefaultRating())
	drawShrink := math.Abs(a.Mu - DefaultRating().Mu)
	loser3Shrink := math.Abs(updated[3].Mu - DefaultRating().Mu)

	if loser3Shrink <= drawShrink*5 {
		t.Errorf("loser 3 shrink (%f) must be much larger than draw-only shrink (%f) — "+
			"adjacency-chain regression suspected", loser3Shrink, drawShrink)
	}
}

// TestUpdateMultiplayer_4PNonTied is the regression guard on the strict-
// ordering rank shape [0, 1, 2, 3] — the existing 4P test pins ordering
// but not magnitudes. Ranking is preserved AND signal mass is roughly
// conserved.
func TestUpdateMultiplayer_4PNonTied(t *testing.T) {
	cfg := DefaultFFAConfig()
	ratings := []Rating{
		DefaultRating(), DefaultRating(), DefaultRating(), DefaultRating(),
	}
	ranks := []int{0, 1, 2, 3}
	updated := UpdateMultiplayer(cfg, ratings, ranks)

	for i := 0; i < 3; i++ {
		if updated[i].Mu <= updated[i+1].Mu {
			t.Errorf("rank %d (μ=%f) must rate higher than rank %d (μ=%f)",
				i, updated[i].Mu, i+1, updated[i+1].Mu)
		}
	}

	var total float64
	for i := range updated {
		total += updated[i].Mu - ratings[i].Mu
	}
	if math.Abs(total) > 0.05 {
		t.Errorf("signal mass not conserved on strict-ordering 4P: Σ Δμ = %f", total)
	}
}

// TestUpdateMultiplayer_AllDraw pins the all-draw shape [0, 0, 0, 0]:
// every pair is a draw, identical ratings, μ-changes are essentially
// zero. Used to test the no-winner branch (timeouts) cleanly.
func TestUpdateMultiplayer_AllDraw(t *testing.T) {
	cfg := DefaultFFAConfig()
	ratings := []Rating{
		DefaultRating(), DefaultRating(), DefaultRating(), DefaultRating(),
	}
	ranks := []int{0, 0, 0, 0}
	updated := UpdateMultiplayer(cfg, ratings, ranks)

	for i := range updated {
		if math.Abs(updated[i].Mu-ratings[i].Mu) > 0.01 {
			t.Errorf("all-draw equal-rating Δμ[%d]=%f, expected ≈ 0",
				i, updated[i].Mu-ratings[i].Mu)
		}
	}
}

// TestUpdateMultiplayerWithOffsets_FfaShape pins the composition-prior
// wrapper's correctness on the same [0, 1, 1, 1] rank shape: when all
// offsets are 0 it equals UpdateMultiplayer; non-zero offsets shift the
// rank-prediction but the σ-update is invariant.
func TestUpdateMultiplayerWithOffsets_FfaShape(t *testing.T) {
	cfg := DefaultFFAConfig()
	ratings := []Rating{
		DefaultRating(), DefaultRating(), DefaultRating(), DefaultRating(),
	}
	ranks := []int{0, 1, 1, 1}

	// Zero offsets path == vanilla UpdateMultiplayer.
	zeroOffsets := []float64{0, 0, 0, 0}
	withZero := UpdateMultiplayerWithOffsets(cfg, ratings, ranks, zeroOffsets)
	plain := UpdateMultiplayer(cfg, ratings, ranks)
	for i := range withZero {
		if math.Abs(withZero[i].Mu-plain[i].Mu) > 1e-9 {
			t.Errorf("zero-offsets path diverged from vanilla at i=%d: %f vs %f",
				i, withZero[i].Mu, plain[i].Mu)
		}
	}

	// Positive winner offset → winner gains LESS (the prior already
	// "expected" them to win, so the residual update is smaller).
	posOffsets := []float64{+1.0, 0, 0, 0}
	withPos := UpdateMultiplayerWithOffsets(cfg, ratings, ranks, posOffsets)
	if withPos[0].Mu-ratings[0].Mu >= withZero[0].Mu-ratings[0].Mu {
		t.Errorf("positive winner offset should DAMPEN winner's gain; zero-off=%f, pos-off=%f",
			withZero[0].Mu-ratings[0].Mu, withPos[0].Mu-ratings[0].Mu)
	}
}

// TestUpdateMultiplayerWithOffsets_LengthMismatchFallback confirms a
// short/long offsets slice falls back to vanilla UpdateMultiplayer
// rather than panicking or silently mis-attributing offsets.
func TestUpdateMultiplayerWithOffsets_LengthMismatchFallback(t *testing.T) {
	cfg := DefaultFFAConfig()
	ratings := []Rating{DefaultRating(), DefaultRating(), DefaultRating(), DefaultRating()}
	ranks := []int{0, 1, 1, 1}

	short := UpdateMultiplayerWithOffsets(cfg, ratings, ranks, []float64{1.0, 0, 0})
	plain := UpdateMultiplayer(cfg, ratings, ranks)
	for i := range short {
		if math.Abs(short[i].Mu-plain[i].Mu) > 1e-9 {
			t.Errorf("length-mismatch offsets did not fall back to vanilla at i=%d", i)
		}
	}
}
