package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDeckCacheKey_StableOrdering pins the content-addressing
// contract: identical card contents (regardless of map iteration
// order, commander whitespace, or printing decoration on names)
// produce the same cache key.
func TestDeckCacheKey_StableOrdering(t *testing.T) {
	qtys1 := map[string]int{
		"Sol Ring":      1,
		"Demonic Tutor": 1,
		"Forest":        30,
	}
	qtys2 := map[string]int{
		"Forest":        30,
		"Sol Ring":      1,
		"Demonic Tutor": 1,
	}
	k1 := DeckCacheKey("Atraxa, Praetors' Voice", qtys1)
	k2 := DeckCacheKey("Atraxa, Praetors' Voice", qtys2)
	if k1 != k2 {
		t.Errorf("identical contents in different map order produced different keys:\n  k1=%s\n  k2=%s", k1, k2)
	}
	if len(k1) != 64 {
		t.Errorf("expected 64-char hex SHA256, got %d chars: %q", len(k1), k1)
	}
}

// TestDeckCacheKey_PrintingDecorationCollapsed verifies that printing
// decoration ("Forest (THB) 270", "[MID] Forest") normalizes to the
// same logical card, mirroring deckid's normalize-then-hash contract.
// Without this guarantee the same logical deck could cache twice under
// different keys depending on which import path produced the names.
func TestDeckCacheKey_PrintingDecorationCollapsed(t *testing.T) {
	clean := DeckCacheKey("Atraxa, Praetors' Voice", map[string]int{
		"Sol Ring": 1,
		"Forest":   30,
	})
	decorated := DeckCacheKey("Atraxa, Praetors' Voice", map[string]int{
		"Sol Ring (LEA) 270": 1,
		"[MID] Forest":       30,
	})
	if clean != decorated {
		t.Errorf("printing decoration produced a different key:\n  clean=%s\n  decorated=%s", clean, decorated)
	}
}

// TestDeckCacheKey_QuantityMatters pins the inverse: changing a basic-
// land count IS a meaningful deck change and must produce a different
// key. 30 Plains vs 35 Plains is a real mana-base tuning decision and
// the analysis output differs (mana curve, land ratio).
func TestDeckCacheKey_QuantityMatters(t *testing.T) {
	thirty := DeckCacheKey("Cmdr", map[string]int{"Forest": 30})
	thirtyFive := DeckCacheKey("Cmdr", map[string]int{"Forest": 35})
	if thirty == thirtyFive {
		t.Errorf("different basic-land counts produced same key — quantity must participate")
	}
}

// TestDeckCacheKey_CommanderMatters pins that changing the commander
// changes the key — same 99 cards but different commander = different
// deck (deck-construction colors, gameplan).
func TestDeckCacheKey_CommanderMatters(t *testing.T) {
	qtys := map[string]int{"Sol Ring": 1, "Forest": 30}
	a := DeckCacheKey("Atraxa, Praetors' Voice", qtys)
	b := DeckCacheKey("Ezuri, Claw of Progress", qtys)
	if a == b {
		t.Errorf("different commanders with same 99 produced same key")
	}
}

// TestDeckCacheKey_EmptyInputs is the defensive-empty case: nil/empty
// inputs should still produce a deterministic hex string, not panic.
func TestDeckCacheKey_EmptyInputs(t *testing.T) {
	empty := DeckCacheKey("", nil)
	if len(empty) != 64 {
		t.Errorf("empty inputs should still produce 64-char SHA256, got %d", len(empty))
	}
	// Two empty calls must agree.
	if empty != DeckCacheKey("", nil) {
		t.Errorf("empty-input keys disagree across calls")
	}
}

// TestCacheRoundTrip_HitAfterSave is the happy-path test: save a
// report, look it up by the same key, get the same report back.
func TestCacheRoundTrip_HitAfterSave(t *testing.T) {
	dir := t.TempDir()
	key := DeckCacheKey("test cmdr", map[string]int{"Sol Ring": 1, "Forest": 30})

	original := makeFixtureReport()
	if err := SaveToCache(dir, key, original); err != nil {
		t.Fatalf("SaveToCache: %v", err)
	}

	loaded, hit := TryLoadFromCache(dir, key)
	if !hit {
		t.Fatal("expected cache hit after save, got miss")
	}
	if loaded == nil {
		t.Fatal("hit reported but report is nil")
	}
	if loaded.DeckName != original.DeckName {
		t.Errorf("DeckName round-trip: got %q, want %q", loaded.DeckName, original.DeckName)
	}
	if loaded.TotalCards != original.TotalCards {
		t.Errorf("TotalCards round-trip: got %d, want %d", loaded.TotalCards, original.TotalCards)
	}
}

// TestCacheMiss_NoFile pins the missing-file branch: lookup on a key
// that has no cache file returns (nil, false), not an error.
func TestCacheMiss_NoFile(t *testing.T) {
	dir := t.TempDir()
	report, hit := TryLoadFromCache(dir, "nonexistent-key-1234")
	if hit {
		t.Error("expected miss on nonexistent key, got hit")
	}
	if report != nil {
		t.Error("expected nil report on miss")
	}
}

