package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// MechanicCoverage is one templated mechanic's aggregate coverage
// stats. Where by_set buckets by Magic set and by_type buckets by
// primary card type, this bucket is by GAMEPLAY MECHANIC (Flashback,
// Adventure, Saga, Mutate, Prototype, Modal DFC, ...) — the dimension
// that maps most directly to parser-handler ownership.
//
// A set-level report can hide that "Throne of Eldraine has 80%
// coverage" while still saying nothing about Adventure handling
// specifically (Eldraine contained both Adventure cards and non-
// Adventure cards). A mechanic-level report answers "of the cards
// that use Adventure templating, what fraction are parsing right?" —
// which is the question a parser maintainer asks before opening
// scripts/mtg_ast.py.
type MechanicCoverage struct {
	Name      string
	Total     int
	Uncovered int
	Missing   int
	EmptyAST  int
	Partial   int
	OK        int
	OKVanilla int
}

func (m MechanicCoverage) CoveragePct() float64 {
	if m.Total == 0 {
		return 0
	}
	return 100.0 * float64(m.OK+m.OKVanilla) / float64(m.Total)
}

// mechanicMatcher classifies a single result. Returns true when the
// result's oracle text + type line indicate this card uses the
// mechanic. Matchers are lowercase substring tests on r.OracleText
// and r.TypeLine; they're intentionally conservative — a wrong
// match inflates the mechanic's denominator and dilutes its coverage
// signal, which is worse than missing a few edge cases.
type mechanicMatcher func(r result) bool

// mechanicDefs is the curated set of templated mechanics we track.
// Choice criteria: (a) the mechanic has DEDICATED parser-handler
// logic in scripts/mtg_ast.py (so a coverage hit/miss directly
// corresponds to a maintainable surface); (b) the keyword/template
// appears across enough cards (≥10) that a percentage is meaningful;
// (c) the match expression is precise enough to avoid huge false-
// positive sets (e.g., NOT "draw" — too generic).
//
// Vanilla evergreen keywords (flying, vigilance, lifelink, etc.)
// are intentionally excluded: parser handles them as a flat
// Keyword node and rarely fails, so they'd swamp the report with
// "99% coverage" noise.
//
// Order in this slice is the stable display order BEFORE the sort —
// the final report sorts by Uncovered DESC. Add new entries in
// alphabetical order so a future audit knows where to place them.
var mechanicDefs = []struct {
	name  string
	match mechanicMatcher
}{
	{"adventure", typeLineMatcher("adventure")},
	{"adapt", oracleMatcher("adapt ")},
	{"battle", typeLineMatcher("battle")},
	{"bestow", oracleMatcher("bestow")},
	{"choose_modal", func(r result) bool {
		o := strings.ToLower(r.OracleText)
		return strings.Contains(o, "choose one") ||
			strings.Contains(o, "choose two") ||
			strings.Contains(o, "choose up to")
	}},
	{"class", typeLineMatcher("class")},
	{"companion", oracleMatcher("companion")},
	{"connive", oracleMatcher("connive")},
	{"crew", oracleMatcher("crew ")},
	{"cycling", oracleMatcher("cycling")},
	{"dredge", oracleMatcher("dredge ")},
	{"escape", oracleMatcher("escape—")},
	{"evoke", oracleMatcher("evoke ")},
	{"explore", oracleMatcher("explore")},
	{"flashback", oracleMatcher("flashback ")},
	{"kicker", oracleMatcher("kicker ")},
	{"madness", oracleMatcher("madness ")},
	{"meld", func(r result) bool {
		o := strings.ToLower(r.OracleText)
		return strings.Contains(o, "melds with")
	}},
	{"modal_dfc_transform", func(r result) bool {
		// MDFCs and DFCs are layout-keyed in Scryfall:
		//   - layout=modal_dfc  → Zendikar Rising-style MDFCs
		//                          (spell/creature front, land back)
		//   - layout=transform  → Innistrad-style DFCs (werewolves,
		//                          Garruk/Liliana DFC planeswalkers,
		//                          Eldritch Moon meld pairs, etc.)
		// Both layouts print front/back faces, store oracle_text=""
		// at the top level, and put per-face text in card_faces[i].
		// The previous heuristic checked r.OracleText for "transform"
		// or "//", but loadOracle's substitution only loads
		// card_faces[0].oracle_text — which never contains "//" and
		// only contains "transform" for the subset of DFCs that print
		// the keyword on the front face. Result: 90 of 96 MDFCs
		// silently missed the bucket entirely, and the report claimed
		// "100% coverage" on a population that excluded most actual
		// MDFCs.
		//
		// Layout-keyed detection is exact: Scryfall's layout field is
		// authoritative for these card kinds (we already filter
		// art_series / double_faced_token via set_type elsewhere).
		// Falls back to the legacy oracle-text "transform" check so
		// any future card that uses the keyword without one of the
		// canonical layouts (e.g. one-shot transform effects on
		// single-face cards) is still captured.
		if r.Layout == "modal_dfc" || r.Layout == "transform" {
			return true
		}
		o := strings.ToLower(r.OracleText)
		return strings.Contains(o, "transform")
	}},
	{"monstrosity", oracleMatcher("monstrosity")},
	{"mutate", oracleMatcher("mutate ")},
	{"partner", oracleMatcher("partner")},
	{"prototype", oracleMatcher("prototype ")},
	{"saga", typeLineMatcher("saga")},
	{"suspend", oracleMatcher("suspend ")},
	{"surveil", oracleMatcher("surveil ")},
	{"venture", oracleMatcher("venture into the dungeon")},
	{"ward", oracleMatcher("ward ")},
}

