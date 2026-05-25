package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// UncoveredCardEntry is one uncovered card's contribution to a set's
// priority score: how often it appears across the deck corpus.
type UncoveredCardEntry struct {
	Name      string
	Class     string
	Frequency int // number of distinct decks containing this card
}

// SetPriority is one set's deck-weighted scaffold-priority score.
// Score = sum of deck-frequencies of every uncovered card from the
// set. A set with 10 uncovered cards each appearing in 50 decks
// (score 500) beats a set with 100 uncovered cards none of which
// appear in any deck (score 0) — which is the right ordering for
// "what should Thor scaffold next" because the cards nobody plays
// don't actually unblock anything.
type SetPriority struct {
	SetName        string
	Score          int
	UncoveredCount int
	TopCards       []UncoveredCardEntry
}

// scanDeckFrequencies walks every .txt deck under deckDir and returns
// a map from card name to the number of distinct decks the card
// appears in. Lines are parsed in two shapes:
//
//   "COMMANDER: <Name>"       — exact prefix, the commander row
//   "<count> <Name>"          — Moxfield-style entry
//
// Multiple copies of a card in the same deck (e.g. basics) count
// once, since we're measuring how many DECKS care about a card, not
// how many physical copies exist.
//
// Missing directory → returns (empty map, nil) so the caller can
// gracefully degrade rather than abort on a fresh checkout. Other
// errors propagate.
func scanDeckFrequencies(deckDir string) (map[string]int, int, error) {
	if _, err := os.Stat(deckDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]int{}, 0, nil
		}
		return nil, 0, err
	}
	freq := map[string]int{}
	deckCount := 0
	err := filepath.Walk(deckDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		deckCount++
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		seen := map[string]bool{}
		s := bufio.NewScanner(f)
		for s.Scan() {
			name := parseDeckLine(s.Text())
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			freq[name]++
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return freq, deckCount, nil
}

// parseDeckLine extracts a card name from a single deck-file line.
// Returns "" for blank lines, comments, sideboard markers, etc.
//
// Accepted shapes:
//
//	"COMMANDER: <Name>"   — case-sensitive prefix
//	"1 Sol Ring"          — count + space + name
//	"4x Lightning Bolt"   — count with trailing 'x' (some exporters)
func parseDeckLine(raw string) string {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
		return ""
	}
	if strings.HasPrefix(line, "COMMANDER:") {
		return strings.TrimSpace(strings.TrimPrefix(line, "COMMANDER:"))
	}
	// Try Moxfield-style "<count> <name>" or "<count>x <name>".
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	count := strings.TrimSuffix(parts[0], "x")
	if _, err := strconv.Atoi(count); err != nil {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// computeSetPriority builds the priority ranking from a classified
// result set + deck-frequency map. topN controls how many top-cards
// each set carries through (0 = unlimited; for display, the caller
// usually wants ~5).
//
// Output is sorted by Score descending, then SetName ascending for a
// stable tiebreak. Sets with zero score (no uncovered card from
// the set appears in any deck) are kept in the result so the caller
// can decide whether to truncate — the writer drops them.
func computeSetPriority(results []result, freq map[string]int, topN int) []SetPriority {
	type setAgg struct {
		score     int
		uncovered int
		cards     []UncoveredCardEntry
	}
	agg := map[string]*setAgg{}
	for _, r := range results {
		switch r.Class {
		case classMissing, classEmptyAST, classPartial:
		default:
			continue
		}
		setName := r.SetName
		if setName == "" {
			setName = "(unknown)"
		}
		a, ok := agg[setName]
		if !ok {
			a = &setAgg{}
			agg[setName] = a
		}
		f := freq[r.Name]
		a.score += f
		a.uncovered++
		a.cards = append(a.cards, UncoveredCardEntry{
			Name: r.Name, Class: r.Class.String(), Frequency: f,
		})
	}

	out := make([]SetPriority, 0, len(agg))
	for name, a := range agg {
		sort.Slice(a.cards, func(i, j int) bool {
			if a.cards[i].Frequency != a.cards[j].Frequency {
				return a.cards[i].Frequency > a.cards[j].Frequency
			}
			return a.cards[i].Name < a.cards[j].Name
		})
		top := a.cards
		if topN > 0 && len(top) > topN {
			top = top[:topN]
		}
		out = append(out, SetPriority{
			SetName: name, Score: a.score, UncoveredCount: a.uncovered, TopCards: top,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].SetName < out[j].SetName
	})
	return out
}

// writeSetPriorityReport renders the deck-weighted set-priority
// ranking as a markdown worklist for the Thor team. Sets with score
// == 0 are dropped — they have uncovered cards but nobody plays them,
// so they don't belong on the next-up scaffold queue. limit > 0
// truncates the table to the top-N priority sets.
func writeSetPriorityReport(path string, priorities []SetPriority, deckCount int, limit int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Drop zero-score rows: they're noise for this report.
	active := priorities[:0]
	for _, p := range priorities {
		if p.Score > 0 {
			active = append(active, p)
		}
	}
	totalConsidered := len(active)
	if limit > 0 && limit < len(active) {
		active = active[:limit]
	}

	fmt.Fprintf(f, "# Parser Coverage — Set Priority (deck-weighted)\n\n")
	fmt.Fprintf(f, "Sets ranked by **deck-weighted priority score** — the sum of\n")
	fmt.Fprintf(f, "deck-appearance counts for every uncovered card in the set.\n")
	fmt.Fprintf(f, "Unlike `--by-set` (which ranks on raw uncovered count and gets\n")
	fmt.Fprintf(f, "dominated by Art Series filler nobody plays), this report\n")
	fmt.Fprintf(f, "answers \"what should Thor scaffold next to unblock the most\n")
	fmt.Fprintf(f, "decks at once?\". Zero-score sets are omitted.\n\n")

	fmt.Fprintf(f, "## Headline\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(f, "| Decks scanned | %d |\n", deckCount)
	fmt.Fprintf(f, "| Sets with at least one deck-relevant uncovered card | %d |\n", totalConsidered)
	if limit > 0 && limit < totalConsidered {
		fmt.Fprintf(f, "| Sets shown (top-N) | %d |\n", len(active))
	}
	fmt.Fprintln(f)

	fmt.Fprintf(f, "## Priority queue\n\n")
	if len(active) == 0 {
		fmt.Fprintln(f, "_No uncovered cards intersect any scanned deck — nothing prioritized._")
		return nil
	}
	fmt.Fprintf(f, "| Rank | Set | Score | Uncovered cards | Top cards (deck count · class) |\n")
	fmt.Fprintf(f, "|---:|---|---:|---:|---|\n")
	for i, p := range active {
		topParts := make([]string, 0, len(p.TopCards))
		for _, c := range p.TopCards {
			topParts = append(topParts,
				fmt.Sprintf("%s (%d · %s)", escapeCell(c.Name), c.Frequency, c.Class))
		}
		fmt.Fprintf(f, "| %d | %s | %d | %d | %s |\n",
			i+1, escapeCell(p.SetName), p.Score, p.UncoveredCount,
			strings.Join(topParts, "; "))
	}
	fmt.Fprintln(f)

	fmt.Fprintf(f, "## Reading this report\n\n")
	fmt.Fprintf(f, "- **Score** is the sum of deck-appearance counts across every\n")
	fmt.Fprintf(f, "  uncovered card from the set. Higher score = more decks unblocked\n")
	fmt.Fprintf(f, "  per scaffold landed.\n")
	fmt.Fprintf(f, "- **Top cards** are the highest-leverage individual targets within\n")
	fmt.Fprintf(f, "  the set; each entry shows the card's deck-appearance count and\n")
	fmt.Fprintf(f, "  classifier bucket (MISSING / EMPTY_AST / PARTIAL).\n")
	fmt.Fprintf(f, "- A MISSING entry usually means Thor never ingested the card\n")
	fmt.Fprintf(f, "  (re-run with a fresh `oracle-cards.json`); EMPTY_AST and\n")
	fmt.Fprintf(f, "  PARTIAL indicate the parser needs handler/scaffold work.\n")

	return nil
}