// TestCacheStale_VersionMismatch pins the version-invalidation branch.
// We hand-craft a cache file with a stale FreyaVersion (using the
// current key's filename pattern so the file IS found by path) and
// verify TryLoadFromCache rejects it.
func TestCacheStale_VersionMismatch(t *testing.T) {
	dir := t.TempDir()
	key := "stalekey0000000000000000000000000000000000000000000000000000000000"
	path := CacheFilePath(dir, key)

	// Hand-write a cache entry with a stale FreyaVersion.
	stale := cacheEntry{
		FreyaVersion: "r0-ancient",
		DeckHash:     key,
		Report:       makeFixtureReport(),
	}
	data, err := json.Marshal(stale)
	if err != nil {
		t.Fatalf("marshal stale entry: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	_, hit := TryLoadFromCache(dir, key)
	if hit {
		t.Error("expected miss on version-mismatched cache entry, got hit")
	}
}

// TestCacheStale_CorruptFile pins defensive handling of garbage in
// the cache directory — perhaps from a crashed write or manual
// tampering. A corrupt file must NOT crash the loader; it returns
// (nil, false) and analysis falls through to a fresh run.
func TestCacheStale_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	key := "corruptkey00000000000000000000000000000000000000000000000000000000"
	path := CacheFilePath(dir, key)
	if err := os.WriteFile(path, []byte("not even close to valid JSON {{{"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	_, hit := TryLoadFromCache(dir, key)
	if hit {
		t.Error("expected miss on corrupt cache file, got hit")
	}
}

// TestCacheStale_NilReportInside catches the case where the file is
// valid JSON, the FreyaVersion matches, but the embedded Report field
// is nil. We treat that as a miss rather than returning a nil pointer
// the caller might dereference.
func TestCacheStale_NilReportInside(t *testing.T) {
	dir := t.TempDir()
	key := "nilreportkey0000000000000000000000000000000000000000000000000000"
	path := CacheFilePath(dir, key)
	entry := cacheEntry{
		FreyaVersion: FreyaVersion,
		DeckHash:     key,
		Report:       nil,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal nil-report entry: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write nil-report file: %v", err)
	}
	report, hit := TryLoadFromCache(dir, key)
	if hit {
		t.Error("expected miss on entry with nil Report, got hit")
	}
	if report != nil {
		t.Error("expected nil report on miss")
	}
}

// TestCachePath_VersionSuffix pins the filename pattern. Bumping
// FreyaVersion should make all current-version filenames stale; the
// path layout is what makes that automatic (different version =
// different file).
func TestCachePath_VersionSuffix(t *testing.T) {
	path := CacheFilePath("/tmp/cache", "abc123")
	want := filepath.Join("/tmp/cache", "abc123-v"+FreyaVersion+".json")
	if path != want {
		t.Errorf("CacheFilePath = %q, want %q", path, want)
	}
	if !strings.HasSuffix(path, ".json") {
		t.Errorf("expected .json suffix, got %q", path)
	}
}

// TestSaveToCache_CreatesDir pins that SaveToCache creates the cache
// directory tree if it doesn't exist. A fresh checkout has no
// data/freya-cache/ — the first run must not fail because of that.
func TestSaveToCache_CreatesDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "deeply", "nested", "cache")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("test precondition failed: cache dir already exists")
	}
	key := "dirtest000000000000000000000000000000000000000000000000000000000"
	if err := SaveToCache(dir, key, makeFixtureReport()); err != nil {
		t.Fatalf("SaveToCache: %v", err)
	}
	if _, err := os.Stat(CacheFilePath(dir, key)); err != nil {
		t.Errorf("expected cache file to exist after save: %v", err)
	}
}

// TestCacheVersionConstant_NonEmpty is a sanity test: an empty
// FreyaVersion string would make EVERY cache file look unversioned
// (filename pattern degenerates to "key-v.json") and would silently
// disable invalidation. Pin that the constant is set to something.
func TestCacheVersionConstant_NonEmpty(t *testing.T) {
	if strings.TrimSpace(FreyaVersion) == "" {
		t.Error("FreyaVersion must be non-empty for cache invalidation to work")
	}
	if !strings.HasPrefix(FreyaVersion, "r") {
		t.Errorf("FreyaVersion = %q, expected r-prefixed token (e.g. r60.1) by convention", FreyaVersion)
	}
}

// makeFixtureReport produces a minimal FreyaReport suitable for the
// cache round-trip tests. Keeps fields small so the JSON encode
// doesn't pull in classifier code that requires oracle data.
func makeFixtureReport() *FreyaReport {
	return &FreyaReport{
		DeckName:   "fixture",
		DeckPath:   "/tmp/fixture.txt",
		Commander:  "Test Commander",
		TotalCards: 100,
		TrueInfinites: []ComboResult{
			{Cards: []string{"A", "B"}, Description: "test combo"},
		},
	}
}