// oracleMatcher and typeLineMatcher are convenience constructors so
// the mechanicDefs slice reads as a flat table. Both match
// case-insensitively via lowercased substring.
func oracleMatcher(needle string) mechanicMatcher {
	low := strings.ToLower(needle)
	return func(r result) bool {
		return strings.Contains(strings.ToLower(r.OracleText), low)
	}
}

func typeLineMatcher(needle string) mechanicMatcher {
	low := strings.ToLower(needle)
	return func(r result) bool {
		return strings.Contains(strings.ToLower(r.TypeLine), low)
	}
}

// groupByMechanic walks every result against every mechanic matcher
// (M × N — M ≈ 30 mechanics, N ≈ 40k cards — bounded and fast).
// A single card can count toward MULTIPLE mechanics if its text
// touches both (e.g., a Saga that has Cycling). That's intentional:
// the parser owes a correct read on every mechanic the card uses,
// so each gets credit/blame independently.
//
// Output is sorted by Uncovered DESC then by Name ASC for stable
// tiebreaks. Mechanics with Total == 0 (no cards in the corpus use
// them) are dropped so the report doesn't list dead matchers.
func groupByMechanic(results []result) []MechanicCoverage {
	out := make([]MechanicCoverage, 0, len(mechanicDefs))
	for _, def := range mechanicDefs {
		mc := MechanicCoverage{Name: def.name}
		for _, r := range results {
			if !def.match(r) {
				continue
			}
			mc.Total++
			switch r.Class {
			case classMissing:
				mc.Missing++
				mc.Uncovered++
			case classEmptyAST:
				mc.EmptyAST++
				mc.Uncovered++
			case classPartial:
				mc.Partial++
				mc.Uncovered++
			case classOK:
				mc.OK++
			case classOKVanilla:
				mc.OKVanilla++
			}
		}
		if mc.Total > 0 {
			out = append(out, mc)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Uncovered != out[j].Uncovered {
			return out[i].Uncovered > out[j].Uncovered
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// writeByMechanicReport renders the markdown report. Same shape as
// the by-set / by-type reports for consistency. No top-N flag —
// the mechanic count is curated and bounded, so emitting the full
// table is cheap.
func writeByMechanicReport(path string, groups []MechanicCoverage) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# Parser Coverage by Mechanic\n\n")
	fmt.Fprintf(f, "Cards grouped by templated GAMEPLAY MECHANIC (Flashback / Adventure /\n")
	fmt.Fprintf(f, "Saga / Mutate / Prototype / Modal DFC / …) rather than by set or by\n")
	fmt.Fprintf(f, "primary type. This is the dimension that maps directly to parser-\n")
	fmt.Fprintf(f, "handler ownership: each row points at a specific scaffold in\n")
	fmt.Fprintf(f, "`scripts/mtg_ast.py` that can be improved.\n\n")
	fmt.Fprintf(f, "A by-set report can hide that \"Throne of Eldraine is 80%% covered\"\n")
	fmt.Fprintf(f, "without saying anything about Adventure handling specifically (Eldraine\n")
	fmt.Fprintf(f, "contained both Adventure cards and non-Adventure cards). This report\n")
	fmt.Fprintf(f, "answers \"of the cards that USE Adventure templating, what fraction\n")
	fmt.Fprintf(f, "are parsing right?\" — which is the question the maintainer asks before\n")
	fmt.Fprintf(f, "opening the parser source.\n\n")

	fmt.Fprintf(f, "## Summary\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(f, "| Mechanics tracked | %d |\n", len(groups))
	var totUnc, totTot int
	for _, g := range groups {
		totUnc += g.Uncovered
		totTot += g.Total
	}
	fmt.Fprintf(f, "| Uncovered (mechanic-bearing) | %d |\n", totUnc)
	fmt.Fprintf(f, "| Total mechanic-bearing cards | %d |\n\n", totTot)
	fmt.Fprintf(f, "_Note: a single card can count toward multiple mechanics, so the\n")
	fmt.Fprintf(f, "Total here is greater than the unique-card count._\n\n")

	fmt.Fprintf(f, "## Mechanics\n\n")
	fmt.Fprintf(f, "| Rank | Mechanic | Total | Uncovered | Coverage | MISSING | EMPTY_AST | PARTIAL |\n")
	fmt.Fprintf(f, "|---:|---|---:|---:|---:|---:|---:|---:|\n")
	for i, g := range groups {
		fmt.Fprintf(f, "| %d | `%s` | %d | %d | %.1f%% | %d | %d | %d |\n",
			i+1, g.Name, g.Total, g.Uncovered, g.CoveragePct(),
			g.Missing, g.EmptyAST, g.Partial)
	}
	fmt.Fprintln(f)

	fmt.Fprintf(f, "## Reading this report\n\n")
	fmt.Fprintf(f, "- **Low Coverage%% on a high-Total mechanic** is the strongest\n")
	fmt.Fprintf(f, "  signal: it means a well-used template is failing parser-wide.\n")
	fmt.Fprintf(f, "  Fixing the corresponding handler unlocks the most cards per\n")
	fmt.Fprintf(f, "  change.\n")
	fmt.Fprintf(f, "- **Low Coverage%% on a low-Total mechanic** still points at a\n")
	fmt.Fprintf(f, "  systemic gap, but the leverage is smaller — schedule after the\n")
	fmt.Fprintf(f, "  high-Total ones.\n")
	fmt.Fprintf(f, "- **EMPTY_AST dominates the failure mix** → the parser dispatcher\n")
	fmt.Fprintf(f, "  is seeing the card but producing no abilities (handler missing\n")
	fmt.Fprintf(f, "  or returning early). PARTIAL means a handler ran but flagged\n")
	fmt.Fprintf(f, "  errors. MISSING means the parser pipeline never ingested the\n")
	fmt.Fprintf(f, "  card at all.\n")
	fmt.Fprintf(f, "- **Curated matcher list** — the matchers are intentionally\n")
	fmt.Fprintf(f, "  conservative substring tests. Evergreen keywords (flying,\n")
	fmt.Fprintf(f, "  vigilance, trample) are excluded because they parse cleanly\n")
	fmt.Fprintf(f, "  and would just swamp the report with 99%% rows.\n")

	return nil
}

// formatByMechanicSummary returns a stderr-friendly one-liner listing
// the top N mechanics by Uncovered count. Mirrors formatBySetSummary /
// formatByTypeSummary so main.go can log all three in the same shape.
func formatByMechanicSummary(groups []MechanicCoverage, n int) string {
	if len(groups) == 0 {
		return "by-mechanic: no mechanic-bearing cards uncovered"
	}
	if n <= 0 || n > len(groups) {
		n = len(groups)
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, fmt.Sprintf("%s=%d", groups[i].Name, groups[i].Uncovered))
	}
	return "by-mechanic top: " + strings.Join(parts, ", ")
}
