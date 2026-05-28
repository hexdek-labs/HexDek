package party

import (
	"sort"

	"github.com/hexdek/hexdek/internal/podbalance"
)

// QueuedDeck is one entry in the matchmaking pool — a user's deck
// plus its Freya-classified cEDH power tier and tier confidence.
// The Owner string is opaque to the matchmaker but surfaced on
// MatchResult so callers can route the pod selection back to the
// originating users (device IDs / user IDs / party IDs / etc).
type QueuedDeck struct {
	Owner      string  // opaque caller-side ID (device, user, party)
	DeckName   string  // for display + pod balance verdict
	Tier       int     // 1-5 from Freya's ClassifyCEDHPowerTier
	Confidence float64 // 0-1 from Freya's tier confidence (PR #717)
}

// MatchResult is the matchmaker's selection for one 4-deck pod plus
// the queue remainder. SelectedPod contains the 4 QueuedDeck entries
// whose pod-balance score was highest; Remaining contains the rest
// (queued users not matched into this pod). Assessment is the
// podbalance struct for the selected pod, surfaced so callers can
// render the verdict ("Balanced pod: T3/T3/T4/T4 (avg 3.50)") and
// gate on the Balance band ("don't auto-launch lopsided pods").
type MatchResult struct {
	SelectedPod []QueuedDeck
	Remaining   []QueuedDeck
	Assessment  *podbalance.PodBalanceAssessment
}

// PodSize is the target pod size — Commander is canonically a
// 4-player format. Exported for tests + the (future) party-mode-
// 3-player extension that would need to override.
const PodSize = 4

// MatchBalancedPod selects the most balanced 4-deck pod from the
// queue pool. The "most balanced" pod is the one with:
//
//  1. Minimum TierSpread (max(tier) - min(tier) across the 4 decks).
//     A spread of 0-1 is "balanced" per podbalance.BalanceBandForSpread.
//  2. Tie-breaker: highest average tier confidence — when two pods
//     tie on spread, prefer the pod whose tier classifications are
//     more decisively measured. A boundary-tier pod is genuinely
//     more uncertain about whether it WILL play balanced.
//  3. Further tie-breaker: earliest-queued bias — when two pods tie
//     on both spread + avg confidence, prefer the pod containing the
//     earliest-queued user (lower index in the input pool). This
//     prevents starvation: a user who joined the queue first
//     shouldn't wait forever just because their tier sits between
//     two cleaner candidate pods.
//
// Returns nil when the pool has fewer than PodSize=4 entries (not
// enough decks to assemble a pod). Returns a MatchResult with the
// SelectedPod, the Remaining queue (input order preserved), and the
// PodBalanceAssessment when a pod is selected.
//
// Algorithm: enumerates all C(N, 4) combinations and scores each.
// For typical matchmaking pool sizes (N ≤ 20), C(20, 4) = 4,845
// — trivial. For larger pools the function still runs but at
// O(N^4) — callers expecting massive pools should batch-divide
// upstream.
func MatchBalancedPod(pool []QueuedDeck) *MatchResult {
	if len(pool) < PodSize {
		return nil
	}

	best := candidate{spread: 1 << 30} // start at "infinite" spread

	// 4-deep nested loop: enumerate all 4-combinations from the pool.
	// O(N^4) — fine for typical pool sizes. The PodSize=4 const is
	// hard-baked here; if we ever support 3-player pods this loop
	// nest needs to be generalized to recursive combination
	// enumeration.
	n := len(pool)
	for a := 0; a < n-3; a++ {
		for b := a + 1; b < n-2; b++ {
			for c := b + 1; c < n-1; c++ {
				for d := c + 1; d < n; d++ {
					pod := []podbalance.PodDeckTier{
						{DeckName: pool[a].DeckName, Tier: pool[a].Tier, Confidence: pool[a].Confidence},
						{DeckName: pool[b].DeckName, Tier: pool[b].Tier, Confidence: pool[b].Confidence},
						{DeckName: pool[c].DeckName, Tier: pool[c].Tier, Confidence: pool[c].Confidence},
						{DeckName: pool[d].DeckName, Tier: pool[d].Tier, Confidence: pool[d].Confidence},
					}
					asm := podbalance.AssessPodBalance(pod)
					cand := candidate{
						indexes:       [PodSize]int{a, b, c, d},
						spread:        asm.TierSpread,
						avgConfidence: asm.AvgConfidence,
						earliestIndex: a,
						assessment:    asm,
					}
					if betterCandidate(cand, best) {
						best = cand
					}
				}
			}
		}
	}

	selected := make([]QueuedDeck, PodSize)
	chosen := map[int]bool{}
	for i, idx := range best.indexes {
		selected[i] = pool[idx]
		chosen[idx] = true
	}

	remaining := make([]QueuedDeck, 0, len(pool)-PodSize)
	for i, qd := range pool {
		if !chosen[i] {
			remaining = append(remaining, qd)
		}
	}

	return &MatchResult{
		SelectedPod: selected,
		Remaining:   remaining,
		Assessment:  best.assessment,
	}
}

// betterCandidate decides whether `cand` beats `best` per the
// matchmaker's preference ordering:
//
//  1. Lower spread wins
//  2. On spread tie: higher avg confidence wins
//  3. On both ties: lower earliestIndex wins (starvation avoidance)
//
// When `best` is the sentinel (spread=1<<30), `cand` always wins —
// the first real candidate seeds the comparison.
func betterCandidate(cand, best candidate) bool {
	if cand.spread != best.spread {
		return cand.spread < best.spread
	}
	if cand.avgConfidence != best.avgConfidence {
		return cand.avgConfidence > best.avgConfidence
	}
	return cand.earliestIndex < best.earliestIndex
}

// candidate is shadowed by the named struct in MatchBalancedPod so
// betterCandidate has the same field set. Declared at package scope
// for the function-signature reference; the actual instance lives
// only inside MatchBalancedPod's closure.
type candidate struct {
	indexes       [PodSize]int
	spread        int
	avgConfidence float64
	earliestIndex int
	assessment    *podbalance.PodBalanceAssessment
}

// SortPoolByTier returns a copy of the pool sorted by tier ascending,
// then by confidence descending within the same tier. Useful for
// admin debugging ("show me the matchmaking queue grouped by tier")
// and for callers that want a stable display order without altering
// the input pool. The matchmaker itself does NOT pre-sort — the
// starvation tie-break depends on the original input order.
func SortPoolByTier(pool []QueuedDeck) []QueuedDeck {
	out := make([]QueuedDeck, len(pool))
	copy(out, pool)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier < out[j].Tier
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
}
