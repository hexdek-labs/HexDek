package main

import (
	"strings"
	"testing"
)

// newTestOracle builds a minimal in-memory oracleDB indexed the same way
// loadOracle indexes real data (normalizeName + folded-lowercase keys),
// so oracle.lookup resolves by the exact same rules the production path
// uses. Kept local to this test file so we don't depend on the 163MB
// oracle bulk file.
func newTestOracle(entries ...*oracleEntry) *oracleDB {
	db := &oracleDB{byName: map[string]*oracleEntry{}}
	for _, e := range entries {
		norm := normalizeName(e.Name)
		if _, ok := db.byName[norm]; !ok {
			db.byName[norm] = e
		}
		lower := foldQuotes(strings.ToLower(strings.TrimSpace(e.Name)))
		if _, ok := db.byName[lower]; !ok {
			db.byName[lower] = e
		}
	}
	return db
}

// TestRecomputeLegality_ClearsPoisonedSetCodeCommander is the load-
// bearing regression for the cache-poisoning bug.
//
// The cache key is content-addressed via normalizeForCacheKey =
// NormalizeName(CleanCardName(s)), and CleanCardName STRIPS the
// "(SET) N" printing suffix. But oracle.lookup normalizes via freya's
// normalizeName, which does NOT strip that suffix. So a commander line
// carrying a set-code suffix hashes to the SAME cache key as its clean
// form yet FAILS oracle resolution ("commander not found") — the exact
// divergence that let a poisoned "illegal" verdict, once cached, be
// served to a since-corrected deck forever.
//
// This test pins: (1) the two commander forms share one cache key,
// (2) the suffix form recomputes to an INVALID verdict (the poison is
// real), and (3) the clean form recomputes to a VALID verdict — i.e.
// recomputing legality on read breaks the poisoned entry.
func TestRecomputeLegality_SetCodeAndQuantityCommandersResolve(t *testing.T) {
	oracle := newTestOracle(&oracleEntry{
		Name:          "Gimbal, Gremlin Prodigy",
		TypeLine:      "Legendary Creature — Goblin Artificer",
		OracleText:    "Whenever you roll one or more dice, put that many +1/+1 counters on Gimbal.",
		ColorIdentity: []string{"R"},
	})
	cardQtys := map[string]int{"Mountain": 99}
	const totalCards = 100
	const clean = "Gimbal, Gremlin Prodigy"

	// r64 lookup-normalization fix: a commander that kept a leading deck
	// quantity or a "(SET) N" printing suffix from a plain-text paste / import
	// now RESOLVES instead of producing a false ILLEGAL ("not found in oracle
	// database"). All of these forms are the same card.
	for _, form := range []string{
		clean,                                 // clean
		"Gimbal, Gremlin Prodigy (MOC) 3",     // set-code suffix
		"1 Gimbal, Gremlin Prodigy",           // leading quantity
		"1 Gimbal, Gremlin Prodigy (MOC) 3",   // both
	} {
		r := RecomputeLegality(form, cardQtys, oracle, totalCards)
		if !r.CommanderOK.Valid {
			t.Errorf("commander %q should resolve, got message %q", form, r.CommanderOK.Message)
		}
		if !r.Valid {
			t.Errorf("deck with commander %q should be legal overall, got Errors=%v", form, r.Errors)
		}
	}

	// The cache-key collision that made the original poison inescapable still
	// holds — all forms fold to the same key (so a corrected re-import lands on
	// the same entry). The r64 fix means that entry is no longer poisoned.
	if DeckCacheKey("Gimbal, Gremlin Prodigy (MOC) 3", cardQtys) != DeckCacheKey(clean, cardQtys) {
		t.Fatal("set-code suffix must normalize to the same cache key as the clean name")
	}
}

// TestRecomputeLegality_ServingPathOverwritesCachedVerdict mirrors the
// analyzeDeckFileCached cache-hit path: a cached FreyaReport carrying a stale
// INVALID legality has that field overwritten by a fresh recompute against the
// current deck text before being served, leaving the expensive analysis intact.
// Uses a genuinely-unresolvable commander for the invalid state (the r64 fix
// now tolerates set-code / quantity artifacts, so those no longer poison).
func TestRecomputeLegality_ServingPathOverwritesCachedVerdict(t *testing.T) {
	oracle := newTestOracle(&oracleEntry{
		Name:          "Gimbal, Gremlin Prodigy",
		TypeLine:      "Legendary Creature — Goblin Artificer",
		ColorIdentity: []string{"R"},
	})
	cardQtys := map[string]int{"Mountain": 99}

	// Cached report whose legality is genuinely INVALID — a commander that does
	// not resolve even after normalization (a real miss, not a set-code/quantity
	// artifact).
	cached := &FreyaReport{
		DeckName:   "gimbal",
		Commander:  "Notacard, Phantom Commander",
		TotalCards: 100,
		Legality:   RecomputeLegality("Notacard, Phantom Commander", cardQtys, oracle, 100),
	}
	if cached.Legality.Valid {
		t.Fatalf("precondition: a truly-unresolvable commander should be INVALID")
	}

	// Serving path: recompute against the CURRENT (correct) deck text and
	// overwrite the cached field.
	cached.Legality = RecomputeLegality("Gimbal, Gremlin Prodigy", cardQtys, oracle, cached.TotalCards)
	if !cached.Legality.Valid {
		t.Fatalf("served report should carry the fresh VALID legality, got Errors=%v", cached.Legality.Errors)
	}
	if !cached.Legality.CommanderOK.Valid {
		t.Fatalf("served report commander check should be valid after recompute")
	}
	if cached.DeckName != "gimbal" {
		t.Fatalf("recompute must not disturb the cached analysis fields")
	}
}
