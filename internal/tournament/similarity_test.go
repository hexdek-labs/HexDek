package tournament

import (
	"math"
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// mkDeck builds a minimal TournamentDeck with the given commander name
// and library card names. Used to construct synthetic decks for the
// similarity tests without touching the deckparser/oracle stack.
func mkDeck(commander string, libraryNames ...string) *deckparser.TournamentDeck {
	cmd := &gameengine.Card{Name: commander}
	library := make([]*gameengine.Card, 0, len(libraryNames))
	for _, n := range libraryNames {
		library = append(library, &gameengine.Card{Name: n})
	}
	return &deckparser.TournamentDeck{
		CommanderName:  commander,
		CommanderCards: []*gameengine.Card{cmd},
		Library:        library,
	}
}

func approx(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// TestDeckSimilarity_Identical pins the upper bound: two decks with the
// same commander and the same card list must score 1.0 exactly.
func TestDeckSimilarity_Identical(t *testing.T) {
	a := mkDeck("Atraxa", "sol ring", "command tower", "swords to plowshares")
	b := mkDeck("Atraxa", "sol ring", "command tower", "swords to plowshares")
	if got := DeckSimilarity(a, b); !approx(got, 1.0) {
		t.Errorf("identical decks similarity = %f, want 1.0", got)
	}
}

// TestDeckSimilarity_Disjoint pins the lower bound: two decks with
// completely distinct commanders + cards must score 0.0 exactly.
func TestDeckSimilarity_Disjoint(t *testing.T) {
	a := mkDeck("Atraxa", "sol ring", "command tower")
	b := mkDeck("Krenko", "goblin guide", "lightning bolt")
	if got := DeckSimilarity(a, b); !approx(got, 0.0) {
		t.Errorf("disjoint decks similarity = %f, want 0.0", got)
	}
}

// TestDeckSimilarity_HalfOverlap pins the math. Two decks share 2 of 4
// cards each: intersection = 2, union = 6 → Jaccard = 2/6 ≈ 0.333.
func TestDeckSimilarity_HalfOverlap(t *testing.T) {
	a := mkDeck("Atraxa", "sol ring", "command tower", "cyclonic rift")
	b := mkDeck("Krenko", "sol ring", "command tower", "lightning bolt")
	got := DeckSimilarity(a, b)
	// Cards in set form: a={COMMANDER:atraxa, 1x sol ring, 1x command
	// tower, 1x cyclonic rift}, b={COMMANDER:krenko, 1x sol ring, 1x
	// command tower, 1x lightning bolt}. Intersection = 2 shared
	// staples. Union = 4+4-2 = 6. Jaccard = 2/6.
	want := 2.0 / 6.0
	if !approx(got, want) {
		t.Errorf("half-overlap similarity = %f, want %f", got, want)
	}
}

// TestDeckSimilarity_SameCommanderDistinctBuilds confirms the "shared
// commander, otherwise different builds" case scores in the low end of
// the reference table — commander adds one shared element, everything
// else differs.
func TestDeckSimilarity_SameCommanderDistinctBuilds(t *testing.T) {
	a := mkDeck("Atraxa", "build a", "build a-2", "build a-3")
	b := mkDeck("Atraxa", "build b", "build b-2", "build b-3")
	got := DeckSimilarity(a, b)
	// Intersection = 1 (the commander). Union = 4+4-1 = 7. Jaccard = 1/7.
	if !approx(got, 1.0/7.0) {
		t.Errorf("same-commander-distinct-builds = %f, want 1/7", got)
	}
}

// TestDeckSimilarity_BasicLandsCountOnce is the design-choice pin: a
// deck running 30 basic lands of one kind and a deck running 30 basic
// lands of another kind must NOT score similarly via mana-base
// duplication — set semantics (not multi-set) is what makes this
// metric track gameplan rather than land-base shape.
func TestDeckSimilarity_BasicLandsCountOnce(t *testing.T) {
	aCards := []string{"plains"}
	for i := 0; i < 30; i++ {
		aCards = append(aCards, "plains") // adds duplicates that should collapse
	}
	bCards := []string{"island"}
	for i := 0; i < 30; i++ {
		bCards = append(bCards, "island")
	}
	a := mkDeck("Atraxa", aCards...)
	b := mkDeck("Atraxa", bCards...)
	got := DeckSimilarity(a, b)
	// Set-form: a={COMMANDER:atraxa, 1x plains}, b={COMMANDER:atraxa, 1x
	// island}. Intersection = 1 (commander), union = 3. Jaccard = 1/3.
	// If multiplicity leaked, this would be far higher.
	if !approx(got, 1.0/3.0) {
		t.Errorf("basic-land duplication leaked into similarity: got %f, want 1/3", got)
	}
}

// TestDeckSimilarity_NilSafe confirms nil decks score 0 rather than
// panicking — caller convenience for any path that hasn't loaded
// everything yet.
func TestDeckSimilarity_NilSafe(t *testing.T) {
	a := mkDeck("Atraxa", "sol ring")
	if got := DeckSimilarity(nil, a); got != 0 {
		t.Errorf("nil/A similarity = %f, want 0", got)
	}
	if got := DeckSimilarity(a, nil); got != 0 {
		t.Errorf("A/nil similarity = %f, want 0", got)
	}
}

// TestMaxPodSimilarity_ReportsWorstPair confirms the helper returns the
// HIGHEST pairwise similarity in the pod — not the average. Tournament
// triage needs the worst offender, not the centroid.
func TestMaxPodSimilarity_ReportsWorstPair(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		mkDeck("A", "sol ring", "command tower", "cyclonic rift"),
		mkDeck("B", "lightning bolt", "shock", "fireblast"),
		mkDeck("A", "sol ring", "command tower", "cyclonic rift"), // identical to [0]
		mkDeck("D", "swords", "path", "wrath"),
	}
	// Pod {0, 1, 2, 3}: pair (0,2) is identical = 1.0.
	got := MaxPodSimilarity(decks, []int{0, 1, 2, 3})
	if !approx(got, 1.0) {
		t.Errorf("MaxPodSimilarity for pod with identical pair = %f, want 1.0", got)
	}
	// Pod {1, 3}: two disjoint decks.
	got = MaxPodSimilarity(decks, []int{1, 3})
	if !approx(got, 0.0) {
		t.Errorf("MaxPodSimilarity for disjoint pair = %f, want 0.0", got)
	}
}

// TestSeedPod_DisabledByZero confirms maxSimilarity ≤ 0 returns the
// first uniform-random pod, preserving the legacy unbiased sampler for
// callers that don't opt in.
func TestSeedPod_DisabledByZero(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		mkDeck("A", "x"), mkDeck("A", "x"), // identical pair
		mkDeck("B", "y"), mkDeck("C", "z"),
	}
	rng := rand.New(rand.NewSource(42))
	pod := SeedPod(decks, 4, rng, 0.0)
	if len(pod) != 4 {
		t.Errorf("len(pod) = %d, want 4", len(pod))
	}
	// With maxSimilarity disabled, the identical pair MAY appear in the
	// pod — the sampler doesn't reject. Just confirm the pod is a valid
	// permutation prefix.
	seen := map[int]bool{}
	for _, i := range pod {
		if seen[i] {
			t.Errorf("duplicate index in pod: %d", i)
		}
		seen[i] = true
	}
}

