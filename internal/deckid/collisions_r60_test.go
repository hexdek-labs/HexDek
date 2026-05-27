package deckid

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// collisions_r60_test.go — Hash-collision verification at scale.
//
// ComputeHash reduces (commanders, library) to SHA-256 of a normalized
// sorted "1x name\n..." payload. The cryptographic part is essentially
// collision-free at 10K samples (P ≈ 5e-71 for the SHA-256 birthday
// bound), so a collision here would almost certainly come from the
// normalization/sort/join layer collapsing two distinct deck contents to
// the same payload string. This test generates 10000 deterministically
// random but structurally realistic decklists (1 commander + 99 picks
// from a shared staple pool + variable basics) and asserts zero hash
// collisions.

// stapleCards is the synthetic card pool for collision testing. Names
// don't need to map to real cards — what matters is that the pool is
// large enough that random 99-card libraries are unique content-wise.
// Pool size 1500 → C(1500, 99) is astronomical, so 10K random samples
// will all be content-distinct.
func stapleCards(n int) []string {
	pool := make([]string, n)
	for i := 0; i < n; i++ {
		pool[i] = fmt.Sprintf("Staple Card %04d of the Forge", i)
	}
	return pool
}

func commanderNames(n int) []string {
	pool := make([]string, n)
	for i := 0; i < n; i++ {
		pool[i] = fmt.Sprintf("Commander Persona %03d, Voice of Test", i)
	}
	return pool
}

func basicLandNames() []string {
	return []string{"Plains", "Island", "Swamp", "Mountain", "Forest", "Wastes"}
}

// genDeck builds one deterministic-random decklist (1 commander, 99
// library cards) using the supplied rng. The library mixes ~80 random
// staples with ~19-25 basics in a varying color split.
func genDeck(rng *rand.Rand, commanders, staples, basics []string) ([]*gameengine.Card, []*gameengine.Card) {
	cmdName := commanders[rng.Intn(len(commanders))]
	cmd := []*gameengine.Card{{Name: cmdName}}

	library := make([]*gameengine.Card, 0, 99)
	// Sample ~75-82 distinct staples without replacement.
	nStaples := 75 + rng.Intn(8)
	perm := rng.Perm(len(staples))[:nStaples]
	for _, idx := range perm {
		library = append(library, &gameengine.Card{Name: staples[idx]})
	}
	// Fill remainder with basics; pick a random color split each deck
	// (some monocolor, some 3-color etc.) so basic-land mixes vary.
	remaining := 99 - len(library)
	for remaining > 0 {
		library = append(library, &gameengine.Card{Name: basics[rng.Intn(len(basics))]})
		remaining--
	}
	return cmd, library
}

func TestDeckHash_NoCollisionsAt10K(t *testing.T) {
	const trials = 10000
	rng := rand.New(rand.NewSource(0xDEC4)) // deterministic
	staples := stapleCards(1500)
	commanders := commanderNames(800)
	basics := basicLandNames()

	hashes := make(map[DeckHash]int, trials)
	collisions := 0
	for i := 0; i < trials; i++ {
		cmd, lib := genDeck(rng, commanders, staples, basics)
		h := ComputeHash(cmd, lib)
		if h == "" {
			t.Fatalf("trial %d: ComputeHash returned empty hash", i)
		}
		if prev, ok := hashes[h]; ok {
			collisions++
			if collisions <= 3 {
				t.Errorf("collision: trial %d shares hash %s with trial %d", i, string(h)[:16], prev)
			}
		} else {
			hashes[h] = i
		}
	}
	if collisions > 0 {
		t.Fatalf("found %d hash collisions across %d decks — normalization/sort/join layer is collapsing distinct content",
			collisions, trials)
	}
	t.Logf("collision-free: %d distinct decks hashed with 0 collisions (hash space utilized: %d unique values)",
		trials, len(hashes))
}

