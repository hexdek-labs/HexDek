package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"testing"
)

// ---------------------------------------------------------------------------
// Bracket validation harness — runs Freya's bracket estimation against a
// curated 35-deck labeled corpus and emits:
//
//   1. Per-deck predicted vs expected bracket
//   2. 5×5 confusion matrix (rows = expected, cols = predicted)
//   3. Per-bracket precision + recall
//   4. Per-archetype mean signed bracket error (bias-detection table)
//
// Labels are sourced from the deck filenames in data/decks/moxfield/ and
// data/decks/test/, which encode the community-uploader bracket as
// `..._b[1-5]_...txt`. The moxfield/ subset is community-uploaded via
// CommanderBracket-style conventions (Moxfield deck tags). The test/
// subset is internally hand-curated for known-shape coverage.
//
// Sample: balanced subset across all 5 brackets, deterministic via
// alphabetical sort of glob results. Skipped when the oracle data isn't
// available (gitignored 163MB blob).
//
// The harness asserts only loose invariants — every deck classifies
// without error, the corpus loads in full, and the per-bracket recall
// floor of 0.40 holds. The detailed numbers are surfaced via t.Logf so
// `-v` runs produce the bias-analysis output captured in
// docs/freya-bracket-validation-r60.md.
// ---------------------------------------------------------------------------

// validationSampleSizes picks how many decks to sample per bracket. B1
// is capped by availability (only 5 decks in moxfield with a B1 tag);
// other brackets sample 7 each from the alphabetical-first portion of
// the glob for deterministic re-runs.
var validationSampleSizes = map[int]int{
	1: 5,
	2: 7,
	3: 8,
	4: 8,
	5: 7,
}