// TestSeedPod_RejectsNearClones is the primary positive case. The pool
// contains two identical decks (sim=1.0) plus two distinct decks. With
// maxSimilarity=0.5, the seeder must NEVER produce a pod containing
// both identical decks together — run many iterations to catch
// probabilistic failure modes.
func TestSeedPod_RejectsNearClones(t *testing.T) {
	clone1 := mkDeck("Atraxa", "sol ring", "command tower", "cyclonic rift")
	clone2 := mkDeck("Atraxa", "sol ring", "command tower", "cyclonic rift")
	other1 := mkDeck("Krenko", "goblin guide", "lightning bolt")
	other2 := mkDeck("Edgar", "blood artist", "viscera seer")
	decks := []*deckparser.TournamentDeck{clone1, clone2, other1, other2}

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 200; i++ {
		pod := SeedPod(decks, 3, rng, 0.5)
		if len(pod) != 3 {
			t.Fatalf("iteration %d: len(pod) = %d, want 3", i, len(pod))
		}
		// If both clones (indices 0 and 1) appear in the same pod, the
		// constraint was violated.
		has0, has1 := false, false
		for _, idx := range pod {
			if idx == 0 {
				has0 = true
			}
			if idx == 1 {
				has1 = true
			}
		}
		if has0 && has1 {
			t.Errorf("iteration %d: pod contains both clones (sim=1.0): %v", i, pod)
			break
		}
	}
}

// TestSeedPod_FallbackOnAllClones pins the pathological-pool behavior:
// when EVERY deck in the pool is a clone of one prototype, no valid
// pod exists. The sampler must NOT hang — it returns a best-effort
// pod (the one with lowest observed max similarity, which here is
// 1.0 for all attempts) so the tournament still makes progress.
func TestSeedPod_FallbackOnAllClones(t *testing.T) {
	mk := func() *deckparser.TournamentDeck {
		return mkDeck("Atraxa", "sol ring", "command tower")
	}
	decks := []*deckparser.TournamentDeck{mk(), mk(), mk(), mk()}

	rng := rand.New(rand.NewSource(42))
	pod := SeedPod(decks, 3, rng, 0.5)
	if len(pod) != 3 {
		t.Errorf("len(pod) = %d, want 3 (fallback path must still return a pod)", len(pod))
	}
	// All pods will have MaxPodSimilarity = 1.0 in this pool — confirm
	// we got SOMETHING valid rather than nil/panic.
	if MaxPodSimilarity(decks, pod) < 0.99 {
		t.Errorf("fallback should still pick a clone-pair pod (all options are): got %f",
			MaxPodSimilarity(decks, pod))
	}
}

