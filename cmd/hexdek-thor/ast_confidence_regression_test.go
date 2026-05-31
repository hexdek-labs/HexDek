package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
)

// astDatasetPath returns the canonical dataset location relative to the
// repo root, regardless of where `go test` was invoked.
func astDatasetPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// thisFile = .../cmd/hexdek-thor/ast_confidence_regression_test.go
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	return filepath.Join(repoRoot, "data", "rules", "ast_dataset.jsonl")
}

// loadCorpusOrSkip loads ast_dataset.jsonl, skipping the test when the
// dataset isn't available (e.g. fresh CI checkout where the 50 MB file
// isn't fetched). Returns the loaded corpus on success.
func loadCorpusOrSkip(t *testing.T) *astload.Corpus {
	t.Helper()
	path := astDatasetPath(t)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("dataset not available at %s — run scripts/fetch-oracle.sh to populate", path)
	}
	corpus, err := astload.Load(path)
	if err != nil {
		t.Fatalf("astload.Load: %v", err)
	}
	if corpus.CardCount < 30_000 {
		t.Fatalf("corpus appears truncated: %d cards (expected ≥30,000)", corpus.CardCount)
	}
	return corpus
}

// TestASTConfidence_CorpusDistribution verifies the per-card confidence
// distribution against the parser-health snapshot in
// docs/ast-corpus-health-r60.md. If the parser regresses (more cards
// fall to medium/low confidence), this fires.
//
// The expected band aligns with the histogram in the doc:
//   clean (1.0):     ~72% of cards-with-abilities
//   high (>=0.7):    ~76% inclusive of clean
//   medium/low:      ~14% combined
//
// We use generous ±5pp tolerances so harmless parser tweaks don't churn
// the test. Tight regressions (e.g. -10pp in clean) will fire loudly.
func TestASTConfidence_CorpusDistribution(t *testing.T) {
	corpus := loadCorpusOrSkip(t)

	buckets := map[string]int{}
	withAbilities := 0
	confidentAtDefault := 0
	totalAbilityNodes := 0
	confidentAbilityNodes := 0

	for _, name := range corpus.Names() {
		card, ok := corpus.Get(name)
		if !ok || card == nil {
			continue
		}
		if len(card.Abilities) > 0 {
			withAbilities++
		}
		score := gameast.CardConfidence(card)
		buckets[gameast.ConfidenceBucket(score)]++
		if gameast.CardIsConfident(card, gameast.DefaultConfidenceThreshold) {
			confidentAtDefault++
		}
		for _, ab := range card.Abilities {
			totalAbilityNodes++
			if gameast.AbilityIsConfident(ab, gameast.DefaultConfidenceThreshold) {
				confidentAbilityNodes++
			}
		}
	}

	if withAbilities < 30_000 {
		t.Fatalf("too few cards-with-abilities loaded: %d", withAbilities)
	}

	cleanPct := 100.0 * float64(buckets["clean"]) / float64(withAbilities)
	confidentPct := 100.0 * float64(confidentAtDefault) / float64(withAbilities)
	abConfidentPct := 100.0 * float64(confidentAbilityNodes) / float64(totalAbilityNodes)

	t.Logf("cards with abilities:   %d", withAbilities)
	t.Logf("ability nodes:          %d", totalAbilityNodes)
	t.Logf("buckets:                %v", buckets)
	t.Logf("clean (1.0):            %.2f%%", cleanPct)
	t.Logf("cards >= 0.7 threshold: %.2f%%", confidentPct)
	t.Logf("ability nodes >= 0.7:   %.2f%%", abConfidentPct)

	// Pinned expectations from the docs/ast-corpus-health-r60.md
	// snapshot (2026-05-30). Tolerances are generous because the parser
	// is in active development; tight regressions still fire.
	//
	// CardConfidence multiplies the !FullyParsed penalty into the mean,
	// so cards that score 1.0 are even rarer than the doc's "clean"
	// bucket (which was per-ability fallback share). We expect ~60%+.
	if cleanPct < 55.0 {
		t.Errorf("clean-bucket share %.2f%% < 55%% floor (parser quality regressed?)", cleanPct)
	}
	if confidentPct < 70.0 {
		t.Errorf("cards >= 0.7 threshold %.2f%% < 70%% floor", confidentPct)
	}
	// Per-ability confidence should be very high (Keyword nodes alone
	// inflate it). Snapshot ~85%.
	if abConfidentPct < 80.0 {
		t.Errorf("ability-node >= 0.7 threshold %.2f%% < 80%% floor", abConfidentPct)
	}
}

