// Package moxfield provides Moxfield URL import functionality.
//
// FetchDeck downloads a decklist from a Moxfield public URL and returns
// it as a plain-text string in the same format our deckparser expects:
//
//	COMMANDER: Commander Name
//	1 Card Name
//	1 Card Name
//	...
package moxfield

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ----- deck cache -----
//
// The CLI used to call FetchDeckName followed by FetchDeck for every
// import, hitting api2.moxfield.com twice for identical JSON. Worse,
// repeat imports of the same deck (re-runs, scripted batch imports,
// tooling that loops over a list with overlapping IDs) hammered the
// API anew each time. Two layers fix this:
//
//   1. inProcCache — sync.Map of deckID -> *apiResponse, lifetime of
//      the process. Zero-config; the FetchDeck/FetchDeckName/FetchDeckRaw
//      pair only does one HTTP round-trip per deck per process.
//   2. Disk cache (~/.cache/hexdek/moxfield/{id}.json, TTL-gated).
//      Survives across CLI invocations so back-to-back re-imports skip
//      the API. Tunable via env:
//        HEXDEK_MOXFIELD_CACHE=0           disable disk cache entirely
//        HEXDEK_MOXFIELD_CACHE_TTL=<sec>   override default 1h TTL
//        HEXDEK_MOXFIELD_CACHE_DIR=<dir>   override default cache dir
//
// Both layers are bypassable from the CLI via the --refresh flag, which
// calls Refresh(deckID) to invalidate the deck before fetching.

var (
	inProcCache sync.Map // map[string]*apiResponse

	// httpClientVar and apiBaseVar are package vars so tests can swap
	// the transport (httptest.Server) without touching production code.
	httpClientVar = &http.Client{Timeout: 30 * time.Second}
	apiBaseVar    = "https://api2.moxfield.com"
)

const (
	defaultCacheTTL = 1 * time.Hour
	cacheEnvDisable = "HEXDEK_MOXFIELD_CACHE"
	cacheEnvTTL     = "HEXDEK_MOXFIELD_CACHE_TTL"
	cacheEnvDir     = "HEXDEK_MOXFIELD_CACHE_DIR"
)

func cacheEnabled() bool {
	return os.Getenv(cacheEnvDisable) != "0"
}

func cacheTTL() time.Duration {
	if s := os.Getenv(cacheEnvTTL); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return defaultCacheTTL
}

func cacheDir() string {
	if d := os.Getenv(cacheEnvDir); d != "" {
		return d
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "hexdek", "moxfield")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "hexdek", "moxfield")
}

func cachePath(deckID string) string {
	dir := cacheDir()
	if dir == "" {
		return ""
	}
	// Defensive: deck IDs are alnum+_-, but sanitize anyway in case Moxfield
	// changes their ID format.
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, deckID)
	return filepath.Join(dir, safe+".json")
}

// readDiskCache returns the cached apiResponse for deckID if present and
// not expired. Returns nil on miss / expired / IO error (best-effort).
func readDiskCache(deckID string) *apiResponse {
	if !cacheEnabled() {
		return nil
	}
	path := cachePath(deckID)
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if time.Since(info.ModTime()) > cacheTTL() {
		return nil
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var data apiResponse
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil
	}
	return &data
}

// writeDiskCache persists the apiResponse for deckID. Best-effort: a
// failed write logs nothing and returns silently — the in-process
// cache and the fresh response still serve the current call.
func writeDiskCache(deckID string, body []byte) {
	if !cacheEnabled() {
		return
	}
	path := cachePath(deckID)
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	_ = os.WriteFile(path, body, 0644)
}

// SetAPIBaseForTest overrides the Moxfield API base URL and returns a
// cleanup that restores the original. Test-only escape hatch for
// out-of-package callers (cmd/hexdek-import end-to-end tests) — the
// in-package cache_r60_test.go pokes apiBaseVar directly. Always
// reset via the returned cleanup; otherwise other tests in the same
// process inherit the override.
func SetAPIBaseForTest(base string) func() {
	orig := apiBaseVar
	apiBaseVar = base
	return func() { apiBaseVar = orig }
}

// Refresh invalidates both cache layers for a deck ID so the next
// fetch hits the API. Used by the importer's --refresh flag.
func Refresh(deckID string) {
	inProcCache.Delete(deckID)
	if path := cachePath(deckID); path != "" {
		_ = os.Remove(path)
	}
}

// resetCachesForTest clears both cache layers and removes the entire
// cache dir. Test-only — package-private export via a _test.go shim
// in callers that need it.
func resetCachesForTest() {
	inProcCache.Range(func(k, _ any) bool { inProcCache.Delete(k); return true })
	if d := cacheDir(); d != "" {
		_ = os.RemoveAll(d)
	}
}