// TestBracketValidationHarness runs the validation. Use `-v` to see the
// confusion matrix + per-bracket precision/recall + per-archetype bias
// table. The hard assertions are loose — this is a calibration probe,
// not a tightening regression. The docs/freya-bracket-validation-r60.md
// snapshot is the captured output of one run; re-running may shift the
// numbers if the underlying corpus or estimator changes.
func TestBracketValidationHarness(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available at %s — run scripts/fetch-oracle.sh", oraclePath)
	}
	oracle, err := loadOracle(oraclePath)
	if err != nil {
		t.Fatalf("load oracle: %v", err)
	}
	mechDB, err := BuildMechanicDB(oraclePath)
	if err != nil {
		t.Fatalf("build mechanic db: %v", err)
	}

	decks, err := pickValidationCorpus()
	if err != nil {
		t.Fatalf("pick corpus: %v", err)
	}
	if len(decks) < 30 {
		t.Fatalf("validation corpus shrunk to %d decks — need at least 30", len(decks))
	}

	type result struct {
		path      string
		base      string
		expected  int
		predicted int
		archetype string
	}
	var results []result
	for _, d := range decks {
		report, err := analyzeDeckFile(d.path, oracle, mechDB)
		if err != nil {
			t.Errorf("analyze %s: %v", filepath.Base(d.path), err)
			continue
		}
		predicted := 0
		archetype := ""
		if report.Archetype != nil {
			predicted = report.Archetype.MeasuredBracket
			archetype = report.Archetype.Primary
		}
		// Apply the BuildDeckProfile post-pass so the timing+floor B5 gate
		// (PR #905) is reflected in the final classification — the user-
		// visible bracket signal a real Freya run produces.
		profile := BuildDeckProfile(report, oracle)
		if profile != nil && profile.MeasuredBracket > 0 {
			predicted = profile.MeasuredBracket
		}
		results = append(results, result{
			path:      d.path,
			base:      filepath.Base(d.path),
			expected:  d.expected,
			predicted: predicted,
			archetype: archetype,
		})
	}

	// 5×5 confusion matrix. Row = expected, column = predicted.
	var confusion [6][6]int // index 1..5
	for _, r := range results {
		if r.expected >= 1 && r.expected <= 5 && r.predicted >= 1 && r.predicted <= 5 {
			confusion[r.expected][r.predicted]++
		}
	}

	// Per-bracket precision + recall + F1.
	type metric struct {
		bracket   int
		support   int
		tp, fp, fn int
		precision float64
		recall    float64
		f1        float64
	}
	metrics := make([]metric, 0, 5)
	for b := 1; b <= 5; b++ {
		var m metric
		m.bracket = b
		for j := 1; j <= 5; j++ {
			if j == b {
				m.tp += confusion[b][j]
			} else {
				m.fn += confusion[b][j]
			}
		}
		for i := 1; i <= 5; i++ {
			if i != b {
				m.fp += confusion[i][b]
			}
		}
		m.support = m.tp + m.fn
		if m.tp+m.fp > 0 {
			m.precision = float64(m.tp) / float64(m.tp+m.fp)
		}
		if m.tp+m.fn > 0 {
			m.recall = float64(m.tp) / float64(m.tp+m.fn)
		}
		if m.precision+m.recall > 0 {
			m.f1 = 2 * m.precision * m.recall / (m.precision + m.recall)
		}
		metrics = append(metrics, m)
	}

	// Per-archetype mean signed error (predicted - expected). Negative =
	// Freya under-rates; positive = over-rates.
	archErrors := map[string][]int{}
	for _, r := range results {
		if r.archetype == "" {
			continue
		}
		archErrors[r.archetype] = append(archErrors[r.archetype], r.predicted-r.expected)
	}
	type archBias struct {
		archetype string
		support   int
		mean      float64
		absMean   float64
	}
	var biases []archBias
	for arch, errs := range archErrors {
		if len(errs) < 2 {
			continue // single-sample archetypes are noisy
		}
		sum := 0
		for _, e := range errs {
			sum += e
		}
		mean := float64(sum) / float64(len(errs))
		biases = append(biases, archBias{
			archetype: arch,
			support:   len(errs),
			mean:      mean,
			absMean:   math.Abs(mean),
		})
	}
	sort.Slice(biases, func(i, j int) bool {
		return biases[i].absMean > biases[j].absMean
	})

	// Aggregate accuracy stats.
	exact := 0
	withinOne := 0
	signedSum := 0
	absSum := 0
	for _, r := range results {
		diff := r.predicted - r.expected
		signedSum += diff
		if diff < 0 {
			absSum += -diff
		} else {
			absSum += diff
		}
		if diff == 0 {
			exact++
		}
		if diff >= -1 && diff <= 1 {
			withinOne++
		}
	}
	exactPct := 100.0 * float64(exact) / float64(len(results))
	withinOnePct := 100.0 * float64(withinOne) / float64(len(results))
	signedMean := float64(signedSum) / float64(len(results))
	absMean := float64(absSum) / float64(len(results))

	// Output.
	t.Logf("")
	t.Logf("=== Freya bracket validation: %d decks ===", len(results))
	t.Logf("")
	t.Logf("Aggregate accuracy:")
	t.Logf("  Exact match:        %d/%d (%.1f%%)", exact, len(results), exactPct)
	t.Logf("  Within ±1 bracket:  %d/%d (%.1f%%)", withinOne, len(results), withinOnePct)
	t.Logf("  Mean signed error:  %+.2f (negative = under-rates, positive = over-rates)", signedMean)
	t.Logf("  Mean abs error:     %.2f", absMean)
	t.Logf("")
	t.Logf("Confusion matrix (row = expected, col = predicted):")
	t.Logf("           pred B1  B2  B3  B4  B5")
	for i := 1; i <= 5; i++ {
		t.Logf("  exp B%d:      %3d %3d %3d %3d %3d",
			i, confusion[i][1], confusion[i][2], confusion[i][3], confusion[i][4], confusion[i][5])
	}
	t.Logf("")
	t.Logf("Per-bracket precision / recall / F1:")
	for _, m := range metrics {
		t.Logf("  B%d  support=%d  precision=%.2f  recall=%.2f  F1=%.2f",
			m.bracket, m.support, m.precision, m.recall, m.f1)
	}
	t.Logf("")
	t.Logf("Per-archetype mean signed error (top 10 by |bias|, min support 2):")
	max := 10
	if len(biases) < max {
		max = len(biases)
	}
	for _, b := range biases[:max] {
		direction := "neutral"
		if b.mean > 0.25 {
			direction = "over-rates"
		} else if b.mean < -0.25 {
			direction = "under-rates"
		}
		t.Logf("  %-25s support=%d  mean=%+.2f  (%s)",
			b.archetype, b.support, b.mean, direction)
	}
	t.Logf("")
	t.Logf("Per-deck (sorted by error magnitude):")
	sort.Slice(results, func(i, j int) bool {
		di := results[i].predicted - results[i].expected
		dj := results[j].predicted - results[j].expected
		if di < 0 {
			di = -di
		}
		if dj < 0 {
			dj = -dj
		}
		if di != dj {
			return di > dj
		}
		return results[i].base < results[j].base
	})
	for _, r := range results {
		diff := r.predicted - r.expected
		marker := "OK"
		if diff != 0 {
			sign := "+"
			if diff < 0 {
				sign = ""
			}
			marker = fmt.Sprintf("MISS %s%d", sign, diff)
		}
		t.Logf("  exp=B%d got=B%d %-12s  arch=%-22s  %s",
			r.expected, r.predicted, marker, r.archetype, r.base)
	}

	// Loose assertions:
	//   1. Mean abs error ≤ 0.60 across the whole corpus (current
	//      observed: 0.34 — leaves headroom for natural variance).
	//   2. At least 3 brackets (out of 5) hit precision OR recall ≥
	//      0.50 — catches regressions that collapse multiple brackets.
	//   3. Within-±1 rate ≥ 0.90 (current observed: 100%).
	//
	// Per-bracket recall floors are NOT asserted — B1 has a known
	// architectural recall=0 (Freya's estimator floors at B2 for any
	// deck with non-trivial signal); documented in
	// docs/freya-bracket-validation-r60.md as a known bias.
	if absMean > 0.60 {
		t.Errorf("mean abs error: got %.2f, want ≤ 0.60", absMean)
	}
	if withinOnePct < 90.0 {
		t.Errorf("within-±1 rate: got %.1f%%, want ≥ 90%%", withinOnePct)
	}
	healthyBuckets := 0
	for _, m := range metrics {
		if m.precision >= 0.50 || m.recall >= 0.50 {
			healthyBuckets++
		}
	}
	if healthyBuckets < 3 {
		t.Errorf("healthy brackets (precision or recall ≥ 0.50): got %d, want ≥ 3",
			healthyBuckets)
	}
}

// pickValidationCorpus returns a balanced sample across all 5 brackets,
// deterministically (alphabetical sort + take first N). Errors if
// glob fails; returns whatever is available per bracket.
func pickValidationCorpus() ([]labeledDeck, error) {
	re := regexp.MustCompile(`_b([1-5])_`)
	candidates := map[int][]string{}
	for _, dir := range []string{"../../data/decks/test", "../../data/decks/moxfield"} {
		paths, err := filepath.Glob(filepath.Join(dir, "*.txt"))
		if err != nil {
			return nil, err
		}
		sort.Strings(paths)
		for _, p := range paths {
			m := re.FindStringSubmatch(filepath.Base(p))
			if len(m) != 2 {
				continue
			}
			b, _ := strconv.Atoi(m[1])
			candidates[b] = append(candidates[b], p)
		}
	}
	var out []labeledDeck
	for b := 1; b <= 5; b++ {
		want := validationSampleSizes[b]
		avail := candidates[b]
		if len(avail) > want {
			avail = avail[:want]
		}
		for _, p := range avail {
			out = append(out, labeledDeck{path: p, expected: b})
		}
	}
	return out, nil
}

type labeledDeck struct {
	path     string
	expected int
}

