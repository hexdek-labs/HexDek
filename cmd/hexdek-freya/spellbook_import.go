package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Commander Spellbook import.
//
// Mirrors the public Commander Spellbook variants API (json.commanderspellbook.com).
// We import the JSON, convert each variant to a KnownCombo, and dedupe
// against the hand-curated KnownCombos slice via canonical card-name keys.
//
// Network access is segregated: ParseSpellbookJSON works on bytes (used by
// tests), LoadSpellbookCache reads from disk, FetchSpellbook hits the URL.
// Tests use fixtures only — no live network.
// ---------------------------------------------------------------------------

// ImportedCombos holds Spellbook combos loaded at startup. Empty until a
// --spellbook cache is loaded via main.go. analysis.go reads via
// allKnownCombos() so the dedupe + merge happens in one place.
var ImportedCombos []KnownCombo

// DefaultSpellbookURL is the public bulk-variants endpoint.
const DefaultSpellbookURL = "https://json.commanderspellbook.com/variants.json"

// DefaultSpellbookCache is where main.go writes the downloaded JSON. Path
// is gitignored alongside other large data files; loader tolerates absence.
const DefaultSpellbookCache = "data/rules/spellbook.json"

// spellbookFile mirrors the documented Commander Spellbook variants payload.
// Unknown fields are ignored by encoding/json so the parser is resilient
// to upstream schema additions.
type spellbookFile struct {
	Variants []spellbookVariant `json:"variants"`
}

type spellbookVariant struct {
	ID          string             `json:"id"`
	Uses        []spellbookUse     `json:"uses"`
	Produces    []spellbookProduce `json:"produces"`
	Description string             `json:"description"`
	Identity    string             `json:"identity"`
	Status      string             `json:"status"`
}

type spellbookUse struct {
	Card     spellbookCardRef `json:"card"`
	Quantity int              `json:"quantity"`
}

type spellbookCardRef struct {
	Name string `json:"name"`
}

type spellbookProduce struct {
	Feature spellbookFeature `json:"feature"`
}

type spellbookFeature struct {
	Name string `json:"name"`
}

// ParseSpellbookJSON converts raw Spellbook JSON into KnownCombo entries.
// Returns the parsed combos plus a slice of human-readable warnings for
// variants that were skipped (missing pieces, unsupported status, etc.).
// An error is returned only for catastrophic JSON parse failures; partial
// data is preferred over no data.
func ParseSpellbookJSON(raw []byte) ([]KnownCombo, []string, error) {
	var file spellbookFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, nil, fmt.Errorf("spellbook: parse: %w", err)
	}

	combos := make([]KnownCombo, 0, len(file.Variants))
	warnings := []string{}
	seenKey := map[string]bool{}

	for _, v := range file.Variants {
		// Only import variants Spellbook has confirmed work. Statuses like
		// "DRAFT", "NEEDS_REVIEW", "NOT_WORKING" stay out of Freya's deck
		// analysis — surfacing them would hurt precision.
		if !spellbookStatusOK(v.Status) {
			continue
		}

		var pieces []string
		for _, u := range v.Uses {
			name := strings.TrimSpace(u.Card.Name)
			if name == "" {
				continue
			}
			pieces = append(pieces, name)
		}
		if len(pieces) < 2 {
			warnings = append(warnings,
				fmt.Sprintf("variant %s: needs >=2 cards, has %d", v.ID, len(pieces)))
			continue
		}

		// Intra-import dedupe: Spellbook can list the same card set under
		// multiple variant IDs (different prerequisite phrasings). One per
		// canonical key.
		key := canonicalComboKey(pieces)
		if seenKey[key] {
			continue
		}
		seenKey[key] = true

		combos = append(combos, KnownCombo{
			Name:        spellbookComboName(pieces, v.ID),
			Pieces:      pieces,
			Type:        spellbookInferType(v.Produces),
			Mandatory:   true, // Spellbook only lists working combos
			Description: spellbookDescription(v),
		})
	}

	return combos, warnings, nil
}

// spellbookStatusOK gates which Spellbook statuses Freya will trust.
// Empty status (older variant entries) is treated as OK; the canonical
// "OK" / "EXAMPLE" verbs are accepted; anything else is rejected.
func spellbookStatusOK(s string) bool {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "OK", "EXAMPLE":
		return true
	}
	return false
}

// spellbookInferType infers Freya's loop classification from the variant's
// produced features. "Win the game", "Infinite damage", "Infinite mill" →
// true_infinite (mandatory kill). Plain infinite resources → true_infinite
// (mandatory cycle). Everything else falls back to determined.
func spellbookInferType(produces []spellbookProduce) string {
	for _, p := range produces {
		n := strings.ToLower(p.Feature.Name)
		if strings.Contains(n, "win the game") || strings.Contains(n, "lose the game") {
			return "true_infinite"
		}
	}
	for _, p := range produces {
		n := strings.ToLower(p.Feature.Name)
		if strings.Contains(n, "infinite") {
			return "true_infinite"
		}
	}
	return "determined"
}

