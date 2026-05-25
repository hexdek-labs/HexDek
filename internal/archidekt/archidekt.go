// Package archidekt provides Archidekt URL import functionality.
//
// FetchDeck downloads a decklist from an Archidekt public URL and
// returns it as a plain-text string in the same format our deckparser
// expects (and that internal/moxfield emits):
//
//	COMMANDER: Commander Name
//	1 Card Name
//	1 Card Name
//	...
//	// Sideboard: 1 Card
//	// Maybeboard: 1 Card
//
// Archidekt's data model differs from Moxfield's in two ways the
// importer has to handle:
//
//  1. There's no separate "commanders" board — commanders are normal
//     mainboard rows tagged with a category named "Commander" (or
//     "Commander/Companion" / "Partner", depending on the deck).
//     We detect them by matching the configured category names against
//     the per-card category list.
//
//  2. There's no fixed "maybeboard" board either. Archidekt lets users
//     define arbitrary categories; the platform marks each category
//     with `includedInDeck` (counts toward the legal 100) and
//     `includedInPrice`. A card is "in the deck" only if at least one
//     of its categories has `includedInDeck: true`. Cards whose only
//     categories all flag `includedInDeck: false` (Maybeboard,
//     Sideboard, Considering, etc.) get routed to the comment-prefixed
//     output sections the deckparser already silently drops from the
//     legal deck.
package archidekt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// httpClientVar and apiBaseVar are package vars so tests can swap
	// the transport (httptest.Server) without touching production code.
	// Same pattern as internal/moxfield.
	httpClientVar = &http.Client{Timeout: 30 * time.Second}
	apiBaseVar    = "https://archidekt.com"

	// inProcCache mirrors internal/moxfield's lifetime-of-process cache.
	// Disk cache is intentionally deferred to a follow-up PR — the
	// in-process cache covers the common "FetchDeckName + FetchDeck
	// back-to-back" pattern that drives most repeat hits.
	inProcCache sync.Map // map[string]*apiResponse
)

// SetAPIBaseForTest overrides the Archidekt API base URL and returns a
// cleanup that restores the original. Always invoke the returned
// cleanup; otherwise other tests in the same process inherit the
// override.
func SetAPIBaseForTest(base string) func() {
	orig := apiBaseVar
	apiBaseVar = base
	return func() { apiBaseVar = orig }
}

// resetCachesForTest clears the in-process cache. Used by tests that
// want fresh fetches between scenarios.
func resetCachesForTest() {
	inProcCache.Range(func(k, _ any) bool { inProcCache.Delete(k); return true })
}

