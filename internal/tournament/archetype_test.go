package tournament

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
)

// TestOpposingArchetypes_SymmetryAndCanonicalPairs pins the symmetric
// contract — opposing(a, b) must equal opposing(b, a) — and verifies
// the canonical rock-paper-scissors pairings documented at the table:
// combo↔stax, combo↔control, storm↔stax, storm↔control,
// aggro_wide↔control, aggro_wide↔stax, voltron↔theft,
// reanimator↔{control,stax}, aristocrats↔stax,
// spellslinger↔stax, tribal↔control, enchantress↔control,
// mill↔control, superfriends↔{aggro_wide,voltron}.
func TestOpposingArchetypes_SymmetryAndCanonicalPairs(t *testing.T) {
	cases := []struct {
		a, b Archetype
		want bool
	}{
		{ArchetypeComboInfinite, ArchetypeStax, true},
		{ArchetypeStax, ArchetypeComboInfinite, true}, // symmetry
		{ArchetypeComboInfinite, ArchetypeControl, true},
		{ArchetypeStorm, ArchetypeStax, true},
		{ArchetypeStorm, ArchetypeControl, true},
		{ArchetypeAggroGoWide, ArchetypeControl, true},
		{ArchetypeAggroGoWide, ArchetypeStax, true},
		{ArchetypeVoltron, ArchetypeTheftClone, true},
		{ArchetypeReanimator, ArchetypeControl, true},
		{ArchetypeReanimator, ArchetypeStax, true},
		{ArchetypeAristocrats, ArchetypeStax, true},
		{ArchetypeSpellslinger, ArchetypeStax, true},
		{ArchetypeTribal, ArchetypeControl, true},
		{ArchetypeEnchantress, ArchetypeControl, true},
		{ArchetypeMill, ArchetypeControl, true},
		{ArchetypeSuperfriends, ArchetypeAggroGoWide, true},
		{ArchetypeSuperfriends, ArchetypeVoltron, true},

		// Same archetype is never opposing (you don't counter
		// yourself).
		{ArchetypeComboInfinite, ArchetypeComboInfinite, false},
		{ArchetypeControl, ArchetypeControl, false},

		// Non-pairs (or "no documented opposition") return false.
		{ArchetypeVoltron, ArchetypeAggroGoWide, false},
		{ArchetypeLifegain, ArchetypeMill, false},
		{ArchetypeBlinkFlicker, ArchetypeStorm, false},

		// Unknown short-circuits to false even against documented
		// opposers.
		{ArchetypeUnknown, ArchetypeStax, false},
		{ArchetypeComboInfinite, ArchetypeUnknown, false},
	}
	for _, c := range cases {
		if got := OpposingArchetypes(c.a, c.b); got != c.want {
			t.Errorf("OpposingArchetypes(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// TestNormalizeArchetype_CasingAndWhitespace pins the round-trip
// helper. Freya report dumps occasionally surface lowercased or
// padded archetype strings; NormalizeArchetype must absorb both
// without losing the canonical mapping.
func TestNormalizeArchetype_CasingAndWhitespace(t *testing.T) {
	cases := []struct {
		in   string
		want Archetype
	}{
		{"Combo / Infinite", ArchetypeComboInfinite},
		{"combo / infinite", ArchetypeComboInfinite},
		{"  Stax  ", ArchetypeStax},
		{"AGGRO / GO WIDE", ArchetypeAggroGoWide},
		{"", ArchetypeUnknown},
		{"unrelated string", ArchetypeUnknown},
	}
	for _, c := range cases {
		if got := NormalizeArchetype(c.in); got != c.want {
			t.Errorf("NormalizeArchetype(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestPodHasOpposingPair_DetectsOnePairAnywhere confirms the helper
// returns true on the FIRST opposing pair it finds, regardless of
// pod position. Tournament gauntlets just need "does this pod have
// at least one matchup?" — count-of-pairs would be over-engineering.
func TestPodHasOpposingPair_DetectsOnePairAnywhere(t *testing.T) {
	archetypes := []Archetype{
		ArchetypeComboInfinite, // 0
		ArchetypeStorm,         // 1
		ArchetypeStax,          // 2 — opposes 0 and 1
		ArchetypeLifegain,      // 3
	}
	if !PodHasOpposingPair(archetypes, []int{0, 2}) {
		t.Error("combo+stax pair should be opposing")
	}
	if !PodHasOpposingPair(archetypes, []int{1, 3, 2}) {
		t.Error("storm+stax in larger pod should be opposing")
	}
	if PodHasOpposingPair(archetypes, []int{0, 1, 3}) {
		t.Error("no opposing pair in {combo, storm, lifegain}: false positive")
	}
}

// TestPodHasOpposingPair_EmptyInputs is the defensive-input guard.
// Empty archetypes / pod-of-1 / out-of-range indices must all return
// false without panicking.
func TestPodHasOpposingPair_EmptyInputs(t *testing.T) {
	if PodHasOpposingPair(nil, []int{0, 1}) {
		t.Error("nil archetypes should yield false")
	}
	if PodHasOpposingPair([]Archetype{ArchetypeStax}, []int{0}) {
		t.Error("single-deck pod can't have a pair")
	}
	// Out-of-range indices are silently skipped.
	if PodHasOpposingPair([]Archetype{ArchetypeStax, ArchetypeComboInfinite},
		[]int{0, 1, 5}) != true {
		t.Error("legitimate pair should still be detected with one OOB index")
	}
}

// TestSeedPodWithOptions_OppositionBiasesSelection is the primary
// positive case for the archetype path. With one combo and one stax
// (opposing) plus two lifegain decks (non-opposing), the sampler
// MUST eventually pick a pod containing both the combo and the
// stax. Run many iterations to catch probabilistic failure modes.
func TestSeedPodWithOptions_OppositionBiasesSelection(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		mkDeck("Combo Cmdr", "thoracle"),    // 0 — combo
		mkDeck("Lifegain1 Cmdr", "soul warden"), // 1 — lifegain
		mkDeck("Stax Cmdr", "winter orb"),   // 2 — stax (opposes combo)
		mkDeck("Lifegain2 Cmdr", "ajani's pridemate"), // 3 — lifegain
	}
	archs := []Archetype{
		ArchetypeComboInfinite,
		ArchetypeLifegain,
		ArchetypeStax,
		ArchetypeLifegain,
	}
	opts := SeedPodOptions{
		PreferOpposition: true,
		Archetypes:       archs,
	}

	rng := rand.New(rand.NewSource(42))
	// 200 iterations; every pod must contain the combo+stax pair
	// (the sampler MUST find them — they're the only opposing pair).
	for i := 0; i < 200; i++ {
		pod := SeedPodWithOptions(decks, 3, rng, opts)
		if len(pod) != 3 {
			t.Fatalf("iter %d: len(pod) = %d, want 3", i, len(pod))
		}
		has0, has2 := false, false
		for _, idx := range pod {
			if idx == 0 {
				has0 = true
			}
			if idx == 2 {
				has2 = true
			}
		}
		if !(has0 && has2) {
			t.Errorf("iter %d: pod %v missing combo (0) or stax (2)", i, pod)
			break
		}
	}
}

// TestSeedPodWithOptions_OppositionFallback confirms graceful
// degradation: when NO opposing pairs exist in the pool (all decks
// are the same archetype), the sampler doesn't hang — it returns a
// regular uniform-random pod after exhausting the retry budget.
func TestSeedPodWithOptions_OppositionFallback(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		mkDeck("A", "x"), mkDeck("B", "y"), mkDeck("C", "z"), mkDeck("D", "w"),
	}
	archs := []Archetype{
		ArchetypeLifegain, ArchetypeLifegain, ArchetypeLifegain, ArchetypeLifegain,
	}
	opts := SeedPodOptions{PreferOpposition: true, Archetypes: archs}

	rng := rand.New(rand.NewSource(42))
	pod := SeedPodWithOptions(decks, 3, rng, opts)
	if len(pod) != 3 {
		t.Errorf("len(pod) = %d, want 3 (fallback path must still return a pod)", len(pod))
	}
}

// TestSeedPodWithOptions_OppositionAndSimilarityCompose pins the
// orthogonal-constraint contract. Two near-clone combo decks and one
// stax deck: the sampler must (a) avoid pairing the two clones, AND
// (b) include the stax deck for opposition. Only one pod satisfies
// both: {clone1 OR clone2, stax, third deck}.
func TestSeedPodWithOptions_OppositionAndSimilarityCompose(t *testing.T) {
	clone1 := mkDeck("Combo1", "thoracle", "demonic consultation", "tainted pact")
	clone2 := mkDeck("Combo2", "thoracle", "demonic consultation", "tainted pact") // sim=1.0 to clone1
	stax := mkDeck("Stax", "winter orb", "stasis", "smokestack")
	mid := mkDeck("Mid", "rhystic study", "smothering tithe")
	decks := []*deckparser.TournamentDeck{clone1, clone2, stax, mid}
	archs := []Archetype{
		ArchetypeComboInfinite, ArchetypeComboInfinite,
		ArchetypeStax, ArchetypeLifegain,
	}
	opts := SeedPodOptions{
		MaxSimilarity:    0.5,
		PreferOpposition: true,
		Archetypes:       archs,
	}

	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 100; i++ {
		pod := SeedPodWithOptions(decks, 3, rng, opts)
		if len(pod) != 3 {
			t.Fatalf("iter %d: len = %d", i, len(pod))
		}
		// Similarity: clones (0 and 1) must NOT both appear.
		has0, has1 := false, false
		hasStax := false
		for _, idx := range pod {
			if idx == 0 {
				has0 = true
			}
			if idx == 1 {
				has1 = true
			}
			if idx == 2 {
				hasStax = true
			}
		}
		if has0 && has1 {
			t.Errorf("iter %d: pod contains both clones: %v", i, pod)
		}
		// Opposition: stax (idx 2) must be present to oppose one of the combos.
		if !hasStax {
			t.Errorf("iter %d: pod missing stax (idx 2): %v", i, pod)
		}
	}
}

// TestSeedPod_BackCompat_UnaffectedByOpposition confirms the back-
// compat wrapper SeedPod(...) still behaves identically to its #145
// contract — opposition logic is opt-in via SeedPodWithOptions. A
// caller using the old signature gets the same uniform-random
// behavior they always did.
func TestSeedPod_BackCompat_UnaffectedByOpposition(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		mkDeck("A", "x"), mkDeck("B", "y"),
		mkDeck("C", "z"), mkDeck("D", "w"), mkDeck("E", "v"),
	}

	rng1 := rand.New(rand.NewSource(123))
	rng2 := rand.New(rand.NewSource(123))
	for i := 0; i < 10; i++ {
		oldAPI := SeedPod(decks, 3, rng1, 0.5)
		newAPI := SeedPodWithOptions(decks, 3, rng2,
			SeedPodOptions{MaxSimilarity: 0.5})
		if len(oldAPI) != len(newAPI) {
			t.Fatalf("iter %d: lengths differ %d vs %d", i, len(oldAPI), len(newAPI))
		}
		for j := range oldAPI {
			if oldAPI[j] != newAPI[j] {
				t.Errorf("iter %d slot %d: %d vs %d", i, j, oldAPI[j], newAPI[j])
			}
		}
	}
}

// TestSeedPodWithOptions_DeterministicWithSeed pins reproducibility
// under the combined-constraint path. Identical seed + decks +
// archetypes + opts → identical pod sequence. Critical for
// SeedContract integration and replay-based audits.
func TestSeedPodWithOptions_DeterministicWithSeed(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		mkDeck("Combo", "thoracle"), mkDeck("Stax", "stasis"),
		mkDeck("Aggro", "goblin guide"), mkDeck("Control", "counterspell"),
		mkDeck("Reanimator", "reanimate"),
	}
	archs := []Archetype{
		ArchetypeComboInfinite, ArchetypeStax,
		ArchetypeAggroGoWide, ArchetypeControl, ArchetypeReanimator,
	}
	opts := SeedPodOptions{
		MaxSimilarity:    0.5,
		PreferOpposition: true,
		Archetypes:       archs,
	}

	rng1 := rand.New(rand.NewSource(99))
	rng2 := rand.New(rand.NewSource(99))
	for i := 0; i < 30; i++ {
		p1 := SeedPodWithOptions(decks, 4, rng1, opts)
		p2 := SeedPodWithOptions(decks, 4, rng2, opts)
		if len(p1) != len(p2) {
			t.Fatalf("iter %d: lengths differ", i)
		}
		for j := range p1 {
			if p1[j] != p2[j] {
				t.Errorf("iter %d slot %d: %d vs %d", i, j, p1[j], p2[j])
			}
		}
	}
}

// TestSeedPodWithOptions_AllZero_IsLegacyPath confirms the
// zero-value options struct behaves identically to a uniform-random
// rng.Perm draw. Important so existing PoolMode callers who don't
// set the new fields don't accidentally pay the rejection-sampling
// cost.
func TestSeedPodWithOptions_AllZero_IsLegacyPath(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		mkDeck("A", "x"), mkDeck("B", "y"), mkDeck("C", "z"), mkDeck("D", "w"),
	}
	rng1 := rand.New(rand.NewSource(5))
	rng2 := rand.New(rand.NewSource(5))
	for i := 0; i < 20; i++ {
		viaOpts := SeedPodWithOptions(decks, 3, rng1, SeedPodOptions{})
		// Hand-rolled equivalent of the legacy uniform draw.
		perm := rng2.Perm(4)
		want := append([]int(nil), perm[:3]...)
		for j := range viaOpts {
			if viaOpts[j] != want[j] {
				t.Errorf("iter %d slot %d: opts=%d, legacy=%d", i, j, viaOpts[j], want[j])
			}
		}
	}
}

// TestSeedPodWithOptions_OppositionIgnoredWithoutArchetypes pins the
// silent-no-op contract: PreferOpposition=true with an empty
// Archetypes slice must NOT cause the sampler to reject every pod
// (no archetype data = no opposition signal). Equivalent to the
// legacy path.
func TestSeedPodWithOptions_OppositionIgnoredWithoutArchetypes(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		mkDeck("A", "x"), mkDeck("B", "y"), mkDeck("C", "z"),
	}
	opts := SeedPodOptions{PreferOpposition: true, Archetypes: nil}
	rng := rand.New(rand.NewSource(1))
	pod := SeedPodWithOptions(decks, 3, rng, opts)
	if len(pod) != 3 {
		t.Errorf("len = %d, want 3 (opposition with no archetypes should no-op)", len(pod))
	}
}