func spellbookDescription(v spellbookVariant) string {
	desc := strings.TrimSpace(v.Description)
	var features []string
	for _, p := range v.Produces {
		if n := strings.TrimSpace(p.Feature.Name); n != "" {
			features = append(features, n)
		}
	}
	header := "Imported from Commander Spellbook"
	if v.ID != "" {
		header += " (" + v.ID + ")"
	}
	if len(features) > 0 {
		header += " — produces: " + strings.Join(features, ", ")
	}
	if desc == "" {
		return header
	}
	return header + ". " + desc
}

func spellbookComboName(pieces []string, id string) string {
	if len(pieces) <= 3 {
		return strings.Join(pieces, " + ")
	}
	return fmt.Sprintf("%s + %d more (Spellbook %s)", pieces[0], len(pieces)-1, id)
}

// canonicalComboKey returns a normalized, order-independent key for a set
// of card names. Used for dedupe between curated and imported combos.
// Normalization: trim, NFC-ish lowercase, strip leading/trailing punctuation.
// Names with diacritics or apostrophes (e.g. "Jaya's Immolating Inferno")
// pass through ASCII-fold-free; Spellbook and Scryfall agree on spelling,
// so byte equality after lowercase suffices.
func canonicalComboKey(pieces []string) string {
	norm := make([]string, 0, len(pieces))
	for _, p := range pieces {
		n := strings.ToLower(strings.TrimSpace(p))
		if n == "" {
			continue
		}
		norm = append(norm, n)
	}
	sort.Strings(norm)
	return strings.Join(norm, "|")
}

// MergeKnownCombos dedupes imported Spellbook combos against the curated
// list. Curated entries always win (they carry hand-authored outlets/stops
// that Spellbook doesn't model). Returns (merged, dropped) where `dropped`
// lists imported combos that collided with a curated entry — useful for
// import-time reporting.
func MergeKnownCombos(curated, imported []KnownCombo) (merged, dropped []KnownCombo) {
	curatedKeys := map[string]bool{}
	for _, c := range curated {
		curatedKeys[canonicalComboKey(c.Pieces)] = true
	}

	merged = make([]KnownCombo, 0, len(curated)+len(imported))
	merged = append(merged, curated...)

	importedKeys := map[string]bool{}
	for _, c := range imported {
		key := canonicalComboKey(c.Pieces)
		if curatedKeys[key] {
			dropped = append(dropped, c)
			continue
		}
		if importedKeys[key] {
			// Already added an imported copy of this combo; skip.
			dropped = append(dropped, c)
			continue
		}
		importedKeys[key] = true
		merged = append(merged, c)
	}
	return merged, dropped
}

// LoadSpellbookCache reads a Spellbook JSON cache file from disk and
// returns the parsed combos. Returns (nil, nil, nil) when the file is
// absent — callers treat that as "no spellbook configured" rather than an
// error, so Freya runs fine on machines that haven't fetched the dump.
func LoadSpellbookCache(path string) ([]KnownCombo, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("spellbook cache: %w", err)
	}
	return ParseSpellbookJSON(data)
}

// FetchSpellbook downloads the Spellbook variants JSON from url and writes
// it to cachePath atomically (write to .tmp, then rename). Caller is
// responsible for choosing when to fetch — Freya does not auto-refresh on
// every invocation; the cache file is the source of truth at deck-analysis
// time. Returns the byte count written.
func FetchSpellbook(url, cachePath string) (int, error) {
	client := &http.Client{Timeout: 90 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("spellbook fetch: build request: %w", err)
	}
	req.Header.Set("User-Agent", "hexdek-freya/1.0 (+https://hexdek.dev)")
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("spellbook fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("spellbook fetch: HTTP %d", resp.StatusCode)
	}

	tmp := cachePath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, fmt.Errorf("spellbook fetch: create tmp: %w", err)
	}
	n, err := io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return int(n), fmt.Errorf("spellbook fetch: write: %w", err)
	}
	if err := os.Rename(tmp, cachePath); err != nil {
		os.Remove(tmp)
		return int(n), fmt.Errorf("spellbook fetch: rename: %w", err)
	}
	return int(n), nil
}

// allKnownCombos returns the merged set of curated + imported combos.
// analysis.go uses this in place of KnownCombos directly so the dedupe
// happens once per call rather than scattered across the file.
func allKnownCombos() []KnownCombo {
	if len(ImportedCombos) == 0 {
		return KnownCombos
	}
	// ImportedCombos arrive from Commander Spellbook without a curated
	// Class; backfill via the heuristic classifier before the merge so
	// downstream consumers see a uniformly-classified set.
	ClassifySpellbookImported(ImportedCombos)
	merged, _ := MergeKnownCombos(KnownCombos, ImportedCombos)
	return merged
}