// archidektURLRE extracts the deck ID from an Archidekt URL.
// Supports:
//
//	https://www.archidekt.com/decks/123456
//	https://archidekt.com/decks/123456
//	https://archidekt.com/decks/123456/some-slug
//	www.archidekt.com/decks/123456
//	archidekt.com/decks/123456
//
// Deck IDs are numeric on archidekt.com (unlike moxfield's alnum hash).
// The regex deliberately accepts only digits so a URL like
// `/decks/foo` is treated as malformed rather than silently using
// "foo" as an ID.
var archidektURLRE = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?archidekt\.com/decks/(\d+)`)

// apiResponse mirrors the subset of the Archidekt API response we need.
// Archidekt's `/api/decks/{id}/` endpoint returns the full deck blob
// including unrelated fields (owner profile, format, edhrec_score,
// etc.); we ignore everything outside this struct.
type apiResponse struct {
	Name       string        `json:"name"`
	Categories []apiCategory `json:"categories"`
	Cards      []apiCard     `json:"cards"`
}

// apiCategory describes one of the deck's user-or-platform categories.
// `IncludedInDeck` is the gate that decides whether a card living in
// this category counts toward the legal deck (true) or routes to the
// commented-out output (false: Maybeboard, Sideboard, Considering).
type apiCategory struct {
	Name           string `json:"name"`
	IncludedInDeck bool   `json:"includedInDeck"`
	IsPremier      bool   `json:"isPremier"`
}

// apiCard is one entry in the deck's flat `cards` list. A single card
// can live in multiple categories; the `Categories` string slice
// reflects every category it's been placed in.
type apiCard struct {
	Quantity   int      `json:"quantity"`
	Categories []string `json:"categories"`
	Card       struct {
		OracleCard struct {
			Name string `json:"name"`
		} `json:"oracleCard"`
	} `json:"card"`
}

// ExtractDeckID returns the numeric deck ID from an Archidekt URL.
// Returns "" if the URL doesn't match the expected shape.
func ExtractDeckID(url string) string {
	m := archidektURLRE.FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return m[1]
}

// FetchDeck downloads a decklist from an Archidekt URL and returns
// it formatted as our text decklist (see formatDecklist).
func FetchDeck(url string) (string, error) {
	deckID := ExtractDeckID(url)
	if deckID == "" {
		return "", fmt.Errorf("archidekt: could not extract deck ID from URL %q", url)
	}
	return FetchDeckByID(deckID)
}

// FetchDeckByID downloads a decklist from the Archidekt API using a deck ID.
func FetchDeckByID(deckID string) (string, error) {
	data, err := fetchDeckRaw(deckID)
	if err != nil {
		return "", err
	}
	return formatDecklist(data)
}

// FetchDeckName downloads just the deck name from an Archidekt URL.
// Shares the in-process cache so calling FetchDeckName then FetchDeck
// only makes one HTTP round-trip.
func FetchDeckName(url string) (string, error) {
	deckID := ExtractDeckID(url)
	if deckID == "" {
		return "", fmt.Errorf("archidekt: could not extract deck ID from URL %q", url)
	}
	data, err := fetchDeckRaw(deckID)
	if err != nil {
		return "", err
	}
	return data.Name, nil
}

// fetchDeckRaw is the single source of truth for "give me the parsed
// Archidekt API response for this deck ID." Consults the in-process
// cache, then the API.
func fetchDeckRaw(deckID string) (*apiResponse, error) {
	if deckID == "" {
		return nil, fmt.Errorf("archidekt: empty deck ID")
	}
	if v, ok := inProcCache.Load(deckID); ok {
		return v.(*apiResponse), nil
	}

	apiURL := fmt.Sprintf("%s/api/decks/%s/", apiBaseVar, deckID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("archidekt: build request: %w", err)
	}
	req.Header.Set("User-Agent", "hexdek/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClientVar.Do(req)
	if err != nil {
		return nil, fmt.Errorf("archidekt: fetch %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("archidekt: API returned %d for deck %s: %s", resp.StatusCode, deckID, string(body))
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("archidekt: read response: %w", err)
	}
	var data apiResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("archidekt: parse JSON: %w", err)
	}
	inProcCache.Store(deckID, &data)
	return &data, nil
}

// commanderCategoryNames is the set of category names Archidekt uses
// (and that users commonly use as variants) to mark a commander row.
// All comparisons are case-insensitive on the API side, so we
// lowercase before matching. "Commander" is the canonical category
// name Archidekt creates by default on a Commander-format deck.
//
//	Commander / Partner: standard partner pair gets two rows in the
//	  Commander category, each card may additionally carry "Partner"
//	  in its categories list — both still flag as commander here.
//	Companion: Archidekt currently treats Companions as a regular
//	  category named "Companion"; the importer routes them through
//	  the // Companion: comment line instead of marking them commander.
var commanderCategoryNames = map[string]bool{
	"commander": true,
	"partner":   true,
	"helper":    true, // some users use "Helper" for the second partner
}

// inDeckCategoryIndex builds a lookup that says, for each declared
// category, whether cards living in that category count toward the
// legal 100. Categories not declared in the response default to true
// (Archidekt's runtime behavior: any user-typed category that isn't
// pre-registered acts as a regular in-deck grouping).
func inDeckCategoryIndex(cats []apiCategory) map[string]bool {
	out := make(map[string]bool, len(cats))
	for _, c := range cats {
		out[strings.ToLower(c.Name)] = c.IncludedInDeck
	}
	return out
}

// classifyCard returns one of:
//
//	"commander"  — has a category in commanderCategoryNames AND
//	               that category is includedInDeck
//	"mainboard"  — at least one category is includedInDeck, but
//	               none of them are commander
//	"sideboard"  — declared as Sideboard / Companion / Maybeboard /
//	               Considering / etc. (everything excluded from
//	               the legal deck). The specific bucket is the
//	               first such category name (lowercased).
//
// Cards with no categories at all default to "mainboard" — Archidekt
// treats them as legal-deck inclusion by default.
func classifyCard(c apiCard, included map[string]bool) (kind, bucket string) {
	if len(c.Categories) == 0 {
		return "mainboard", ""
	}
	for _, name := range c.Categories {
		low := strings.ToLower(strings.TrimSpace(name))
		// Commander match wins regardless of where it appears in the
		// list; a card in BOTH "Commander" and a side category is
		// still a commander.
		if commanderCategoryNames[low] {
			// Honor the IncludedInDeck flag: a user might define
			// a Commander category with the flag flipped off for
			// some out-of-format experiment.
			if isIncluded(low, included) {
				return "commander", ""
			}
		}
	}
	for _, name := range c.Categories {
		low := strings.ToLower(strings.TrimSpace(name))
		if isIncluded(low, included) {
			return "mainboard", ""
		}
	}
	// Every category this card lives in is excluded — route to the
	// first one's bucket so the output preserves the user's intent.
	first := strings.ToLower(strings.TrimSpace(c.Categories[0]))
	return "sideboard", first
}

// isIncluded answers "does this category count toward the legal deck."
// Categories not present in the response default to true (Archidekt's
// runtime behavior for free-form labels).
func isIncluded(low string, included map[string]bool) bool {
	if v, ok := included[low]; ok {
		return v
	}
	return true
}

// bucketComment maps an excluded-category name to the comment prefix
// the downstream deckparser already recognizes. Anything we don't
// know becomes a generic "// <Category>:" prefix.
func bucketComment(bucket string) string {
	switch bucket {
	case "sideboard":
		return "Sideboard"
	case "maybeboard":
		return "Maybeboard"
	case "considering":
		return "Considering"
	case "companion":
		return "Companion"
	default:
		// Preserve the category name but uppercase the first letter
		// so the comment stays human-readable.
		if bucket == "" {
			return "Sideboard"
		}
		return strings.ToUpper(bucket[:1]) + bucket[1:]
	}
}

// formatDecklist converts an Archidekt API response into our text
// decklist format. Deterministic output: every section is sorted by
// card name before emission, matching internal/moxfield's policy so
// repeat fetches produce byte-identical files.
func formatDecklist(data *apiResponse) (string, error) {
	included := inDeckCategoryIndex(data.Categories)

	commanders := []apiCard{}
	main := []apiCard{}
	// excluded is grouped by bucket name so the output emits one
	// "// <Bucket>:" line per card in deterministic order.
	excluded := map[string][]apiCard{}

	for _, c := range data.Cards {
		if c.Card.OracleCard.Name == "" || c.Quantity <= 0 {
			continue
		}
		kind, bucket := classifyCard(c, included)
		switch kind {
		case "commander":
			commanders = append(commanders, c)
		case "mainboard":
			main = append(main, c)
		default:
			excluded[bucket] = append(excluded[bucket], c)
		}
	}

	sortByName := func(xs []apiCard) {
		sort.SliceStable(xs, func(i, j int) bool {
			return xs[i].Card.OracleCard.Name < xs[j].Card.OracleCard.Name
		})
	}
	sortByName(commanders)
	sortByName(main)
	for k := range excluded {
		sortByName(excluded[k])
	}

	var sb strings.Builder
	for _, c := range commanders {
		sb.WriteString("COMMANDER: " + c.Card.OracleCard.Name + "\n")
	}
	for _, c := range main {
		fmt.Fprintf(&sb, "%d %s\n", c.Quantity, c.Card.OracleCard.Name)
	}
	// Sort the excluded bucket names so output ordering is stable across
	// runs even when Archidekt returns map keys in random order.
	bucketNames := make([]string, 0, len(excluded))
	for k := range excluded {
		bucketNames = append(bucketNames, k)
	}
	sort.Strings(bucketNames)
	for _, b := range bucketNames {
		label := bucketComment(b)
		for _, c := range excluded[b] {
			fmt.Fprintf(&sb, "// %s: %d %s\n", label, c.Quantity, c.Card.OracleCard.Name)
		}
	}

	if len(commanders) == 0 && len(main) == 0 {
		return "", fmt.Errorf("archidekt: deck %q is empty (no mainboard or commanders)", data.Name)
	}
	return sb.String(), nil
}