// moxfieldURLRE extracts the deck ID from a Moxfield URL.
// Supports:
//
//	https://www.moxfield.com/decks/abc123
//	https://moxfield.com/decks/abc123
//	www.moxfield.com/decks/abc123
//	moxfield.com/decks/abc123
var moxfieldURLRE = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?moxfield\.com/decks/([A-Za-z0-9_-]+)`)

// apiResponse mirrors the subset of the Moxfield v3 API response we need.
//
// The v3 API nests boards under a top-level "boards" object; older responses
// exposed mainboard/commanders directly at the top level. We accept both so
// older cached fixtures keep working.
type apiResponse struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Boards struct {
		Mainboard  apiBoard `json:"mainboard"`
		Commanders apiBoard `json:"commanders"`
		Sideboard  apiBoard `json:"sideboard"`
		Companions apiBoard `json:"companions"`
	} `json:"boards"`
	// Legacy top-level fields (pre-v3 boards wrapper).
	Mainboard  map[string]apiCardEntry `json:"mainboard"`
	Commanders map[string]apiCardEntry `json:"commanders"`
	Sideboard  map[string]apiCardEntry `json:"sideboard"`
	Companions map[string]apiCardEntry `json:"companions"`

	// User-applied tags. Moxfield exposes two parallel tagging surfaces:
	//   - Hubs:  community-curated canonical labels (Combo, Token, Stax,
	//            +1/+1 Counters, Tribal, Voltron, etc.). Each hub is a
	//            structured object with a name + slug.
	//   - Tags:  free-form per-user strings. Less normalized; some decks
	//            use these to mirror Hub names with custom casing or to
	//            add personal tags not in the Hub vocabulary.
	// Both surface as the same archetype-hint signal for downstream
	// consumers (Freya can use them to weight an archetype guess); we
	// merge + dedupe them via deckTags().
	Hubs []apiHub `json:"hubs"`
	Tags []string `json:"tags"`
}

// apiHub mirrors a Moxfield Hub entry. We only need the display name;
// Moxfield also returns slug + description + image fields we ignore.
type apiHub struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type apiBoard struct {
	Count int                     `json:"count"`
	Cards map[string]apiCardEntry `json:"cards"`
}

func (r *apiResponse) commanders() map[string]apiCardEntry {
	if len(r.Boards.Commanders.Cards) > 0 {
		return r.Boards.Commanders.Cards
	}
	return r.Commanders
}

func (r *apiResponse) mainboard() map[string]apiCardEntry {
	if len(r.Boards.Mainboard.Cards) > 0 {
		return r.Boards.Mainboard.Cards
	}
	return r.Mainboard
}

func (r *apiResponse) sideboard() map[string]apiCardEntry {
	if len(r.Boards.Sideboard.Cards) > 0 {
		return r.Boards.Sideboard.Cards
	}
	return r.Sideboard
}

func (r *apiResponse) companions() map[string]apiCardEntry {
	if len(r.Boards.Companions.Cards) > 0 {
		return r.Boards.Companions.Cards
	}
	return r.Companions
}

// deckTags returns the merged Hubs + Tags list, deduped
// case-insensitively and sorted alphabetically (case-sensitive sort so
// the output is deterministic regardless of which surface contributed
// a given tag). Empty / whitespace-only entries are dropped.
//
// Hubs win on case when both surfaces contribute the same tag: if the
// API returns Hub "Combo" + free-form Tag "combo", the output contains
// "Combo" only. Hubs are iterated first.
func (r *apiResponse) deckTags() []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" {
			return
		}
		key := strings.ToLower(t)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, t)
	}
	for _, h := range r.Hubs {
		add(h.Name)
	}
	for _, t := range r.Tags {
		add(t)
	}
	sort.Strings(out)
	return out
}

type apiCardEntry struct {
	Quantity int     `json:"quantity"`
	Card     apiCard `json:"card"`
}

type apiCard struct {
	Name string `json:"name"`
}

// ExtractDeckID extracts the deck ID from a Moxfield URL.
// Returns empty string if the URL doesn't match.
func ExtractDeckID(url string) string {
	m := moxfieldURLRE.FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return m[1]
}

// FetchDeck downloads a decklist from a Moxfield URL.
// URL format: https://www.moxfield.com/decks/{id}
// Uses the Moxfield public API: https://api2.moxfield.com/v3/decks/all/{id}
// Returns the decklist as a string in the same format our parser expects:
//
//	COMMANDER: Commander Name
//	1 Card Name
//	1 Card Name
//	...
func FetchDeck(url string) (string, error) {
	deckID := ExtractDeckID(url)
	if deckID == "" {
		return "", fmt.Errorf("moxfield: could not extract deck ID from URL %q", url)
	}

	return FetchDeckByID(deckID)
}

// FetchDeckByID downloads a decklist from the Moxfield API using a deck ID.
func FetchDeckByID(deckID string) (string, error) {
	data, err := fetchDeckRaw(deckID)
	if err != nil {
		return "", err
	}
	return formatDecklist(data)
}

// FetchDeckName downloads just the deck name from a Moxfield URL.
// Shares the full-deck cache so the common importer pattern
// (FetchDeckName + FetchDeck back-to-back) only makes one HTTP call.
func FetchDeckName(url string) (string, error) {
	deckID := ExtractDeckID(url)
	if deckID == "" {
		return "", fmt.Errorf("moxfield: could not extract deck ID from URL %q", url)
	}
	data, err := fetchDeckRaw(deckID)
	if err != nil {
		return "", err
	}
	return data.Name, nil
}

// DeckMeta exposes deck-level Moxfield metadata for callers that want
// more than the bare decklist text — currently the deck's display
// name and its user-applied tags (Hubs ∪ free-form tags, deduped and
// sorted). Returned by FetchDeckMeta.
type DeckMeta struct {
	Name string
	Tags []string
}

// FetchDeckMeta returns the deck's name and user-applied tags. Shares
// the in-process + disk cache with FetchDeck/FetchDeckName so calling
// FetchDeckMeta in addition to FetchDeck doesn't add a second HTTP hit.
func FetchDeckMeta(url string) (*DeckMeta, error) {
	deckID := ExtractDeckID(url)
	if deckID == "" {
		return nil, fmt.Errorf("moxfield: could not extract deck ID from URL %q", url)
	}
	data, err := fetchDeckRaw(deckID)
	if err != nil {
		return nil, err
	}
	return &DeckMeta{Name: data.Name, Tags: data.deckTags()}, nil
}

// fetchDeckRaw is the single source of truth for "give me the parsed
// Moxfield API response for this deck ID." Consults the in-process
// cache, then the disk cache, then the API in that order. Cache
// writes happen on both layers after a successful fetch.
func fetchDeckRaw(deckID string) (*apiResponse, error) {
	if deckID == "" {
		return nil, fmt.Errorf("moxfield: empty deck ID")
	}

	if v, ok := inProcCache.Load(deckID); ok {
		return v.(*apiResponse), nil
	}
	if data := readDiskCache(deckID); data != nil {
		inProcCache.Store(deckID, data)
		return data, nil
	}

	apiURL := fmt.Sprintf("%s/v3/decks/all/%s", apiBaseVar, deckID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("moxfield: build request: %w", err)
	}
	req.Header.Set("User-Agent", "hexdek/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClientVar.Do(req)
	if err != nil {
		return nil, fmt.Errorf("moxfield: fetch %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("moxfield: API returned %d for deck %s: %s", resp.StatusCode, deckID, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("moxfield: read response: %w", err)
	}

	var data apiResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("moxfield: parse JSON: %w", err)
	}

	inProcCache.Store(deckID, &data)
	writeDiskCache(deckID, bodyBytes)
	return &data, nil
}

// formatDecklist converts a Moxfield API response into our text decklist format.
//
// Each board is iterated in card-name order so the output is deterministic.
// Partner decks emit two COMMANDER: lines; without sorting, Go's randomized
// map iteration would flip which partner appears first across fetches, and
// the downstream deckparser treats the first commander as primary.
func formatDecklist(data *apiResponse) (string, error) {
	// Build card content into its own builder first so the empty-deck
	// check below stays a pure "are there cards?" question — without
	// this, a deck with tags but no cards would slip past the check.
	var cardsBuf strings.Builder
	sb := &cardsBuf

	writeEntries := func(entries map[string]apiCardEntry, format func(apiCardEntry) string) {
		keys := make([]string, 0, len(entries))
		for k := range entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			line := format(entries[k])
			if line != "" {
				sb.WriteString(line)
			}
		}
	}

	writeEntries(data.commanders(), func(e apiCardEntry) string {
		if e.Card.Name == "" {
			return ""
		}
		return fmt.Sprintf("COMMANDER: %s\n", e.Card.Name)
	})
	writeEntries(data.mainboard(), func(e apiCardEntry) string {
		if e.Card.Name == "" || e.Quantity <= 0 {
			return ""
		}
		return fmt.Sprintf("%d %s\n", e.Quantity, e.Card.Name)
	})
	writeEntries(data.sideboard(), func(e apiCardEntry) string {
		if e.Card.Name == "" || e.Quantity <= 0 {
			return ""
		}
		return fmt.Sprintf("// Sideboard: %d %s\n", e.Quantity, e.Card.Name)
	})
	writeEntries(data.companions(), func(e apiCardEntry) string {
		if e.Card.Name == "" || e.Quantity <= 0 {
			return ""
		}
		return fmt.Sprintf("// Companion: %d %s\n", e.Quantity, e.Card.Name)
	})

	cards := cardsBuf.String()
	if strings.TrimSpace(cards) == "" {
		return "", fmt.Errorf("moxfield: deck %q is empty (no mainboard or commanders)", data.Name)
	}

	// Prepend the `// Tags:` line (when present) so it lands above the
	// commander/mainboard lines. The downstream deckparser silently
	// skips unrecognized `//` / `#` comments, so positioning is free
	// — putting it at the top makes the tag set visible at a glance
	// when a human eyeballs the .txt file.
	var out strings.Builder
	if tags := data.deckTags(); len(tags) > 0 {
		out.WriteString("// Tags: " + strings.Join(tags, ", ") + "\n")
	}
	out.WriteString(cards)
	return out.String(), nil
}
