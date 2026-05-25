package trueskill

import (
	"fmt"
	"math"
)

// CompositionPrior is a composition-conditioned expected-winrate
// model for 4-player Commander pods. Implements the minimum-viable
// version of the design from PR #398 (`docs/composition-elo-prior-
// r60.md`): Option 1 — pairwise matchup approximation.
//
// Given a deck's archetype and the 4-archetype pod it's playing in,
// ExpectedWinrate returns the expected baseline winrate the deck
// should achieve before player skill enters the equation. TrueSkill
// updates can then score the residual (observed − expected) instead
// of raw winrate, which the meta-study (PR #322) proved is
// confounded with composition by ~10× more than with seat or skill.
//
// MVP scope: in-memory only (no SQLite persistence yet — that's a
// follow-up). Bootstrappable from the meta-study CSV or live-game
// observations via ObserveGame.
type CompositionPrior struct {
	// Pairwise records: (archetype_a, archetype_b) → wins-by-a /
	// games-where-both-played. Both directions stored so the
	// pairwise lookup is O(1) regardless of which archetype is the
	// query.
	matchupWins  map[archetypePair]int
	matchupGames map[archetypePair]int

	// Archetype baseline: archetype → total games / wins across all
	// observed pods. Used as the fallback when pairwise data is
	// sparse for a particular pod.
	archWins  map[string]int
	archGames map[string]int

	// Pod size — 4 for Commander, configurable for other formats.
	podSize int
}

// archetypePair is the canonical pairwise key. We store BOTH
// directions ((a,b) and (b,a)) so ExpectedWinrate doesn't need to
// canonicalize the lookup order.
type archetypePair struct {
	a, b string
}

// NewCompositionPrior returns an empty prior configured for the
// given pod size (4 for Commander). All ExpectedWinrate calls on a
// fresh prior return the uniform baseline (1/podSize) until either
// ObserveGame or a bootstrap loader fills the tables.
func NewCompositionPrior(podSize int) *CompositionPrior {
	if podSize < 2 {
		podSize = 4
	}
	return &CompositionPrior{
		matchupWins:  map[archetypePair]int{},
		matchupGames: map[archetypePair]int{},
		archWins:     map[string]int{},
		archGames:    map[string]int{},
		podSize:      podSize,
	}
}

// ExpectedWinrate returns the deck's expected winrate (0..1) in a
// pod with the given archetypes. The lookup tier order matches the
// design doc's recommendation for Option 1:
//
//  1. Pairwise approximation: mean of matchup_winrate(deck, opp)
//     over each opponent in the pod. Used when ≥1 pairwise cell
//     has data.
//  2. Archetype baseline: deck-archetype's mean winrate across all
//     observed pods. Used when no pairwise data exists for the
//     specific opponents.
//  3. Uniform fallback: 1/podSize (= 0.25 for 4-player). Used when
//     the archetype has never been observed at all.
//
// Opponents that equal the deck's own archetype (i.e. mirror seats)
// are skipped from the pairwise mean — a deck doesn't beat itself
// in expectation.
func (cp *CompositionPrior) ExpectedWinrate(deckArchetype string, pod []string) float64 {
	if cp == nil || deckArchetype == "" {
		return 1.0 / float64(cp.podSizeOrDefault())
	}
	pairTotal := 0.0
	pairCount := 0
	for _, opp := range pod {
		if opp == "" || opp == deckArchetype {
			continue
		}
		key := archetypePair{a: deckArchetype, b: opp}
		games := cp.matchupGames[key]
		if games == 0 {
			continue
		}
		wins := cp.matchupWins[key]
		pairTotal += float64(wins) / float64(games)
		pairCount++
	}
	if pairCount > 0 {
		return pairTotal / float64(pairCount)
	}
	// Pairwise miss → archetype baseline.
	if g := cp.archGames[deckArchetype]; g > 0 {
		return float64(cp.archWins[deckArchetype]) / float64(g)
	}
	// Total cold-start → uniform.
	return 1.0 / float64(cp.podSizeOrDefault())
}

