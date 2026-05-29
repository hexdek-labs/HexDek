package main

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// seed_deck_alloc_r60_test.go — Probe C of the Loki r60 25k-sweep forensic.
//
// The 25k-sweep surfaced 3324 CardIdentity invariant hits ("same *Card
// pointer appears in two zones simultaneously"). Before investing in
// engine-side root-cause work, we have to rule out the HARNESS as the
// source. The hypothesis under test: does the --seed-cards / --seed-cards-all-seats
// injection path (and the broader chaos deck-build loop) accidentally
// alias one *Card pointer across multiple seats' decks, making the
// "same pointer in two zones" violation a fixture artifact rather than
// an engine bug?
//
// The deck-build loop runs at runChaosGame lines 812-868:
//
//	for i, cd := range chaosDecks {
//	    ...
//	    lib := make([]*gameengine.Card, 0, len(cd.Cards))
//	    for _, name := range cd.Cards {
//	        c := buildCardFromName(name, corpus, meta)  // <-- allocator
//	        ...
//	        lib = append(lib, c)
//	    }
//	}
//
// buildCardFromName (main.go:1102) constructs `&gameengine.Card{...}`
// fresh on every call. The shared AST pointer (`c.AST = cardAST`) is
// correct — oracle data is read-only and is supposed to be shared.
// The Card struct itself is the unit that the CardIdentity invariant
// tracks, so each call MUST return a distinct heap address.
//
// CORRECT pattern: fresh allocation per call (distinct pointers).
// BUG pattern:     cached/memoized return (aliased pointers).
//
// Verdict:
//   * Test passes -> harness is clean; the 3324 hits are genuine engine
//     bugs requiring root-cause analysis.
//   * Test fails  -> harness has pointer-aliasing; sweep numbers are
//     fixture-contaminated and need re-running after the fix.

// minimalMeta builds a MetaDB containing "Blood Moon" and a handful of
// generic filler cards, enough to populate each seat's library beyond
// the seed slot. We don't need a real AST corpus: buildCardFromName
// (main.go:1110) short-circuits on `cardAST == nil && md == nil`, so a
// non-nil MetaDB entry alone is sufficient to drive the allocation
// path.
func minimalMeta(t *testing.T) *deckparser.MetaDB {
	t.Helper()
	rows := strings.Join([]string{
		`{"name":"Blood Moon","type_line":"Enchantment","mana_cost":"{2}{R}","cmc":3,"colors":["R"],"power":"","toughness":""}`,
		`{"name":"Mountain","type_line":"Basic Land — Mountain","mana_cost":"","cmc":0,"colors":[],"power":"","toughness":""}`,
		`{"name":"Plains","type_line":"Basic Land — Plains","mana_cost":"","cmc":0,"colors":[],"power":"","toughness":""}`,
		`{"name":"Island","type_line":"Basic Land — Island","mana_cost":"","cmc":0,"colors":[],"power":"","toughness":""}`,
		`{"name":"Swamp","type_line":"Basic Land — Swamp","mana_cost":"","cmc":0,"colors":[],"power":"","toughness":""}`,
	}, "\n")
	meta, err := deckparser.LoadMetaReader(strings.NewReader(rows))
	if err != nil {
		t.Fatalf("LoadMetaReader: %v", err)
	}
	if meta.Get("Blood Moon") == nil {
		t.Fatal("minimalMeta: Blood Moon missing from MetaDB")
	}
	return meta
}