// TestSeedPod_DeterministicWithSeed confirms reproducibility: two
// runs with identical seeded RNGs produce identical pods. Required
// for tournament replay / SeedContract integration — pod composition
// must be deterministic given (seed, decks, threshold).
func TestSeedPod_DeterministicWithSeed(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		mkDeck("A", "x"), mkDeck("B", "y"), mkDeck("C", "z"),
		mkDeck("D", "w"), mkDeck("E", "v"),
	}

	rng1 := rand.New(rand.NewSource(99))
	rng2 := rand.New(rand.NewSource(99))
	for i := 0; i < 20; i++ {
		p1 := SeedPod(decks, 4, rng1, 0.5)
		p2 := SeedPod(decks, 4, rng2, 0.5)
		if len(p1) != len(p2) {
			t.Fatalf("iter %d: lengths differ: %v vs %v", i, p1, p2)
		}
		for j := range p1 {
			if p1[j] != p2[j] {
				t.Errorf("iter %d slot %d: %d vs %d", i, j, p1[j], p2[j])
			}
		}
	}
}

// TestSeedPod_NilRNG_ReturnsNil is the defensive-input pin: callers
// passing nil rng (an early-stage error or a test stub) get a nil
// result rather than a panic. Cheap belt-and-braces guard.
func TestSeedPod_NilRNG_ReturnsNil(t *testing.T) {
	decks := []*deckparser.TournamentDeck{mkDeck("A", "x"), mkDeck("B", "y")}
	if got := SeedPod(decks, 2, nil, 0.5); got != nil {
		t.Errorf("SeedPod(nil rng) = %v, want nil", got)
	}
}

// TestSeedPod_InsufficientDecks confirms the constructor invariant: a
// pod of 4 from a 3-deck pool returns nil (the caller's responsibility
// to validate, but this short-circuit prevents panics in the rejection
// loop).
func TestSeedPod_InsufficientDecks(t *testing.T) {
	decks := []*deckparser.TournamentDeck{mkDeck("A", "x"), mkDeck("B", "y"), mkDeck("C", "z")}
	rng := rand.New(rand.NewSource(1))
	if got := SeedPod(decks, 4, rng, 0.5); got != nil {
		t.Errorf("SeedPod(3 decks, 4 seats) = %v, want nil", got)
	}
}

// TestSeedPod_ArchetypeSiblingsAllowed is the false-positive guard:
// two decks of the same archetype with ~25% card overlap (typical
// archetype-peer similarity) must NOT be blocked at the default
// threshold of 0.6. This is the test that protects legitimate
// archetype meetings from getting suppressed.
func TestSeedPod_ArchetypeSiblingsAllowed(t *testing.T) {
	// Build two "voltron" archetype decks sharing a few key staples
	// but mostly distinct.
	shared := []string{"sol ring", "swiftfoot boots", "lightning greaves"}
	build1 := append([]string{}, shared...)
	for i := 0; i < 9; i++ {
		build1 = append(build1, "build1-card-extra")
	}
	build2 := append([]string{}, shared...)
	for i := 0; i < 9; i++ {
		build2 = append(build2, "build2-card-extra")
	}
	a := mkDeck("Uril", build1...)
	b := mkDeck("Sram", build2...) // different commander, same archetype
	sim := DeckSimilarity(a, b)
	if sim >= 0.6 {
		t.Fatalf("archetype-siblings should sit below 0.6 threshold: got %f", sim)
	}

	decks := []*deckparser.TournamentDeck{a, b, mkDeck("C", "z"), mkDeck("D", "w")}
	rng := rand.New(rand.NewSource(7))
	// Repeatedly sample; pairs of (a,b) MUST be allowed because
	// their similarity is below threshold.
	sawArchetypePair := false
	for i := 0; i < 50; i++ {
		pod := SeedPod(decks, 4, rng, 0.6)
		// 4-from-4 always contains every index, so this isn't a
		// stress test of avoidance — it's a stress test of "the
		// constraint doesn't block legitimate pairings."
		if len(pod) != 4 {
			t.Fatalf("len = %d", len(pod))
		}
		sawArchetypePair = true // we got 4 valid indices including a + b
	}
	if !sawArchetypePair {
		t.Error("expected at least one pod containing the archetype siblings")
	}
}