// TestDeckHash_IdenticalContentSameHash — idempotence baseline. Two
// independent builds of the same decklist must hash to the same value.
func TestDeckHash_IdenticalContentSameHash(t *testing.T) {
	build := func() ([]*gameengine.Card, []*gameengine.Card) {
		cmd := []*gameengine.Card{{Name: "Edgar Markov"}}
		lib := []*gameengine.Card{
			{Name: "Sol Ring"}, {Name: "Arcane Signet"}, {Name: "Command Tower"},
			{Name: "Plains"}, {Name: "Plains"}, {Name: "Swamp"}, {Name: "Mountain"},
		}
		return cmd, lib
	}
	a, b := build()
	c, d := build()
	if ComputeHash(a, b) != ComputeHash(c, d) {
		t.Fatalf("identical content produced different hashes")
	}
}

// TestDeckHash_OrderInvariant — shuffling card order must NOT change the
// hash (the sort step is supposed to canonicalize).
func TestDeckHash_OrderInvariant(t *testing.T) {
	cmd := []*gameengine.Card{{Name: "Atraxa, Praetors' Voice"}}
	libA := []*gameengine.Card{
		{Name: "Sol Ring"}, {Name: "Cyclonic Rift"}, {Name: "Doubling Season"},
		{Name: "Plains"}, {Name: "Island"}, {Name: "Swamp"}, {Name: "Forest"},
	}
	libB := []*gameengine.Card{
		{Name: "Forest"}, {Name: "Swamp"}, {Name: "Island"}, {Name: "Plains"},
		{Name: "Doubling Season"}, {Name: "Cyclonic Rift"}, {Name: "Sol Ring"},
	}
	if ComputeHash(cmd, libA) != ComputeHash(cmd, libB) {
		t.Fatalf("hash changed when library order changed — sort step is broken")
	}
}

// TestDeckHash_SingleSwapChangesHash — a one-card swap must produce a
// different hash (otherwise the hash can't distinguish actual deck
// variants).
func TestDeckHash_SingleSwapChangesHash(t *testing.T) {
	cmd := []*gameengine.Card{{Name: "Krenko, Mob Boss"}}
	libA := []*gameengine.Card{
		{Name: "Sol Ring"}, {Name: "Goblin Recruiter"}, {Name: "Mountain"}, {Name: "Mountain"},
	}
	libB := []*gameengine.Card{
		{Name: "Sol Ring"}, {Name: "Goblin Matron"}, {Name: "Mountain"}, {Name: "Mountain"},
	}
	if ComputeHash(cmd, libA) == ComputeHash(cmd, libB) {
		t.Fatalf("single-card swap produced same hash — hash can't distinguish deck variants")
	}
}

// TestDeckHash_NormalizationIdempotent — casing / whitespace / accents
// must NOT distinguish hashes (canonicalization happens via
// normalizeName).
func TestDeckHash_NormalizationIdempotent(t *testing.T) {
	cmd := []*gameengine.Card{{Name: "Jace, the Mind Sculptor"}}
	libA := []*gameengine.Card{{Name: "Sol Ring"}, {Name: "Mana Crypt"}}
	libB := []*gameengine.Card{{Name: "  SOL  RING  "}, {Name: "MANA crypt"}}
	if ComputeHash(cmd, libA) != ComputeHash(cmd, libB) {
		t.Fatalf("normalization gap: casing/whitespace produced different hashes")
	}
}

// TestDeckHash_MultiplicityCounted — having 30 Plains vs 35 Plains must
// produce different hashes (the basic-land mana base IS the variant
// surface for many decks).
func TestDeckHash_MultiplicityCounted(t *testing.T) {
	cmd := []*gameengine.Card{{Name: "Avacyn, Angel of Hope"}}
	make30 := func() []*gameengine.Card {
		out := make([]*gameengine.Card, 30)
		for i := range out {
			out[i] = &gameengine.Card{Name: "Plains"}
		}
		return out
	}
	make35 := func() []*gameengine.Card {
		out := make([]*gameengine.Card, 35)
		for i := range out {
			out[i] = &gameengine.Card{Name: "Plains"}
		}
		return out
	}
	if ComputeHash(cmd, make30()) == ComputeHash(cmd, make35()) {
		t.Fatalf("30-Plains vs 35-Plains hashed identically — multiplicity is being collapsed")
	}
}