// TestSeedDeckBuild_BloodMoonPointersAreUnique exercises the inner
// allocator (buildCardFromName) directly. Four calls with the same
// name should return four distinct *Card heap addresses. If any pair
// aliases, the seed-cards injection across multiple seats would put
// the SAME *Card into multiple libraries — and the CardIdentity
// invariant would flag every subsequent zone change of that card as
// a "duplication" that the engine never actually caused.
func TestSeedDeckBuild_BloodMoonPointersAreUnique(t *testing.T) {
	meta := minimalMeta(t)

	const nSeats = 4
	cards := make([]*gameengine.Card, nSeats)
	for i := 0; i < nSeats; i++ {
		c := buildCardFromName("Blood Moon", nil, meta)
		if c == nil {
			t.Fatalf("seat %d: buildCardFromName returned nil for Blood Moon", i)
		}
		c.Owner = i
		cards[i] = c
	}

	// Pairwise pointer-equality check. The CardIdentity invariant
	// compares by `==` on *Card, so this is the exact identity test
	// the engine's invariant scan runs at every state-based-action
	// pass.
	for i := 0; i < nSeats; i++ {
		for j := i + 1; j < nSeats; j++ {
			if cards[i] == cards[j] {
				t.Errorf("HARNESS POINTER ALIASING: seat %d and seat %d share *Card %p (%q) — "+
					"buildCardFromName is returning a memoized/cached pointer instead of allocating fresh. "+
					"The 25k-sweep CardIdentity hits are fixture artifacts, not engine bugs.",
					i, j, cards[i], cards[i].Name)
			}
		}
	}

	// Cross-check: the AST pointer is EXPECTED to alias (oracle data
	// is read-only and shared by design). If buildCardFromName ever
	// starts deep-cloning the AST, that's a different kind of bug
	// (memory waste), so pin the expected sharing here. With nil
	// corpus, AST is nil on every card — which still satisfies the
	// "all equal" check trivially. Pin under non-nil-corpus shape
	// only if someone wires astload.Corpus into the loki harness in
	// the future. For now, just assert the Owner stamp survived per
	// seat to confirm the Card struct is genuinely per-seat mutable.
	for i, c := range cards {
		if c.Owner != i {
			t.Errorf("seat %d: Owner stamp lost (got %d) — buildCardFromName may be sharing struct state",
				i, c.Owner)
		}
	}
}

// TestSeedDeckBuild_FullLibrariesAreDisjoint mirrors the runChaosGame
// per-seat library-build loop (main.go lines 812-868) end-to-end. For
// each of 4 seats, build a library containing Blood Moon + 5 filler
// cards (same name set per seat — the "stress" case where the seed-cards
// injection forces the same card into every seat's library, which is
// the exact path that --seed-cards-all-seats exercises). Then assert
// every *Card pointer across all 4 libraries is unique. Total of
// 4 * 6 = 24 pointers must be 24 distinct heap addresses.
func TestSeedDeckBuild_FullLibrariesAreDisjoint(t *testing.T) {
	meta := minimalMeta(t)
	cardNames := []string{"Blood Moon", "Mountain", "Plains", "Island", "Swamp", "Mountain"}

	const nSeats = 4
	libraries := make([][]*gameengine.Card, nSeats)
	for i := 0; i < nSeats; i++ {
		lib := make([]*gameengine.Card, 0, len(cardNames))
		for _, name := range cardNames {
			c := buildCardFromName(name, nil, meta)
			if c == nil {
				t.Fatalf("seat %d: nil card for %q", i, name)
			}
			c.Owner = i
			lib = append(lib, c)
		}
		libraries[i] = lib
	}

	// Flatten all pointers and check pairwise uniqueness via a map.
	seen := make(map[*gameengine.Card]struct {
		seat int
		idx  int
		name string
	})
	for i, lib := range libraries {
		for j, c := range lib {
			if prev, ok := seen[c]; ok {
				t.Errorf("HARNESS POINTER ALIASING: lib[%d][%d] (%q) shares *Card %p with lib[%d][%d] (%q) — "+
					"chaos deck-build loop is returning aliased pointers across seats. "+
					"All CardIdentity invariant hits must be re-evaluated post-fix.",
					i, j, c.Name, c, prev.seat, prev.idx, prev.name)
				continue
			}
			seen[c] = struct {
				seat int
				idx  int
				name string
			}{seat: i, idx: j, name: c.Name}
		}
	}

	wantTotal := nSeats * len(cardNames)
	if len(seen) != wantTotal {
		t.Errorf("expected %d distinct *Card pointers across %d seats, got %d",
			wantTotal, nSeats, len(seen))
	}
}
