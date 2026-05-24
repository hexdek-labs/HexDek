package tournament

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/deckid"
	"github.com/hexdek/hexdek/internal/deckparser"
)

// DeckSimilarity returns the Jaccard similarity (|A ∩ B| / |A ∪ B|) of
// two decks' card rosters, treating each unique card name as a single
// set element. Range: 0.0 (no shared cards) to 1.0 (identical card list).
//
// We deliberately drop multiplicity (30 Plains in both decks counts
// once, not 30×) because basic-land counts otherwise dominate the
// metric — two Voltron decks with completely different threats but
// identical 30-Plains mana bases would read as 30%-similar even when
// their gameplans are unrelated. Set-Jaccard better tracks
// "structural" similarity for Commander pods: shared commander +
// shared signature cards push the score up; shared lands don't.
//
// Reference values for Commander (~100-card singleton-ish decks):
//
//	Identical:                    1.00
//	Same commander, otherwise disjoint: ~0.01
//	Same archetype, different builds:   ~0.20–0.35
//	Near-clones (~80% shared):           ~0.65–0.75
//	Identical builds (mirror):          ≥0.95
func DeckSimilarity(a, b *deckparser.TournamentDeck) float64 {
	if a == nil || b == nil {
		return 0
	}
	setA := deckCardSet(a)
	setB := deckCardSet(b)
	if len(setA) == 0 && len(setB) == 0 {
		return 1.0
	}
	inter := 0
	for name := range setA {
		if setB[name] {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// deckCardSet returns the unique-card-name set for a deck. Commander
// names are prefixed (matching deckid.CardList's canonical form) so
// "Atraxa as commander" doesn't accidentally collide with "Atraxa in
// 99". Built on top of deckid.CardList to share the existing
// normalization (accent-folding, casing, whitespace).
func deckCardSet(d *deckparser.TournamentDeck) map[string]bool {
	names := deckid.CardList(d.CommanderCards, d.Library)
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// MaxPodSimilarity returns the highest pairwise DeckSimilarity among
// any two decks in the proposed pod. Used by SeedPod's rejection
// sampler; exported so callers can score arbitrary pods (e.g. for
// post-hoc audits of a tournament's pod composition).
func MaxPodSimilarity(decks []*deckparser.TournamentDeck, pod []int) float64 {
	maxSim := 0.0
	for i := 0; i < len(pod); i++ {
		for j := i + 1; j < len(pod); j++ {
			s := DeckSimilarity(decks[pod[i]], decks[pod[j]])
			if s > maxSim {
				maxSim = s
			}
		}
	}
	return maxSim
}

// defaultSeedPodMaxAttempts caps how many random reshuffles SeedPod
// will try before giving up and returning the last-attempted pod.
// 32 is enough for any realistic similarity threshold + deck-pool
// combination: even when ~10% of pairs collide, the probability of
// 32 consecutive collisions is < 1e-32.
const defaultSeedPodMaxAttempts = 32

// SeedPod picks `nSeats` deck indices from `allDecks` such that no
// pair in the pod has DeckSimilarity > maxSimilarity. Falls back to
// a best-effort pod (whichever attempt produced the lowest max
// similarity) if no fully-valid pod is found in
// defaultSeedPodMaxAttempts shuffles.
//
// maxSimilarity ≤ 0 disables the constraint entirely (returns the
// first uniform-random pod), preserving the legacy unbiased sampling
// for callers that don't set the field.
//
// The fallback path lets the tournament still make progress when
// (e.g.) every deck in the pool is a clone of one prototype — a
// pathological case that would otherwise hang the producer goroutine.
// Callers should detect via len(MaxPodSimilarity(...) > threshold)
// post-hoc if they need to log the fallback.
func SeedPod(allDecks []*deckparser.TournamentDeck, nSeats int, rng *rand.Rand, maxSimilarity float64) []int {
	if rng == nil {
		// Defensive: the production caller always passes a seeded RNG,
		// but tests sometimes pass nil to assert on deterministic
		// pre-validated lists.
		return nil
	}
	nDecks := len(allDecks)
	if nSeats <= 0 || nDecks < nSeats {
		return nil
	}

	pickFirst := func() []int {
		perm := rng.Perm(nDecks)
		idxs := make([]int, nSeats)
		copy(idxs, perm[:nSeats])
		return idxs
	}

	if maxSimilarity <= 0 {
		return pickFirst()
	}

	var bestPod []int
	bestSim := 2.0 // Sentinel above any real similarity (max is 1.0).
	for attempt := 0; attempt < defaultSeedPodMaxAttempts; attempt++ {
		pod := pickFirst()
		sim := MaxPodSimilarity(allDecks, pod)
		if sim <= maxSimilarity {
			return pod
		}
		if sim < bestSim {
			bestSim = sim
			bestPod = pod
		}
	}
	return bestPod
}
