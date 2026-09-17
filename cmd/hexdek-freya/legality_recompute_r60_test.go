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
func TestRecomputeLegality_ClearsPoisonedSetCodeCommander(t *testing.T) {
	oracle := newTestOracle(&oracleEntry{
		Name:          "Gimbal, Gremlin Prodigy",
		TypeLine:      "Legendary Creature — Goblin Artificer",
		OracleText:    "Whenever you roll one or more dice, put that many +1/+1 counters on Gimbal.",
		ColorIdentity: []string{"R"},
	})

	// 99 Mountains + 1 commander = 100. Mountains are unresolved here
	// (not in the mini-oracle) but singleton-exempt as basics and
	// skipped by the color-identity check, so the ONLY thing that swings
	// overall legality is whether the commander resolves.
	cardQtys := map[string]int{"Mountain": 99}
	const totalCards = 100

	const malformed = "Gimbal, Gremlin Prodigy (MOC) 3"
	const corrected = "Gimbal, Gremlin Prodigy"

	// (1) Both commander forms MUST land on the same cache entry — this
	// is precisely why the stale verdict can't be escaped by re-import.
	keyBad := DeckCacheKey(malformed, cardQtys)
	keyGood := DeckCacheKey(corrected, cardQtys)
	if keyBad != keyGood {
		t.Fatalf("precondition failed: set-code suffix must normalize to the same cache key\n  malformed=%s\n  corrected=%s", keyBad, keyGood)
	}

	// (2) The poisoned verdict is genuinely reproducible: the suffix form
	// fails commander resolution and the whole report is invalid.
	poisoned := RecomputeLegality(malformed, cardQtys, oracle, totalCards)
	if poisoned.Valid {
		t.Fatalf("precondition failed: malformed commander should be INVALID, got Valid=true")
	}
	if poisoned.CommanderOK.Valid {
		t.Fatalf("precondition failed: malformed commander should fail the commander check")
	}
	if !strings.Contains(strings.ToLower(poisoned.CommanderOK.Message), "not found") {
		t.Fatalf("expected 'not found' commander message, got %q", poisoned.CommanderOK.Message)
	}

	// (3) Recomputing against the CORRECTED text — same cache key — yields
	// a fresh VALID verdict. This is the fix: legality reflects the deck
	// as it is now, not as it was when first cached.
	fresh := RecomputeLegality(corrected, cardQtys, oracle, totalCards)
	if !fresh.CommanderOK.Valid {
		t.Fatalf("corrected commander should resolve as a legal commander, got message %q", fresh.CommanderOK.Message)
	}
	if !fresh.Valid {
		t.Fatalf("corrected deck should be legal overall, got Errors=%v", fresh.Errors)
	}
}

// TestRecomputeLegality_ServingPathOverwritesCachedVerdict mirrors the
// analyzeDeckFileCached cache-hit path: a cached FreyaReport carrying a
// stale INVALID legality has that field overwritten by a fresh recompute
// against the current (corrected) deck text before being served. The
// expensive analysis on the cached report is left untouched.
func TestRecomputeLegality_ServingPathOverwritesCachedVerdict(t *testing.T) {
	oracle := newTestOracle(&oracleEntry{
		Name:          "Gimbal, Gremlin Prodigy",
		TypeLine:      "Legendary Creature — Goblin Artificer",
		ColorIdentity: []string{"R"},
	})
	cardQtys := map[string]int{"Mountain": 99}

	// A cached report from the FIRST (malformed) import: the analysis is
	// present, but Legality is the poisoned invalid verdict.
	cached := &FreyaReport{
		DeckName:   "gimbal",
		Commander:  "Gimbal, Gremlin Prodigy (MOC) 3",
		TotalCards: 100,
		Legality:   RecomputeLegality("Gimbal, Gremlin Prodigy (MOC) 3", cardQtys, oracle, 100),
	}
	if cached.Legality.Valid {
		t.Fatalf("precondition: cached legality should be the poisoned invalid verdict")
	}

	// Serving path (mirrors cache.go analyzeDeckFileCached): recompute
	// legality against the CURRENT deck text and overwrite the cached
	// field. In production the current commander/qtys come from the fresh
	// parse pass; here we pass the corrected form directly.
	cached.Legality = RecomputeLegality("Gimbal, Gremlin Prodigy", cardQtys, oracle, cached.TotalCards)

	if !cached.Legality.Valid {
		t.Fatalf("served report should carry the fresh VALID legality, got Errors=%v", cached.Legality.Errors)
	}
	if !cached.Legality.CommanderOK.Valid {
		t.Fatalf("served report commander check should be valid after recompute")
	}
	// The rest of the (expensive) cached analysis is untouched.
	if cached.DeckName != "gimbal" {
		t.Fatalf("recompute must not disturb the cached analysis fields")
	}
}