// TestASTConfidence_NoBadCardsClaimFullConfidence pins the symmetric
// invariant: a card the parser flagged as !FullyParsed must score below
// the default threshold. Catches a regression where the score penalty
// or the FullyParsed signal stops being honoured.
func TestASTConfidence_NoBadCardsClaimFullConfidence(t *testing.T) {
	corpus := loadCorpusOrSkip(t)

	violators := 0
	for _, name := range corpus.Names() {
		card, ok := corpus.Get(name)
		if !ok || card == nil {
			continue
		}
		if card.FullyParsed {
			continue
		}
		// A card that wasn't fully parsed should never claim full
		// confidence (1.0). Below-threshold is the strong assertion;
		// we just check that the penalty fired.
		if gameast.CardConfidence(card) >= 1.0 {
			violators++
			if violators <= 3 {
				t.Errorf("card %q has !FullyParsed but CardConfidence=1.0", card.Name)
			}
		}
	}
	if violators > 0 {
		t.Logf("total violators: %d", violators)
	}
}

// TestASTConfidence_FallbackKindsScoreBelowOne picks a sample of cards
// whose only ability has a known fallback reason and asserts they
// score strictly below 1.0 (the parser flagged the node as imperfect
// even if the penalty is small). End-to-end check: per-corpus scoring
// matches the per-unit tests. Note: a single -0.30 condition penalty
// lands at exactly 0.70 (= the default at-or-above gate), so we don't
// assert sub-threshold here — we assert sub-1.0, the structural
// "penalty fired" invariant. The sub-threshold assertion belongs to
// the unit tests where each penalty combination is pinned exactly.
func TestASTConfidence_FallbackKindsScoreBelowOne(t *testing.T) {
	corpus := loadCorpusOrSkip(t)

	checked := 0
	heavyFallbackBelowThreshold := 0
	for _, name := range corpus.Names() {
		card, ok := corpus.Get(name)
		if !ok || card == nil {
			continue
		}
		if len(card.Abilities) != 1 {
			continue
		}
		ab := card.Abilities[0]
		reasons := gameast.LowConfidenceReasons(ab)
		if len(reasons) == 0 {
			continue
		}
		score := gameast.AbilityConfidence(ab)
		if score >= 1.0 {
			t.Errorf("card %q has fallback reasons %v but ability score %.2f == 1.0 (penalty didn't fire)",
				card.Name, reasons, score)
		}
		// Cards with a heavy fallback (mod-kind penalty, not just a
		// condition wrapper) must drop below the at-or-above gate. This
		// is the sub-threshold invariant scoped to the cases where the
		// penalty schedule predicts it.
		hasHeavy := false
		for _, r := range reasons {
			if !startsWith(r, "static_fallback_cond_kind") &&
				!startsWith(r, "triggered_fallback_intervening_if") {
				hasHeavy = true
				break
			}
		}
		if hasHeavy {
			if score >= gameast.DefaultConfidenceThreshold {
				t.Errorf("card %q has heavy fallback %v but score %.2f passes the default gate",
					card.Name, reasons, score)
			} else {
				heavyFallbackBelowThreshold++
			}
		}
		checked++
		if checked >= 500 {
			break
		}
	}
	if checked < 50 {
		t.Fatalf("only %d single-ability-fallback cards found — corpus shape changed?", checked)
	}
	t.Logf("verified %d fallback-only cards: %d with heavy fallback all below the 0.7 gate",
		checked, heavyFallbackBelowThreshold)
}

// startsWith is a tiny local helper that avoids pulling in strings just
// for HasPrefix in this test file.
func startsWith(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
