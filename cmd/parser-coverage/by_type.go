package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// TypeGroup is one card-type's aggregate coverage stats — mirrors
// SetGroup in by_set.go, but keyed off primaryType(type_line) so the
// report buckets to Creature / Instant / Sorcery / Land / Artifact /
// Enchantment / Planeswalker / Battle / Saga / etc. instead of by set.
//
// Bucketing follows primaryType's first-non-supertype rule: a card
// with type "Artifact Creature — Construct" buckets under Artifact
// because that's the first non-supertype word. This is documented
// behavior of the shared helper; the report's reading-guide footer
// calls it out so readers don't double-count Hangarback-style cards
// under "Creature".
type TypeGroup struct {
	TypeName  string
	Total     int
	Uncovered int
	Missing   int
	EmptyAST  int
	Partial   int
	OK        int
	OKVanilla int
}

func (g TypeGroup) CoveragePct() float64 {
	if g.Total == 0 {
		return 0
	}
	return 100.0 * float64(g.OK+g.OKVanilla) / float64(g.Total)
}

// groupByType rolls every result up by primaryType. Cards whose
// type_line yields an empty primary type bucket under "(none)" so
// they aren't silently dropped — that bucket is normally tiny but
// useful as a tripwire if the parser starts producing unexpected
// shapes.
//
// Output order: descending by Uncovered count, then ascending by
// TypeName for stable tiebreak. Same prioritized-worklist shape as
// the --by-set report.
func groupByType(results []result) []TypeGroup {
	m := map[string]*TypeGroup{}
	for _, r := range results {
		t := primaryType(r.TypeLine)
		if t == "" {
			t = "(none)"
		}
		g, ok := m[t]
		if !ok {
			g = &TypeGroup{TypeName: t}
			m[t] = g
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
	out := make([]TypeGroup, 0, len(m))
	for _, g := range m {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Uncovered != out[j].Uncovered {
			return out[i].Uncovered > out[j].Uncovered
		}
		return out[i].TypeName < out[j].TypeName
	})
	return out
}

// writeByTypeReport writes a markdown report grouping cards by
// primary type. Types are bounded (~10 in practice) so there's no
// top-N flag — the whole table is always emitted.
//
// Coverage% appears next to absolute counts so a reader can spot
// type-areas that are systematically under-covered (e.g.,
// "Enchantment coverage is 60% — that's worse than Creature 95%,
// even though there are fewer enchantments overall").
func writeByTypeReport(path string, groups []TypeGroup) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# Parser Coverage by Card Type\n\n")
	fmt.Fprintf(f, "Cards grouped by primary type (Creature / Instant / Sorcery /\n")
	fmt.Fprintf(f, "Land / Artifact / Enchantment / Planeswalker / Battle / …),\n")
	fmt.Fprintf(f, "ranked by uncovered count. Useful for finding type-areas where\n")
	fmt.Fprintf(f, "the parser has systematic gaps — a low Coverage%% in one type is\n")
	fmt.Fprintf(f, "a stronger signal than a high absolute Uncovered count, because\n")
	fmt.Fprintf(f, "it points at a shape the parser handles worse on average.\n\n")

	fmt.Fprintf(f, "## Summary\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(f, "| Primary types in audit | %d |\n", len(groups))
	var totUnc, totTot int
	for _, g := range groups {
		totUnc += g.Uncovered
		totTot += g.Total
	}
	fmt.Fprintf(f, "| Uncovered | %d |\n", totUnc)
	fmt.Fprintf(f, "| Total cards | %d |\n\n", totTot)

	fmt.Fprintf(f, "## Types\n\n")
	fmt.Fprintf(f, "| Rank | Type | Total | Uncovered | Coverage | MISSING | EMPTY_AST | PARTIAL |\n")
	fmt.Fprintf(f, "|---:|---|---:|---:|---:|---:|---:|---:|\n")
	for i, g := range groups {
		fmt.Fprintf(f, "| %d | %s | %d | %d | %.1f%% | %d | %d | %d |\n",
			i+1, escapeCell(g.TypeName), g.Total, g.Uncovered, g.CoveragePct(),
			g.Missing, g.EmptyAST, g.Partial)
	}
	fmt.Fprintln(f)

	fmt.Fprintf(f, "## Reading this report\n\n")
	fmt.Fprintf(f, "- **Systematic gaps** — a type with low Coverage%% indicates the\n")
	fmt.Fprintf(f, "  parser doesn't handle that shape well; that's where new\n")
	fmt.Fprintf(f, "  scaffolds buy the most coverage per change.\n")
	fmt.Fprintf(f, "- **Volume vs rate** — Creatures will usually have the highest\n")
	fmt.Fprintf(f, "  absolute uncovered count just because there are so many of\n")
	fmt.Fprintf(f, "  them; the Coverage%% column normalizes for size.\n")
	fmt.Fprintf(f, "- **Multi-type cards** — cards like Hangarback Walker (\"Artifact\n")
	fmt.Fprintf(f, "  Creature — Construct\") bucket by the first non-supertype word\n")
	fmt.Fprintf(f, "  in the type line, so they count under Artifact, not Creature.\n")
	fmt.Fprintf(f, "- **(none)** aggregates entries whose type line yielded no\n")
	fmt.Fprintf(f, "  primary-type word (usually data quirks; should be a small bucket).\n")

	return nil
}

// formatByTypeSummary returns a one-line stderr-friendly summary of
// the top types by uncovered count, useful for logging alongside the
// other set/type/history one-liners.
func formatByTypeSummary(groups []TypeGroup, n int) string {
	if len(groups) == 0 {
		return "by-type: no uncovered cards"
	}
	if n <= 0 || n > len(groups) {
		n = len(groups)
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, fmt.Sprintf("%s=%d", groups[i].TypeName, groups[i].Uncovered))
	}
	return "by-type top: " + strings.Join(parts, ", ")
}
