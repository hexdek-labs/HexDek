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
	"regexp"
	"sort"
	"strings"
	"time"
)

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
	apiURL := fmt.Sprintf("https://api2.moxfield.com/v3/decks/all/%s", deckID)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("moxfield: build request: %w", err)
	}
	// Moxfield API requires a User-Agent header.
	req.Header.Set("User-Agent", "hexdek/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("moxfield: fetch %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("moxfield: API returned %d for deck %s: %s", resp.StatusCode, deckID, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("moxfield: read response: %w", err)
	}

	var data apiResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return "", fmt.Errorf("moxfield: parse JSON: %w", err)
	}

	return formatDecklist(&data)
}

// FetchDeckName downloads just the deck name from a Moxfield URL.
func FetchDeckName(url string) (string, error) {
	deckID := ExtractDeckID(url)
	if deckID == "" {
		return "", fmt.Errorf("moxfield: could not extract deck ID from URL %q", url)
	}

	apiURL := fmt.Sprintf("https://api2.moxfield.com/v3/decks/all/%s", deckID)
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("moxfield: build request: %w", err)
	}
	req.Header.Set("User-Agent", "hexdek/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("moxfield: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("moxfield: API returned %d", resp.StatusCode)
	}

	var data struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("moxfield: parse: %w", err)
	}
	return data.Name, nil
}

// formatDecklist converts a Moxfield API response into our text decklist format.
//
// Each board is iterated in card-name order so the output is deterministic.
// Partner decks emit two COMMANDER: lines; without sorting, Go's randomized
// map iteration would flip which partner appears first across fetches, and
// the downstream deckparser treats the first commander as primary.
func formatDecklist(data *apiResponse) (string, error) {
	var sb strings.Builder

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

	result := sb.String()
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("moxfield: deck %q is empty (no mainboard or commanders)", data.Name)
	}

	return result, nil
}