// TestDeckHash_ShuffledVariantsAllIdentical — 1000 random shuffles of the
// same canonical 100-card list (1 commander + 99 library) must produce
// exactly one hash. Extends TestDeckHash_OrderInvariant from 2 orderings
// to 1000 to stress the sort step under realistic deck size.
func TestDeckHash_ShuffledVariantsAllIdentical(t *testing.T) {
	const trials = 1000
	cmd := []*gameengine.Card{{Name: "Atraxa, Praetors' Voice"}}

	canonical := make([]*gameengine.Card, 0, 99)
	for i := 0; i < 65; i++ {
		canonical = append(canonical, &gameengine.Card{Name: fmt.Sprintf("Canonical Card %03d", i)})
	}
	basics := basicLandNames()
	for i := 0; i < 34; i++ {
		canonical = append(canonical, &gameengine.Card{Name: basics[i%len(basics)]})
	}

	baseline := ComputeHash(cmd, canonical)
	rng := rand.New(rand.NewSource(0x5FF1E))
	seen := map[DeckHash]struct{}{baseline: {}}
	for i := 0; i < trials; i++ {
		shuffled := make([]*gameengine.Card, len(canonical))
		copy(shuffled, canonical)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		h := ComputeHash(cmd, shuffled)
		if h != baseline {
			t.Fatalf("trial %d: shuffle changed hash (%s != %s) — sort step not canonicalizing under realistic deck size",
				i, string(h)[:16], string(baseline)[:16])
		}
		seen[h] = struct{}{}
	}
	if len(seen) != 1 {
		t.Fatalf("expected exactly 1 unique hash across %d shuffles, got %d", trials, len(seen))
	}
	t.Logf("shuffle-invariant: %d random shuffles of canonical 100-card list all produced hash %s", trials, string(baseline)[:16])
}

// TestDeckHash_NearMissCollisionStressed — generate 10K decks that share
// 95+ of 100 cards (a single staple swap differentiates each one) and
// verify all hashes are distinct. This stresses the normalization layer
// in the regime where the inputs are MOST similar — exactly where any
// structural collision would manifest.
func TestDeckHash_NearMissCollisionStressed(t *testing.T) {
	const trials = 10000
	cmdName := "Atraxa, Praetors' Voice"
	cmd := []*gameengine.Card{{Name: cmdName}}

	// Build a fixed 99-card "spine": 70 staples + 29 basics. Each trial
	// swaps slot 0 for a uniquely-named staple to differentiate decks
	// by exactly one card.
	spine := make([]*gameengine.Card, 0, 99)
	for i := 0; i < 70; i++ {
		spine = append(spine, &gameengine.Card{Name: fmt.Sprintf("Spine Staple %03d", i)})
	}
	for i := 0; i < 29; i++ {
		spine = append(spine, &gameengine.Card{Name: "Forest"})
	}

	hashes := make(map[DeckHash]int, trials)
	for i := 0; i < trials; i++ {
		lib := make([]*gameengine.Card, len(spine))
		copy(lib, spine)
		lib = append(lib, &gameengine.Card{Name: fmt.Sprintf("Unique Variant %05d", i)})
		h := ComputeHash(cmd, lib)
		if prev, ok := hashes[h]; ok {
			t.Fatalf("near-miss collision at trial %d (also matches trial %d): single-card-swap variants hashed identically",
				i, prev)
		}
		hashes[h] = i
	}
	t.Logf("near-miss stress: %d single-card-swap variants, 0 collisions", trials)
}
