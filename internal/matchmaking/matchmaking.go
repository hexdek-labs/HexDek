// Package matchmaking implements rating-aware pod assembly for
// multiplayer tournaments. Instead of random round-robin, it selects
// pods that maximize information gain: decks with high uncertainty
// (large σ) are preferred, and rating-similar decks are grouped
// together for tighter pairwise comparisons.
package matchmaking

import (
	"math"
	"math/rand"
	"sort"
)

// DeckEntry represents a deck available for matchmaking.
type DeckEntry struct {
	Index     int
	Commander string
	Mu        float64
	Sigma     float64
	Games     int
	Bracket   int
}

// AssembleBracketPod selects nSeats decks from the pool, preferring
// decks within ±1 bracket of the seed. Uses soft bracket weighting:
// same-bracket gets full score, ±1 gets 50%, ±2+ gets 10%.
//
// Seed selection is bracket-density-aware (see seedDensityScore): a
// deck whose bracket has at least nSeats-1 other same-bracket peers
// in the pool gets full seed weight; sparser brackets get
// proportionally less. This stops a high-σ outlier (e.g. one B1 deck
// in a B5-cEDH-heavy pool) from seeding a pod that the 0.1
// cross-bracket floor would then fill with B5s — the historical
// failure mode where a casual B1 player got 3 cEDH opponents because
// no other B1s existed in the pool. Bracketless decks (Bracket == 0)
// skip the density check and keep their raw info-gain weight, so the
// signal is purely additive — never disadvantages decks that lack
// bracket metadata.
func AssembleBracketPod(rng *rand.Rand, pool []DeckEntry, nSeats int) []int {
	if len(pool) <= nSeats {
		indices := make([]int, len(pool))
		for i := range pool {
			indices[i] = pool[i].Index
		}
		return indices
	}

	scores := make([]float64, len(pool))
	for i, d := range pool {
		scores[i] = infoGainScore(d)
	}

	seedScores := make([]float64, len(pool))
	for i, d := range pool {
		seedScores[i] = scores[i] * seedDensityScore(pool, d.Bracket, nSeats)
	}

	seedIdx := weightedPick(rng, seedScores)
	seed := pool[seedIdx]

	type candidate struct {
		poolIdx int
		score   float64
	}
	var candidates []candidate
	for i, d := range pool {
		if i == seedIdx {
			continue
		}
		proximity := proximityScore(seed.Mu, d.Mu, seed.Sigma, d.Sigma)
		bracketW := bracketWeight(seed.Bracket, d.Bracket)
		candidates = append(candidates, candidate{
			poolIdx: i,
			score:   proximity * scores[i] * bracketW,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	topN := (nSeats - 1) * 3
	if topN > len(candidates) {
		topN = len(candidates)
	}
	topCands := candidates[:topN]
	topScores := make([]float64, len(topCands))
	for i, c := range topCands {
		topScores[i] = c.score
	}

	picked := map[int]bool{seedIdx: true}
	result := []int{pool[seedIdx].Index}

	for len(result) < nSeats && len(topCands) > 0 {
		idx := weightedPick(rng, topScores)
		pIdx := topCands[idx].poolIdx
		if picked[pIdx] {
			topCands = append(topCands[:idx], topCands[idx+1:]...)
			topScores = append(topScores[:idx], topScores[idx+1:]...)
			continue
		}
		picked[pIdx] = true
		result = append(result, pool[pIdx].Index)
		topCands = append(topCands[:idx], topCands[idx+1:]...)
		topScores = append(topScores[:idx], topScores[idx+1:]...)
	}

	if len(result) < nSeats {
		for i, d := range pool {
			if !picked[i] {
				picked[i] = true
				result = append(result, d.Index)
				if len(result) >= nSeats {
					break
				}
			}
		}
	}

	return result
}

// seedDensityScore returns a [0,1] multiplier applied to a candidate
// seed's info-gain score in AssembleBracketPod. The score is the
// fraction of nSeats-1 same-bracket peers the candidate has in the
// pool, clamped to 1.0 — a seed with enough same-bracket peers to
// fill the entire pod gets full weight; sparser brackets get
// linearly less. Bracketless seeds (bracket == 0) return 1.0
// unconditionally so unbracketed decks are not penalized — the new
// signal is strictly additive over the prior behavior.
//
// Example with nSeats=4 (so 3 peers needed):
//   - 5 B3 decks in pool, candidate is B3: peers=4, score=1.0
//   - 3 B3 decks in pool, candidate is B3: peers=2, score=0.67
//   - 1 B3 deck in pool, candidate is B3: peers=0, score=0.0
//     (candidate cannot seed a pure-bracket pod — depreferred so a
//     denser bracket's seed wins the weighted pick instead)
func seedDensityScore(pool []DeckEntry, bracket, nSeats int) float64 {
	if bracket == 0 || nSeats <= 1 {
		return 1.0
	}
	peers := 0
	for _, d := range pool {
		if d.Bracket == bracket {
			peers++
		}
	}
	// Subtract the candidate itself from the peer count.
	peers--
	if peers < 0 {
		peers = 0
	}
	needed := nSeats - 1
	if peers >= needed {
		return 1.0
	}
	return float64(peers) / float64(needed)
}

func bracketWeight(seedBracket, deckBracket int) float64 {
	if seedBracket == 0 || deckBracket == 0 {
		return 1.0
	}
	diff := seedBracket - deckBracket
	if diff < 0 {
		diff = -diff
	}
	switch diff {
	case 0:
		return 1.0
	case 1:
		return 0.5
	default:
		return 0.1
	}
}

// AssemblePod selects nSeats decks from the pool using rating-aware
// matchmaking. The algorithm:
//
//  1. Score each deck by information gain: high σ (uncertain) decks
//     get priority, with a bonus for low game count.
//  2. Pick a seed deck (weighted random by info gain score).
//  3. Fill remaining seats with decks closest in μ to the seed,
//     weighted by their own info gain.
//
// Returns indices into the original pool.
func AssemblePod(rng *rand.Rand, pool []DeckEntry, nSeats int) []int {
	if len(pool) <= nSeats {
		indices := make([]int, len(pool))
		for i := range pool {
			indices[i] = pool[i].Index
		}
		return indices
	}

	// Score each deck by information gain potential.
	scores := make([]float64, len(pool))
	for i, d := range pool {
		scores[i] = infoGainScore(d)
	}

	// Pick seed deck (weighted random by info gain).
	seedIdx := weightedPick(rng, scores)
	seed := pool[seedIdx]

	// Score remaining decks by proximity to seed + their own info gain.
	type candidate struct {
		poolIdx int
		score   float64
	}
	var candidates []candidate
	for i, d := range pool {
		if i == seedIdx {
			continue
		}
		proximity := proximityScore(seed.Mu, d.Mu, seed.Sigma, d.Sigma)
		candidates = append(candidates, candidate{
			poolIdx: i,
			score:   proximity * scores[i],
		})
	}

	// Sort by combined score descending, then pick top nSeats-1.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Add some randomness — don't always pick the absolute top, use
	// weighted selection from the top 3x candidates.
	topN := (nSeats - 1) * 3
	if topN > len(candidates) {
		topN = len(candidates)
	}
	topCands := candidates[:topN]
	topScores := make([]float64, len(topCands))
	for i, c := range topCands {
		topScores[i] = c.score
	}

	picked := map[int]bool{seedIdx: true}
	result := []int{pool[seedIdx].Index}

	for len(result) < nSeats && len(topCands) > 0 {
		idx := weightedPick(rng, topScores)
		pIdx := topCands[idx].poolIdx
		if picked[pIdx] {
			// Remove and retry.
			topCands = append(topCands[:idx], topCands[idx+1:]...)
			topScores = append(topScores[:idx], topScores[idx+1:]...)
			continue
		}
		picked[pIdx] = true
		result = append(result, pool[pIdx].Index)
		topCands = append(topCands[:idx], topCands[idx+1:]...)
		topScores = append(topScores[:idx], topScores[idx+1:]...)
	}

	// Fallback: if we couldn't fill, grab random remaining.
	if len(result) < nSeats {
		for i, d := range pool {
			if !picked[i] {
				picked[i] = true
				result = append(result, d.Index)
				if len(result) >= nSeats {
					break
				}
			}
		}
	}

	return result
}

// infoGainScore measures how much we'd learn from including this deck.
// High σ = uncertain = high info gain. Low games = exploration bonus.
func infoGainScore(d DeckEntry) float64 {
	sigmaScore := d.Sigma / 8.33
	explorationBonus := 1.0 / math.Max(float64(d.Games)+1, 1)
	return sigmaScore + explorationBonus*0.5
}

// proximityScore favors decks with similar ratings for tighter
// pairwise comparisons. The Gaussian kernel width adapts to the
// combined uncertainty of both decks.
func proximityScore(seedMu, deckMu, seedSigma, deckSigma float64) float64 {
	diff := seedMu - deckMu
	bandwidth := math.Max(seedSigma+deckSigma, 1.0)
	return math.Exp(-diff * diff / (2 * bandwidth * bandwidth))
}

func weightedPick(rng *rand.Rand, weights []float64) int {
	if len(weights) == 0 {
		return 0
	}
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return rng.Intn(len(weights))
	}
	r := rng.Float64() * total
	cum := 0.0
	for i, w := range weights {
		if w > 0 {
			cum += w
		}
		if cum >= r {
			return i
		}
	}
	return len(weights) - 1
}
