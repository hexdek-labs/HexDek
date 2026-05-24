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

// SeedPodOptions bundles the two orthogonal pod-seeding constraints:
//
//   - MaxSimilarity rejects pods containing near-clone decks (Jaccard
//     above the threshold; see DeckSimilarity).
//   - PreferOpposition + Archetypes biases the sampler toward pods
//     containing at least one natural-counter pair (combo↔stax,
//     control↔aggro, etc.; see OpposingArchetypes).
//
// Both constraints can be active simultaneously. The sampler tries to
// satisfy BOTH; if no fully-satisfying pod is found within
// defaultSeedPodMaxAttempts shuffles, the constraints relax in order:
// first opposition (preference, not requirement), then similarity
// (fallback to the lowest-collision pod observed). Tournaments always
// make progress.
type SeedPodOptions struct {
	// MaxSimilarity is the upper bound on DeckSimilarity between any
	// two decks in the returned pod. 0 disables.
	MaxSimilarity float64

	// PreferOpposition, when true, biases the sampler toward pods
	// containing at least one OpposingArchetypes pair. Requires
	// Archetypes to be populated; if Archetypes is nil/empty, this
	// flag has no effect (no opposition data to read).
	PreferOpposition bool

	// Archetypes is parallel to the allDecks slice passed to
	// SeedPodWithOptions: Archetypes[i] is the archetype tag for
	// allDecks[i]. Decks without a known archetype should carry
	// ArchetypeUnknown — OpposingArchetypes treats those as
	// non-opposing.
	Archetypes []Archetype
}

// SeedPod picks `nSeats` deck indices from `allDecks` such that no
// pair in the pod has DeckSimilarity > maxSimilarity. Thin wrapper
// around SeedPodWithOptions for back-compat with callers that
// predate the archetype-opposition logic; new callers should prefer
// SeedPodWithOptions.
func SeedPod(allDecks []*deckparser.TournamentDeck, nSeats int, rng *rand.Rand, maxSimilarity float64) []int {
	return SeedPodWithOptions(allDecks, nSeats, rng, SeedPodOptions{
		MaxSimilarity: maxSimilarity,
	})
}

// SeedPodWithOptions is the full-featured pod sampler. See
// SeedPodOptions for the constraint vocabulary. Returns nil for
// invalid inputs (nil rng, insufficient decks). When both constraints
// are off (zero-value options), behaves identically to a single
// rng.Perm(n)[:nSeats] draw — preserving the legacy unbiased path
// for callers that don't opt in.
//
// Constraint relaxation order on retry-budget exhaustion:
//
//  1. Try to satisfy both similarity AND opposition. First pod that
//     does so wins.
//  2. Track the best similarity-only pod (lowest max-pairwise) seen
//     across all attempts as a fallback.
//  3. After defaultSeedPodMaxAttempts shuffles, return the best
//     similarity-only fallback. Opposition is a preference, not a
//     requirement — pathological pools (all-clones, or all-same-
//     archetype) still get a pod and the tournament still runs.
func SeedPodWithOptions(allDecks []*deckparser.TournamentDeck, nSeats int, rng *rand.Rand, opts SeedPodOptions) []int {
	if rng == nil {
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

	// Fast path: both constraints disabled → one shuffle, no biasing.
	wantOpposition := opts.PreferOpposition && len(opts.Archetypes) > 0
	wantSimilarity := opts.MaxSimilarity > 0
	if !wantSimilarity && !wantOpposition {
		return pickFirst()
	}

	var bestPod []int
	bestSim := 2.0 // Sentinel above any real similarity (max is 1.0).
	for attempt := 0; attempt < defaultSeedPodMaxAttempts; attempt++ {
		pod := pickFirst()

		sim := 0.0
		if wantSimilarity {
			sim = MaxPodSimilarity(allDecks, pod)
		}
		simOK := !wantSimilarity || sim <= opts.MaxSimilarity

		oppOK := !wantOpposition || PodHasOpposingPair(opts.Archetypes, pod)

		if simOK && oppOK {
			return pod
		}
		// Track best similarity-only fallback. We don't track
		// opposition-only because opposition is binary (have it or
		// don't) — there's nothing meaningful to "improve toward."
		if simOK && sim < bestSim {
			bestSim = sim
			bestPod = pod
		}
	}
	if bestPod != nil {
		return bestPod
	}
	// Neither constraint satisfied within the budget — last-ditch
	// uniform draw so the producer goroutine doesn't return nil.
	return pickFirst()
}
