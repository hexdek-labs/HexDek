package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// SetGroup is one Magic set's aggregate coverage stats. Total counts
// every audited card from the set; Uncovered = Missing + EmptyAST +
// Partial. The OK/OKVanilla split is kept separately so the report
// can compute a coverage rate and not just an uncovered-count
// ranking — a set with 200 cards and 10 uncovered is a worse
// priority than a set with 30 cards and 10 uncovered.
type SetGroup struct {
	SetName   string
	Total     int
	Uncovered int
	Missing   int
	EmptyAST  int
	Partial   int
	OK        int
	OKVanilla int
}

// CoveragePct returns 100 * (OK + OKVanilla) / Total. Returns 0 for
// the empty-set edge case so callers can rank/sort without a guard.
func (g SetGroup) CoveragePct() float64 {
	if g.Total == 0 {
		return 0
	}
	return 100.0 * float64(g.OK+g.OKVanilla) / float64(g.Total)
}

// groupBySet rolls every result up by SetName. Cards with no
// SetName (legacy data, hand-built fixtures, etc.) fall into the
// "(unknown)" bucket so they aren't silently dropped.
//
// Output order: descending by Uncovered count, then ascending by
// SetName for stable tiebreaking. This puts the largest backlogs at
// the top so the report works as a prioritized worklist.
func groupBySet(results []result) []SetGroup {
	m := map[string]*SetGroup{}
	for _, r := range results {
		name := r.SetName
		if name == "" {
			name = "(unknown)"
		}
		g, ok := m[name]
		if !ok {
			g = &SetGroup{SetName: name}
			m[name] = g
		}
		g.Total++
		switch r.Class {
		case classMissing:
			g.Missing++
			g.Uncovered++
		case classEmptyAST:
			g.EmptyAST++
			g.Uncovered++
		case classPartial:
			g.Partial++
			g.Uncovered++
		case classOK:
			g.OK++
		case classOKVanilla:
			g.OKVanilla++
		}
	}
	out := make([]SetGroup, 0, len(m))
	for _, g := range m {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Uncovered != out[j].Uncovered {
			return out[i].Uncovered > out[j].Uncovered
		}
		return out[i].SetName < out[j].SetName
	})
	return out
}

// writeBySetReport writes a markdown report grouping uncovered cards
// by Magic set. topN > 0 limits the table to the topN highest-priority
// sets (by Uncovered count); topN <= 0 emits every set.
//
// The report intentionally puts coverage% next to uncovered count so
// a reader can spot the difference between "big set with low coverage"
// (set rollout priority) vs "big set with high coverage but absolute
// uncovered count is high anyway" (long-tail polish).
func writeBySetReport(path string, groups []SetGroup, topN int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	totalSets := len(groups)
	if topN > 0 && topN < len(groups) {
		groups = groups[:topN]
	}

	fmt.Fprintf(f, "# Parser Coverage by Set\n\n")
	fmt.Fprintf(f, "Cards grouped by Scryfall `set_name`, ranked by uncovered count.\n")
	fmt.Fprintf(f, "Use this report as a prioritized worklist for set-rollout cleanup —\n")
	fmt.Fprintf(f, "sets at the top have the most parser gaps to close. Coverage%% is\n")
	fmt.Fprintf(f, "shown alongside the absolute count so a small set with a few uncovered\n")
	fmt.Fprintf(f, "cards isn't crowded out by a large set with the same backlog.\n\n")

	fmt.Fprintf(f, "## Summary\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(f, "| Sets in audit | %d |\n", totalSets)
	if topN > 0 && topN < totalSets {
		fmt.Fprintf(f, "| Sets shown (top-N) | %d |\n", len(groups))
	}
	var totUnc, totTot int
	for _, g := range groups {
		totUnc += g.Uncovered
		totTot += g.Total
	}
	fmt.Fprintf(f, "| Uncovered (shown) | %d |\n", totUnc)
	fmt.Fprintf(f, "| Total cards (shown) | %d |\n\n", totTot)

	fmt.Fprintf(f, "## Sets\n\n")
	fmt.Fprintf(f, "| Rank | Set | Total | Uncovered | Coverage | MISSING | EMPTY_AST | PARTIAL |\n")
	fmt.Fprintf(f, "|---:|---|---:|---:|---:|---:|---:|---:|\n")
	for i, g := range groups {
		fmt.Fprintf(f, "| %d | %s | %d | %d | %.1f%% | %d | %d | %d |\n",
			i+1, escapeCell(g.SetName), g.Total, g.Uncovered, g.CoveragePct(),
			g.Missing, g.EmptyAST, g.Partial)
	}
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "## Reading this report\n\n")
	fmt.Fprintf(f, "- **Set rollout priority** — high `Uncovered` AND low `Coverage` is the strongest signal.\n")
	fmt.Fprintf(f, "- **Long-tail polish** — high `Uncovered` but also high `Coverage` means a big set with one stubborn mechanic.\n")
	fmt.Fprintf(f, "- **MISSING vs EMPTY_AST vs PARTIAL** — `MISSING` is a Thor ingest gap, `EMPTY_AST`/`PARTIAL` are parser gaps.\n")
	fmt.Fprintf(f, "- `(unknown)` set name aggregates entries whose Scryfall record lacks `set_name`.\n")

	return nil
}

// formatBySetSummary returns a one-line stderr-friendly summary of
// the top few sets, useful for logging next to the existing --history
// delta line so a CI run shows both progress and worklist at a glance.
func formatBySetSummary(groups []SetGroup, n int) string {
	if len(groups) == 0 {
		return "by-set: no uncovered cards"
	}
	if n <= 0 || n > len(groups) {
		n = len(groups)
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, fmt.Sprintf("%s=%d", groups[i].SetName, groups[i].Uncovered))
	}
	return "by-set top: " + strings.Join(parts, ", ")
}
