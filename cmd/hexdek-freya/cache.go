package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/deckparser"
)

// cache — content-addressed cache for FreyaReport.
//
// Freya analysis is expensive (oracle classification + role tagging +
// archetype inference + win-line walk + value-chain detection + combo
// detection + cluster scoring). A typical 100-card deck takes ~1-3
// seconds end-to-end, and large --all-decks corpora multiply that
// linearly. The cache lets a repeat run on an unchanged deck list
// short-circuit to a JSON-decode that takes ~5ms.
//
// Cache key: SHA256 over (commander + sorted "Nx normalized card name"
// lines). Same card contents → same key, regardless of file order,
// printing decoration, or casing. Quantities matter — a deck with 30
// Plains and one with 35 Plains are different cache entries.
//
// Cache invalidation: the FreyaVersion constant is baked into both the
// filename suffix and the cache entry body. Bumping FreyaVersion makes
// every existing cache file stale (the new filename pattern won't hit
// any old file, and the embedded version check is a defense-in-depth
// in case anyone manually copies a file across versions).

// FreyaVersion is the cache-invalidation token. Bump this string when
// a Freya change alters the FreyaReport schema OR any classifier
// output that would surface as drift in the consistency probe
// (`--mode metrics`).
const FreyaVersion = "r60.1"

// DefaultCacheDir is where Freya looks for cached reports. Gitignored
// in .gitignore so cache pollution doesn't end up in PRs.
const DefaultCacheDir = "data/freya-cache"

// DeckCacheKey computes a content-addressed cache key for a deck.
// Returns a hex SHA256 string. The shape mirrors deckid's normalize-
// then-sort approach but operates on Freya's parsed map[string]int
// quantity surface (rather than the gameengine.Card slices deckid
// expects).
//
// An empty commander + nil/empty qtys still produces a stable hash
// (the SHA256 of an empty string) — callers can rely on the function
// always returning a 64-char hex string.
func DeckCacheKey(commander string, qtys map[string]int) string {
	var lines []string
	if c := strings.TrimSpace(commander); c != "" {
		lines = append(lines, "COMMANDER:"+normalizeForCacheKey(c))
	}
	for name, qty := range qtys {
		if qty <= 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%dx %s", qty, normalizeForCacheKey(name)))
	}
	sort.Strings(lines)
	payload := strings.Join(lines, "\n")
	h := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("%x", h)
}

func normalizeForCacheKey(s string) string {
	return deckparser.NormalizeName(deckparser.CleanCardName(s))
}

// CacheFilePath returns the path where a cache entry with this key
// would live. The FreyaVersion suffix means version-stale entries
// don't collide with current-version entries on lookup.
func CacheFilePath(cacheDir, key string) string {
	return filepath.Join(cacheDir, key+"-v"+FreyaVersion+".json")
}

// cacheEntry is the on-disk format. The DeckHash and FreyaVersion
// fields are redundant with the filename but kept for diagnostic
// integrity — anyone inspecting a cache file by hand can see what it
// claims to be.
type cacheEntry struct {
	FreyaVersion string       `json:"freya_version"`
	DeckHash     string       `json:"deck_hash"`
	Report       *FreyaReport `json:"report"`
}

// TryLoadFromCache attempts to read a cached FreyaReport for the given
// key. Returns (nil, false) on any of:
//   - cache file doesn't exist
//   - file exists but its embedded FreyaVersion doesn't match the
//     current FreyaVersion (defense-in-depth against a manually copied
//     file)
//   - file is corrupt or unparseable
//   - decoded entry has a nil Report
//
// Returns (report, true) on a clean hit. Callers should treat any
// (nil, false) as "cache miss, run analysis fresh."
func TryLoadFromCache(cacheDir, key string) (*FreyaReport, bool) {
	path := CacheFilePath(cacheDir, key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}
	if entry.FreyaVersion != FreyaVersion {
		return nil, false
	}
	if entry.Report == nil {
		return nil, false
	}
	return entry.Report, true
}

// SaveToCache writes a FreyaReport under the given key. Creates the
// cache directory (with parents) if it doesn't exist. A cache write
// failure is non-fatal — callers should log and continue rather than
// abort the analysis run.
func SaveToCache(cacheDir, key string, report *FreyaReport) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	entry := cacheEntry{
		FreyaVersion: FreyaVersion,
		DeckHash:     key,
		Report:       report,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}
	path := CacheFilePath(cacheDir, key)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}
	return nil
}

// analyzeDeckFileCached wraps analyzeDeckFile with cache lookup +
// save. When useCache is false the function is a thin pass-through to
// analyzeDeckFile (no read, no write). When useCache is true:
//   - parse the deck just enough to compute the cache key
//   - on hit: return the cached report
//   - on miss: run analyzeDeckFile, save the result, return it
//
// Cache write failures are logged but not propagated — a failed cache
// write shouldn't break analysis. The cache key parse is independent
// of analyzeDeckFile's parse; on a miss the deck is parsed twice (cheap
// — parsing is ~ms while analysis is ~seconds).
func analyzeDeckFileCached(path string, oracle *oracleDB, mechDB *MechanicDB,
	cacheDir string, useCache bool) (*FreyaReport, error) {
	if !useCache {
		return analyzeDeckFile(path, oracle, mechDB)
	}

	// Compute the key from a lightweight parse pass.
	cardQtys, qtyErr := parseDeckListWithQuantities(path)
	_, commander, parseErr := parseDeckList(path)
	if qtyErr != nil || parseErr != nil {
		// Parse failure — skip cache and let analyzeDeckFile surface
		// the real parse error.
		return analyzeDeckFile(path, oracle, mechDB)
	}

	key := DeckCacheKey(commander, cardQtys)
	if cached, hit := TryLoadFromCache(cacheDir, key); hit {
		return cached, nil
	}

	report, err := analyzeDeckFile(path, oracle, mechDB)
	if err != nil {
		return nil, err
	}
	if saveErr := SaveToCache(cacheDir, key, report); saveErr != nil {
		// Non-fatal: log and continue. The report we computed is
		// still valid — only the future-speedup is lost.
		fmt.Fprintf(os.Stderr, "freya cache save failed for %s: %v\n",
			filepath.Base(path), saveErr)
	}
	return report, nil
}
