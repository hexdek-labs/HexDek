package shuffle

import (
	"math"
	"testing"
)

// chi_square_r60_test.go — Statistical fairness verification for the
// Fisher-Yates shuffle. The existing TestShuffleDistribution uses a
// coarse ±10% bucket-count tolerance which would let a noticeably biased
// shuffle slip through unnoticed. This file adds a Pearson chi-square
// goodness-of-fit test on the position distribution of each element
// across N=10000 shuffles.
//
// Methodology:
//   - Shuffle a deck of n=13 distinct elements 10000 times.
//   - Build counts[element][position] — a 13×13 contingency.
//   - For each element, compute χ² = Σ_p (O_p - E)² / E across positions
//     (df = n-1 = 12, expected E = trials/n ≈ 769.23).
//   - Reject fairness if ANY element's χ² exceeds the threshold.
//
// Threshold: χ² ≥ 49 at df=12 corresponds to α ≈ 1e-6 per element. With
// 13 independent elements tested, family-wise false-positive rate is
// ≈ 1.3e-5 — essentially never flaky on a genuinely fair shuffle, while
// still catching any real-world bias. Expected χ² under H_0 is 12 with
// std ≈ √24 ≈ 4.9, so 49 sits ~7.5σ above the null mean.
//
// To prove the test methodology actually detects bias, the companion
// test runs the same harness against a deliberately-biased "naive"
// shuffler (the classic textbook bug) and asserts the threshold is
// exceeded.

const (
	chiSquareN          = 13
	chiSquareTrials     = 10000
	chiSquareThreshold  = 49.0 // df=12, α≈1e-6
	chiSquareDfMinusOne = 12
)

// chiSquarePerElement runs `trials` shuffles of `[0..n)` using `shuf` and
// returns, for each element, the χ² statistic of its position
// distribution across the n positions. A fair shuffle should produce
// per-element χ² values centered on df = n-1.
func chiSquarePerElement(t *testing.T, n, trials int, shuf func([]int) error) []float64 {
	t.Helper()
	counts := make([][]int, n)
	for i := range counts {
		counts[i] = make([]int, n)
	}
	for trial := 0; trial < trials; trial++ {
		deck := make([]int, n)
		for i := range deck {
			deck[i] = i
		}
		if err := shuf(deck); err != nil {
			t.Fatalf("trial %d: shuffle failed: %v", trial, err)
		}
		for pos, elem := range deck {
			counts[elem][pos]++
		}
	}
	expected := float64(trials) / float64(n)
	chi := make([]float64, n)
	for elem := 0; elem < n; elem++ {
		var s float64
		for pos := 0; pos < n; pos++ {
			d := float64(counts[elem][pos]) - expected
			s += (d * d) / expected
		}
		chi[elem] = s
	}
	return chi
}

func TestShuffleFairness_ChiSquarePositionDistribution(t *testing.T) {
	chi := chiSquarePerElement(t, chiSquareN, chiSquareTrials, Shuffle[int])

	var maxChi float64
	var maxElem int
	var sum float64
	for elem, c := range chi {
		if c > maxChi {
			maxChi = c
			maxElem = elem
		}
		sum += c
		if c > chiSquareThreshold {
			t.Errorf("element %d position-distribution χ² = %.2f exceeds threshold %.1f "+
				"(df=%d, α≈1e-6); shuffle is not uniform",
				elem, c, chiSquareThreshold, chiSquareDfMinusOne)
		}
	}
	avg := sum / float64(chiSquareN)
	t.Logf("chi-square fairness: n=%d trials=%d, avg χ² per element = %.2f (expected ≈ %d under H_0), max χ² = %.2f at element %d",
		chiSquareN, chiSquareTrials, avg, chiSquareDfMinusOne, maxChi, maxElem)

	// Mean check: average per-element χ² should be in the ballpark of df.
	// For df=12, std-of-mean across 13 elements ≈ √(2·12/13) ≈ 1.36; a ±6
	// window (~4.4σ) catches gross systematic skew without flaking on
	// honest variance.
	if math.Abs(avg-float64(chiSquareDfMinusOne)) > 6.0 {
		t.Errorf("average χ² across all elements = %.2f, expected within 6.0 of df=%d "+
			"(systematic skew indicates non-uniform shuffle)",
			avg, chiSquareDfMinusOne)
	}
}

// naiveBiasedShuffle is the classic textbook Fisher-Yates BUG: at each
// step pick a random index from the WHOLE array [0, n) instead of the
// unshuffled tail [i, n). Produces n^n outcomes weighted unevenly across
// the n! permutations, with the most common bias being earlier indices
// over-represented in their original-or-near-original positions.
//
// Used purely as a methodology sanity check — if our χ² harness can
// detect THIS, it can detect any subtler real-world bias in Shuffle.
func naiveBiasedShuffle(cards []int) error {
	n := len(cards)
	for i := 0; i < n; i++ {
		j, err := randIntN(n)
		if err != nil {
			return err
		}
		cards[i], cards[j] = cards[j], cards[i]
	}
	return nil
}

func TestShuffleFairness_ChiSquareDetectsBias(t *testing.T) {
	// Meta-test: run the same χ² harness against the naive-buggy shuffler
	// and assert at least one element's χ² exceeds the threshold. This
	// proves the threshold is tight enough to catch real bias, so the
	// passing TestShuffleFairness_ChiSquarePositionDistribution above
	// isn't a vacuous test that would let bias through.
	chi := chiSquarePerElement(t, chiSquareN, chiSquareTrials, naiveBiasedShuffle)

	var maxChi float64
	var maxElem int
	exceeded := 0
	for elem, c := range chi {
		if c > maxChi {
			maxChi = c
			maxElem = elem
		}
		if c > chiSquareThreshold {
			exceeded++
		}
	}
	t.Logf("naive-biased shuffle: max χ² = %.2f at element %d, %d/%d elements exceeded threshold %.1f",
		maxChi, maxElem, exceeded, chiSquareN, chiSquareThreshold)
	if exceeded == 0 {
		t.Fatalf("naive biased shuffle did NOT trip the χ² threshold at any element "+
			"(max χ² = %.2f, threshold %.1f). The fairness test methodology is too "+
			"lax — it would not catch a real shuffle bug either.",
			maxChi, chiSquareThreshold)
	}
}
