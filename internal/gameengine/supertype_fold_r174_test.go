package gameengine

import (
	"math/rand"
	"strings"
	"testing"
)

// r174 — HasSupertypeOf used to lowercase EVERY element of
// Characteristics.Supertypes on every call. That fold was redundant:
// Supertypes is lowercase by construction. These tests pin both the
// behaviour and the invariant the removal rests on.
//
// NOTE ON SCOPE, because the round started larger: a fast path for the
// `want` normalisation in HasTypeOf was built, tested and then DROPPED.
// An interleaved benchmark measured it at 0.94x on the lowercase case it
// was built for -- i.e. slightly SLOWER. Go's strings.ToLower already
// has a good ASCII scan and returns the input unchanged without
// allocating, so a hand-rolled pre-scan adds a call and saves nothing.
// It is not in the tree. See the r174 notes.

func TestR174_HasSupertypeOf_BothDirections(t *testing.T) {
	gs := newFixtureGame(t)
	// Build through the real path: partitionTypes splits supertypes out
	// of Types, so hand-populating Supertypes would test nothing.
	lord := addBattlefield(gs, 0, "Legendary Lord", 3, 3, "legendary", "creature")
	plain := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	snowy := addBattlefield(gs, 0, "Snow Land", 0, 0, "snow", "land")

	for _, q := range []string{"legendary", "Legendary", " LEGENDARY ", "\tlegendary\n"} {
		if !gs.HasSupertypeOf(lord, q) {
			t.Errorf("HasSupertypeOf(legendary creature, %q) = false; the QUERY "+
				"is still normalised even though the stored element is not", q)
		}
	}
	if gs.HasSupertypeOf(plain, "legendary") {
		t.Error("CR 704.5j — a nonlegendary creature must not report the legendary supertype")
	}
	if gs.HasSupertypeOf(lord, "basic") {
		t.Error("a legendary creature must not report the basic supertype")
	}
	if !gs.HasSupertypeOf(snowy, "snow") {
		t.Error("a snow land must report the snow supertype")
	}
	if gs.HasSupertypeOf(snowy, "legendary") {
		t.Error("a snow land must not report the legendary supertype")
	}
}

// The removal of the per-element fold rests entirely on partitionTypes
// lowercasing every token it files. Pin that invariant directly: if a
// future change lets a token through raw, this reds and names why.
func TestR174_PartitionTypesLowercasesEveryTokenItFiles(t *testing.T) {
	types, supers := partitionTypes([]string{
		"Legendary", "CREATURE", "Artifact", "SNOW", "bAsIc", "Goblin", "WoRlD",
	})
	for _, got := range append(append([]string{}, types...), supers...) {
		if got != strings.ToLower(got) {
			t.Errorf("partitionTypes filed %q un-lowercased — HasSupertypeOf's "+
				"removal of its per-element fold depends on this invariant, so "+
				"restore the fold or fix the writer", got)
		}
	}
	if len(supers) != 4 {
		t.Errorf("expected legendary/snow/basic/world as supertypes, got %v", supers)
	}
	if len(types) != 3 {
		t.Errorf("expected creature/artifact/goblin as types, got %v", types)
	}
}

func BenchmarkR174_HasSupertypeOf(b *testing.B) {
	gs := NewGameState(4, rand.New(rand.NewSource(174)), nil)
	p := addBattlefield(gs, 0, "Legendary Lord", 3, 3, "legendary", "creature")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.HasSupertypeOf(p, "legendary")
	}
}
