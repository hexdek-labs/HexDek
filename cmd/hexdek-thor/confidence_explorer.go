package main

// Confidence explorer — walks the AST corpus, ranks cards by
// CardConfidence, and surfaces the N lowest-scoring entries with their
// problematic AST nodes. Used to target scaffold improvements at the
// cards with the worst parse quality.
//
// Driven by main.go's --confidence-explorer flag. Output format
// (markdown by default; plain text on stdout if --confidence-explorer-out
// is empty):
//
//   ## #N  Card Name  (score 0.18, X abilities, Y ability-fallbacks)
//
//   - ability[0] (static, score 0.20): static_fallback_cond_kind:conditional,
//                                       static_fallback_mod_kind:parsed_tail
//   - ability[1] (triggered, score 1.00): (no issues)
//   ...

import (
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
)

// runConfidenceExplorer is the --confidence-explorer entry point. It
// walks the loaded corpus, computes confidence, surfaces the bottom-N,
// and writes the report to outPath (or stdout if outPath is empty).
func runConfidenceExplorer(corpus *astload.Corpus, limit int, outPath string) error {
	entries := CollectExplorerEntries(corpus, limit)
	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		w = f
	}
	if err := RenderExplorerMarkdown(w, entries, ""); err != nil {
		return err
	}
	if err := RenderReasonSummary(w, entries); err != nil {
		return err
	}
	if outPath != "" {
		log.Printf("confidence-explorer: wrote %d entries to %s", len(entries), outPath)
	}
	return nil
}

// ExplorerEntry is one card's confidence-explorer row. Exported for
// the test harness so synthetic inputs can be asserted on directly.
type ExplorerEntry struct {
	Name         string
	Score        float64
	MinScore     float64
	NumAbilities int
	NumFallback  int
	Abilities    []ExplorerAbility
}

// ExplorerAbility is one ability's row inside an ExplorerEntry.
type ExplorerAbility struct {
	Index   int
	Kind    string
	Score   float64
	Reasons []string
}

// CollectExplorerEntries walks every card in corpus, computes its
// CardConfidence, and returns the entries sorted by ascending score
// (worst first). Limit clamps the result to at most N entries; pass 0
// for unlimited.
func CollectExplorerEntries(corpus *astload.Corpus, limit int) []ExplorerEntry {
	if corpus == nil {
		return nil
	}
	cards := make([]*gameast.CardAST, 0, corpus.CardCount)
	for _, name := range corpus.Names() {
		if c, ok := corpus.Get(name); ok && c != nil {
			cards = append(cards, c)
		}
	}
	return CollectExplorerEntriesFromCards(cards, limit)
}

// CollectExplorerEntriesFromCards is the core scoring + sort + cap
// logic, split out so tests can drive it with synthetic *CardAST
// values without going through the JSONL deserialization layer.
//
// Ties on Score break on Name (alphabetical) for deterministic output.
// Cards with no abilities are skipped — their confidence is vacuously
// 1.0 and they have nothing useful to surface. Cards that score at
// 1.0 AND have zero per-ability fallbacks are also dropped: the
// explorer focuses on the improvement-worthy tail.
func CollectExplorerEntriesFromCards(cards []*gameast.CardAST, limit int) []ExplorerEntry {
	entries := make([]ExplorerEntry, 0, len(cards))
	for _, card := range cards {
		if card == nil || len(card.Abilities) == 0 {
			continue
		}
		entry := buildExplorerEntry(card)
		if entry.NumFallback == 0 && entry.Score >= 1.0 {
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score < entries[j].Score
		}
		return entries[i].Name < entries[j].Name
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

// buildExplorerEntry packages one card's per-ability scoring detail.
func buildExplorerEntry(card *gameast.CardAST) ExplorerEntry {
	entry := ExplorerEntry{
		Name:         card.Name,
		Score:        gameast.CardConfidence(card),
		MinScore:     gameast.CardMinConfidence(card),
		NumAbilities: len(card.Abilities),
	}
	for i, ab := range card.Abilities {
		row := ExplorerAbility{
			Index:   i,
			Kind:    "?",
			Score:   gameast.AbilityConfidence(ab),
			Reasons: gameast.LowConfidenceReasons(ab),
		}
		if ab != nil {
			row.Kind = ab.Kind()
		}
		if len(row.Reasons) > 0 {
			entry.NumFallback++
		}
		entry.Abilities = append(entry.Abilities, row)
	}
	return entry
}

// RenderExplorerMarkdown writes the entries as a markdown report. Pass
// title to override the default header; pass empty string for default.
func RenderExplorerMarkdown(w io.Writer, entries []ExplorerEntry, title string) error {
	if title == "" {
		title = fmt.Sprintf("# AST Confidence Explorer — bottom %d cards", len(entries))
	}
	if _, err := fmt.Fprintln(w, title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintln(w, "_(no low-confidence cards in corpus)_")
		return err
	}
	if _, err := fmt.Fprintf(w,
		"Each entry shows the per-ability score + low-confidence reasons. Reasons map to the parser-side fallback kinds in `internal/gameast/confidence.go` — fixing the most common reasons across the worst entries gives the biggest scaffold-coverage uplift.\n\n",
	); err != nil {
		return err
	}
	for rank, e := range entries {
		header := fmt.Sprintf("## #%d %s  (score %.2f, %d abilities, %d ability-fallbacks)",
			rank+1, e.Name, e.Score, e.NumAbilities, e.NumFallback)
		if _, err := fmt.Fprintln(w, header); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		for _, ab := range e.Abilities {
			reasons := "(no issues)"
			if len(ab.Reasons) > 0 {
				reasons = strings.Join(ab.Reasons, ", ")
			}
			line := fmt.Sprintf("- ability[%d] (%s, score %.2f): %s",
				ab.Index, ab.Kind, ab.Score, reasons)
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

// AggregateReasons returns a {reason → count} map across all
// entries.Abilities[].Reasons. Useful for the operator deciding which
// parser fallback kind to target next: the highest-count reason in the
// bottom-N is the biggest single improvement target.
func AggregateReasons(entries []ExplorerEntry) map[string]int {
	out := make(map[string]int)
	for _, e := range entries {
		for _, ab := range e.Abilities {
			for _, r := range ab.Reasons {
				out[r]++
			}
		}
	}
	return out
}

// RenderReasonSummary emits a sorted summary of the AggregateReasons
// histogram, ranked by count desc. Appended below the per-card list
// in the canonical markdown report.
func RenderReasonSummary(w io.Writer, entries []ExplorerEntry) error {
	hist := AggregateReasons(entries)
	if len(hist) == 0 {
		return nil
	}
	type row struct {
		reason string
		count  int
	}
	rows := make([]row, 0, len(hist))
	for r, c := range hist {
		rows = append(rows, row{r, c})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].reason < rows[j].reason
	})
	if _, err := fmt.Fprintln(w, "## Aggregate reason histogram"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w,
		"Reason counts across the surfaced entries. Highest = biggest single-fix improvement target."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Reason | Count |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "|--------|-------|"); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := fmt.Fprintf(w, "| `%s` | %d |\n", r.reason, r.count); err != nil {
			return err
		}
	}
	return nil
}