// ObserveGame updates the prior with the outcome of one finished
// pod. `pod` is the list of archetypes that played (length should
// equal podSize, but the call accepts any 2+). `winnerArchetype`
// is the archetype that won; pass "" for a draw (only archetype-
// participation counts get updated, no win credit assigned).
//
// Returns an error if the pod is too small or the winner archetype
// isn't present in the pod (when non-empty).
func (cp *CompositionPrior) ObserveGame(pod []string, winnerArchetype string) error {
	if cp == nil {
		return fmt.Errorf("composition prior is nil")
	}
	if len(pod) < 2 {
		return fmt.Errorf("pod size %d too small", len(pod))
	}
	// Validate winner is in pod (only when non-empty).
	if winnerArchetype != "" {
		found := false
		for _, a := range pod {
			if a == winnerArchetype {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("winner archetype %q not in pod", winnerArchetype)
		}
	}

	// Update archetype baseline counters — every pod participant
	// gets one game, the winner gets one win.
	for _, a := range pod {
		if a == "" {
			continue
		}
		cp.archGames[a]++
		if a == winnerArchetype {
			cp.archWins[a]++
		}
	}

	// Update pairwise counters — for each unordered pair in the
	// pod, increment games both directions, and credit the winner.
	for i := 0; i < len(pod); i++ {
		for j := 0; j < len(pod); j++ {
			if i == j {
				continue
			}
			a, b := pod[i], pod[j]
			if a == "" || b == "" {
				continue
			}
			key := archetypePair{a: a, b: b}
			cp.matchupGames[key]++
			if a == winnerArchetype {
				cp.matchupWins[key]++
			}
		}
	}
	return nil
}

// Confidence returns a [0, 1] estimate of how trustworthy the
// prior's ExpectedWinrate is for this (archetype, pod) cell.
// Computed from the mean pairwise game count across opponents:
// more games = more trust = higher confidence, asymptoting to 1.0.
//
// The shape is 1 - exp(-n/k) where k=50 is the "half-trust" point —
// 50 games per pairwise cell brings confidence to ~63%, 150 games
// to ~95%. Callers (e.g. TrueSkill.Update) can use the confidence
// to weight how aggressively the prior shapes the rating update vs
// the player's own σ.
//
// Returns 0 for a cold-start (no pairwise data, no archetype data).
func (cp *CompositionPrior) Confidence(deckArchetype string, pod []string) float64 {
	if cp == nil || deckArchetype == "" {
		return 0
	}
	total := 0
	opps := 0
	for _, opp := range pod {
		if opp == "" || opp == deckArchetype {
			continue
		}
		key := archetypePair{a: deckArchetype, b: opp}
		total += cp.matchupGames[key]
		opps++
	}
	if opps == 0 || total == 0 {
		// Pairwise miss (no opponents OR no games against them) —
		// fall back to archetype baseline samples.
		baselineGames := cp.archGames[deckArchetype]
		return 1.0 - math.Exp(-float64(baselineGames)/confidenceHalfPoint)
	}
	avgGames := float64(total) / float64(opps)
	return 1.0 - math.Exp(-avgGames/confidenceHalfPoint)
}

// confidenceHalfPoint is the per-cell game count at which
// Confidence returns ~0.63. Picked from the meta-study's per-cell
// stderr behavior: at n=50 the stderr ≈ √(0.25/50) ≈ 7pp, which
// is the regime where the prior becomes meaningfully more
// informative than the uniform fallback. At n=150 stderr ≈ 4pp
// and confidence ≈ 0.95.
const confidenceHalfPoint = 50.0

// podSizeOrDefault returns the configured pod size, defaulting to
// 4 for Commander. Nil-safe.
func (cp *CompositionPrior) podSizeOrDefault() int {
	if cp == nil || cp.podSize == 0 {
		return 4
	}
	return cp.podSize
}

// PairwiseSamples returns the number of games observed for the
// specific archetype pair. Exposed for diagnostics and tests; the
// prior's main loop doesn't need it. Returns 0 for unseen pairs.
func (cp *CompositionPrior) PairwiseSamples(a, b string) int {
	if cp == nil {
		return 0
	}
	return cp.matchupGames[archetypePair{a: a, b: b}]
}

// ArchetypeSamples returns the total games observed for the
// archetype across all pods. Diagnostic helper.
func (cp *CompositionPrior) ArchetypeSamples(arch string) int {
	if cp == nil {
		return 0
	}
	return cp.archGames[arch]
}
