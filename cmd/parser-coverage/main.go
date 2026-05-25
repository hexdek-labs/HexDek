// parser-coverage — Audits AST parser coverage against the Scryfall oracle
// corpus.
//
// For every card in data/rules/oracle-cards.json, this tool checks the AST
// produced by the Python parser (loaded from data/rules/ast_dataset.jsonl via
// internal/astload) and classifies the result:
//
//	OK                — corpus entry present, abilities non-empty, no parse errors
//	OK_VANILLA        — corpus entry present, no oracle text (basic land or vanilla)
//	MISSING           — oracle card has no corpus entry at all
//	EMPTY_AST         — corpus entry present, oracle text non-trivial, but 0 abilities
//	PARTIAL           — corpus entry has fully_parsed=false or non-empty parse_errors
//
// Only EMPTY_AST and PARTIAL count against the parser; MISSING is reported
// separately because it reflects the parser pipeline's set, not its parsing.
//
// Output is a markdown report written to --out (default
// docs/parser-coverage-r41.md).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
)

type oracleEntry struct {
	Name       string   `json:"name"`
	Layout     string   `json:"layout"`
	TypeLine   string   `json:"type_line"`
	SetName    string   `json:"set_name"`
	OracleText string   `json:"oracle_text"`
	CardFaces  []struct {
		OracleText string `json:"oracle_text"`
	} `json:"card_faces"`
}

type classification int

const (
	classOK classification = iota
	classOKVanilla
	classMissing
	classEmptyAST
	classPartial
)

func (c classification) String() string {
	switch c {
	case classOK:
		return "OK"
	case classOKVanilla:
		return "OK_VANILLA"
	case classMissing:
		return "MISSING"
	case classEmptyAST:
		return "EMPTY_AST"
	case classPartial:
		return "PARTIAL"
	}
	return "?"
}

type result struct {
	Name        string
	Class       classification
	OracleText  string
	TypeLine    string
	SetName     string
	ParseErrors []string
}

func main() {
	oraclePath := flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards JSON")
	astPath := flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL")
	outPath := flag.String("out", "docs/parser-coverage-r41.md", "markdown report output path")
	sampleSize := flag.Int("sample-size", 20, "number of uncovered cards to randomly sample for the report (0 = disabled)")
	sampleSeed := flag.Int64("sample-seed", 42, "RNG seed for the uncovered-card sample (same seed → same sample, for reproducible reports)")
	sampleClasses := flag.String("sample-classes", "missing,empty_ast,partial", "comma-separated subset of {missing,empty_ast,partial} to draw the sample from")
	actionList := flag.String("action-list", "", "card name to generate a scaffold/handler TODO checklist for; when set, the tool prints the action list and skips the coverage report")
	actionListOut := flag.String("action-list-out", "", "optional path to write the action-list markdown (defaults to stdout)")
	csvExport := flag.String("csv-export", "", "optional path to write a per-card CSV export for spreadsheet analysis; runs additively alongside the markdown report")
	csvIncludeOK := flag.Bool("csv-include-ok", false, "include OK and OK_VANILLA rows in the CSV export (default: uncovered only)")
	historyPath := flag.String("history", "", "optional JSONL file to append this run's stats to; if file exists, prints a delta-vs-previous summary")
	historyLabel := flag.String("history-label", "", "optional label for this history entry (e.g., 'r60', '2026-05-24'); shown in future delta summaries")
	bySetPath := flag.String("by-set", "", "optional path to write a markdown report grouping uncovered cards by Magic set, ranked by uncovered count")
	bySetTopN := flag.Int("by-set-top", 0, "limit the --by-set report to the top N sets by uncovered count (0 = include every set)")
	flag.Parse()

	log.Printf("loading AST corpus from %s ...", *astPath)
	corpus, err := astload.Load(*astPath)
	if err != nil {
		log.Fatalf("astload: %v", err)
	}
	log.Printf("  loaded %d cards in %s (%d parse warnings)", corpus.Count(), corpus.LoadDuration, len(corpus.ParseWarnings))

	log.Printf("loading oracle corpus from %s ...", *oraclePath)
	entries, err := loadOracle(*oraclePath)
	if err != nil {
		log.Fatalf("loadOracle: %v", err)
	}
	log.Printf("  loaded %d unique oracle entries", len(entries))

	results := make([]result, 0, len(entries))
	for _, e := range entries {
		results = append(results, classify(e, corpus))
	}

	if strings.TrimSpace(*actionList) != "" {
		r, ok := findResultByName(results, *actionList)
		if !ok {
			log.Fatalf("--action-list: card %q not found in oracle corpus", *actionList)
		}
		actions := generateActionList(r, r.TypeLine)
		md := renderActionList(r.Name, r.TypeLine, r, actions)
		if *actionListOut == "" {
			fmt.Print(md)
		} else {
			if err := os.WriteFile(*actionListOut, []byte(md), 0o644); err != nil {
				log.Fatalf("write action list: %v", err)
			}
			log.Printf("wrote %s", *actionListOut)
		}
		return
	}

	classFilter, err := parseSampleClasses(*sampleClasses)
	if err != nil {
		log.Fatalf("--sample-classes: %v", err)
	}
	sample := sampleUncovered(results, classFilter, *sampleSize, *sampleSeed)

	if err := writeReport(*outPath, results, len(entries), corpus.Count(), len(corpus.ParseWarnings), sample, *sampleSeed); err != nil {
		log.Fatalf("writeReport: %v", err)
	}
	log.Printf("wrote %s", *outPath)

	if strings.TrimSpace(*csvExport) != "" {
		if err := writeCSV(*csvExport, results, *csvIncludeOK); err != nil {
			log.Fatalf("writeCSV: %v", err)
		}
		log.Printf("wrote %s (csv export, include_ok=%v)", *csvExport, *csvIncludeOK)
	}

	if strings.TrimSpace(*bySetPath) != "" {
		groups := groupBySet(results)
		if err := writeBySetReport(*bySetPath, groups, *bySetTopN); err != nil {
			log.Fatalf("writeBySetReport: %v", err)
		}
		shown := len(groups)
		if *bySetTopN > 0 && *bySetTopN < shown {
			shown = *bySetTopN
		}
		log.Printf("wrote %s (by-set report, %d sets shown of %d)", *bySetPath, shown, len(groups))
		log.Printf("  %s", formatBySetSummary(groups, 5))
	}

	if strings.TrimSpace(*historyPath) != "" {
		entry := resultsToHistoryEntry(results, len(entries), corpus.Count(), len(corpus.ParseWarnings), *historyLabel, time.Now())
		prev, err := readHistory(*historyPath)
		if err != nil {
			log.Fatalf("readHistory: %v", err)
		}
		if len(prev) > 0 {
			delta := computeDelta(prev[len(prev)-1], entry)
			fmt.Fprint(os.Stderr, renderHistoryDelta(delta))
		} else {
			log.Printf("history: no previous entries in %s — recording baseline", *historyPath)
		}
		if err := appendHistory(*historyPath, entry); err != nil {
			log.Fatalf("appendHistory: %v", err)
		}
		log.Printf("appended history entry to %s (label=%q)", *historyPath, *historyLabel)
	}
}

// parseSampleClasses maps the comma-separated CLI list to a set of
// classifications. Names are case-insensitive; whitespace is trimmed.
// Empty / "none" disables sampling. Unknown names are an error so a
// typo doesn't silently produce an empty sample.
func parseSampleClasses(s string) (map[classification]bool, error) {
	out := map[classification]bool{}
	if strings.TrimSpace(s) == "" || strings.EqualFold(strings.TrimSpace(s), "none") {
		return out, nil
	}
	for _, raw := range strings.Split(s, ",") {
		name := strings.ToLower(strings.TrimSpace(raw))
		switch name {
		case "":
			continue
		case "missing":
			out[classMissing] = true
		case "empty_ast", "emptyast":
			out[classEmptyAST] = true
		case "partial":
			out[classPartial] = true
		case "ok", "ok_vanilla":
			return nil, fmt.Errorf("class %q is not an uncovered class (only missing/empty_ast/partial are sample-eligible)", name)
		default:
			return nil, fmt.Errorf("unknown class %q (valid: missing, empty_ast, partial)", name)
		}
	}
	return out, nil
}

// sampleUncovered returns up to n random results drawn from those whose
// Class is in classFilter. The selection is deterministic for a fixed
// seed AND a fixed `results` order — call sites must keep `results` in
// the iteration order produced by loadOracle so reports diff cleanly
// across runs.
//
// Uses reservoir sampling so we don't materialize the full filtered
// slice (the dataset is ~40k cards; the filtered subset is several
// thousand). The output is sorted by name at the end for stable
// markdown rendering.
func sampleUncovered(results []result, classFilter map[classification]bool, n int, seed int64) []result {
	if n <= 0 || len(classFilter) == 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(seed))
	out := make([]result, 0, n)
	seen := 0
	for _, r := range results {
		if !classFilter[r.Class] {
			continue
		}
		seen++
		if len(out) < n {
			out = append(out, r)
			continue
		}
		// Reservoir step: keep with probability n/seen.
		j := rng.Intn(seen)
		if j < n {
			out[j] = r
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// findResultByName looks up a result by card name. Case-sensitive
// first pass (so exact Scryfall names hit immediately), then a
// case-insensitive fallback so the CLI accepts lowercased input.
func findResultByName(results []result, name string) (result, bool) {
	for _, r := range results {
		if r.Name == name {
			return r, true
		}
	}
	low := strings.ToLower(name)
	for _, r := range results {
		if strings.EqualFold(r.Name, low) {
			return r, true
		}
	}
	return result{}, false
}

func loadOracle(path string) ([]oracleEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var raw []oracleEntry
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, err
	}
	unSets := map[string]bool{
		"Unstable": true, "Unhinged": true, "Unglued": true,
		"Unsanctioned": true, "Unfinity": true,
	}
	out := make([]oracleEntry, 0, len(raw))
	seen := map[string]bool{}
	for _, e := range raw {
		if e.Name == "" || seen[e.Name] || unSets[e.SetName] || e.Layout == "token" {
			continue
		}
		seen[e.Name] = true
		if strings.TrimSpace(e.OracleText) == "" && len(e.CardFaces) > 0 {
			e.OracleText = e.CardFaces[0].OracleText
		}
		out = append(out, e)
	}
	return out, nil
}

func classify(e oracleEntry, corpus *astload.Corpus) result {
	card, ok := corpus.Get(e.Name)
	if !ok {
		return result{Name: e.Name, Class: classMissing, OracleText: e.OracleText, TypeLine: e.TypeLine, SetName: e.SetName}
	}
	text := strings.TrimSpace(e.OracleText)
	vanilla := text == "" || isBasicLand(e.TypeLine)
	if !card.FullyParsed || len(card.ParseErrors) > 0 {
		return result{Name: e.Name, Class: classPartial, OracleText: text, TypeLine: e.TypeLine, SetName: e.SetName, ParseErrors: card.ParseErrors}
	}
	if len(card.Abilities) == 0 {
		if vanilla {
			return result{Name: e.Name, Class: classOKVanilla, OracleText: text, TypeLine: e.TypeLine, SetName: e.SetName}
		}
		return result{Name: e.Name, Class: classEmptyAST, OracleText: text, TypeLine: e.TypeLine, SetName: e.SetName}
	}
	return result{Name: e.Name, Class: classOK, OracleText: text, TypeLine: e.TypeLine, SetName: e.SetName}
}

func isBasicLand(typeLine string) bool {
	tl := strings.ToLower(typeLine)
	return strings.Contains(tl, "basic") && strings.Contains(tl, "land")
}

// failurePatterns groups EMPTY_AST + MISSING + PARTIAL cards by an
// oracle-text fingerprint so the report can surface the dominant classes
// of failure.
//
// Patterns are checked in order; first match wins.
var failurePatterns = []struct {
	name  string
	match func(text, typeLine string) bool
}{
	{"double_faced_or_meld", func(t, tl string) bool {
		return strings.Contains(t, "//") || strings.Contains(strings.ToLower(t), "transform") || strings.Contains(strings.ToLower(t), "melds with")
	}},
	{"adventure", func(t, tl string) bool {
		return strings.Contains(t, "Adventure") || strings.Contains(t, "—") && strings.Contains(strings.ToLower(t), "exile this card")
	}},
	{"saga_or_class", func(t, tl string) bool {
		return strings.Contains(t, "Saga") || strings.Contains(t, "Class") || strings.Contains(t, "I —") || strings.Contains(t, "II —")
	}},
	{"battle", func(t, tl string) bool {
		return strings.Contains(strings.ToLower(tl), "battle")
	}},
	{"emblem_or_planeswalker", func(t, tl string) bool {
		return strings.Contains(strings.ToLower(tl), "planeswalker") || strings.Contains(strings.ToLower(tl), "emblem")
	}},
	{"prototype_or_mutate", func(t, tl string) bool {
		l := strings.ToLower(t)
		return strings.Contains(l, "prototype") || strings.Contains(l, "mutate")
	}},
	{"choose_one_modal", func(t, tl string) bool {
		l := strings.ToLower(t)
		return strings.Contains(l, "choose one") || strings.Contains(l, "choose two") || strings.Contains(l, "choose up to")
	}},
	{"replacement_effect", func(t, tl string) bool {
		l := strings.ToLower(t)
		return strings.Contains(l, "if you would") || strings.Contains(l, "instead")
	}},
	{"keyword_only", func(t, tl string) bool {
		// Single-line keyword stew like "Flying, vigilance, lifelink"
		if strings.Count(t, "\n") > 0 {
			return false
		}
		return len(t) > 0 && len(t) < 60
	}},
	{"long_text", func(t, tl string) bool { return len(t) >= 200 }},
}

func classifyPattern(r result) string {
	tl := ""
	for _, p := range failurePatterns {
		if p.match(r.OracleText, tl) {
			return p.name
		}
	}
	return "other"
}

func writeReport(path string, results []result, total, corpusCount, corpusWarnings int, sample []result, sampleSeed int64) error {
	var counts [5]int
	for _, r := range results {
		counts[r.Class]++
	}

	// Bucket non-OK results by oracle-text pattern.
	type bucketEntry struct {
		Class      classification
		Name       string
		OracleText string
	}
	buckets := map[string][]bucketEntry{}
	for _, r := range results {
		if r.Class == classOK || r.Class == classOKVanilla {
			continue
		}
		key := classifyPattern(r)
		buckets[key] = append(buckets[key], bucketEntry{r.Class, r.Name, r.OracleText})
	}

	type bucketStat struct {
		Name    string
		Count   int
		Samples []bucketEntry
	}
	var stats []bucketStat
	for k, v := range buckets {
		samples := v
		if len(samples) > 5 {
			samples = samples[:5]
		}
		stats = append(stats, bucketStat{Name: k, Count: len(v), Samples: samples})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Count > stats[j].Count })
	if len(stats) > 10 {
		stats = stats[:10]
	}

	if err := os.MkdirAll("docs", 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	parseable := counts[classOK] + counts[classOKVanilla]
	successRate := 100.0 * float64(parseable) / float64(total)
	failureRate := 100.0 - successRate

	fmt.Fprintf(f, "# Parser Coverage Audit — R41\n\n")
	fmt.Fprintf(f, "Cross-references every entry in `data/rules/oracle-cards.json` against the\n")
	fmt.Fprintf(f, "AST corpus loaded by `internal/astload` from `data/rules/ast_dataset.jsonl`.\n")
	fmt.Fprintf(f, "The AST is produced by the Python parser (`scripts/mtg_ast.py`); this\n")
	fmt.Fprintf(f, "audit verifies its output is loadable and non-empty for every real card.\n\n")

	fmt.Fprintf(f, "## Headline\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(f, "| Oracle cards examined (post-dedup, non-token, non-Un) | %d |\n", total)
	fmt.Fprintf(f, "| AST corpus size | %d |\n", corpusCount)
	fmt.Fprintf(f, "| Astload parse warnings | %d |\n", corpusWarnings)
	fmt.Fprintf(f, "| Parse success rate (OK + OK_VANILLA) | **%.2f%%** |\n", successRate)
	fmt.Fprintf(f, "| Failure rate (MISSING + EMPTY_AST + PARTIAL) | %.2f%% |\n\n", failureRate)

	fmt.Fprintf(f, "## Classification breakdown\n\n")
	fmt.Fprintf(f, "| Class | Count | Share |\n|---|---:|---:|\n")
	for i := range counts {
		c := classification(i)
		fmt.Fprintf(f, "| %s | %d | %.2f%% |\n", c, counts[i], 100.0*float64(counts[i])/float64(total))
	}
	fmt.Fprintln(f)

	fmt.Fprintf(f, "### What each class means\n\n")
	fmt.Fprintf(f, "- **OK** — card resolves through the astload Corpus and has at least one parsed ability.\n")
	fmt.Fprintf(f, "- **OK_VANILLA** — no oracle text (basic land or vanilla creature). Empty AST is correct.\n")
	fmt.Fprintf(f, "- **MISSING** — oracle card has no entry in the AST dataset. Parser pipeline never ingested it.\n")
	fmt.Fprintf(f, "- **EMPTY_AST** — AST entry exists, card has oracle text, but the parser produced zero abilities. Real parser failure.\n")
	fmt.Fprintf(f, "- **PARTIAL** — `fully_parsed=false` or non-empty `parse_errors`. Parser partially failed.\n\n")

	fmt.Fprintf(f, "## Top 10 failure patterns\n\n")
	if len(stats) == 0 {
		fmt.Fprintf(f, "_No failures detected._\n\n")
	} else {
		fmt.Fprintf(f, "| Pattern | Count | Sample cards |\n|---|---:|---|\n")
		for _, s := range stats {
			names := make([]string, 0, len(s.Samples))
			for _, e := range s.Samples {
				names = append(names, fmt.Sprintf("%s (%s)", e.Name, e.Class))
			}
			fmt.Fprintf(f, "| `%s` | %d | %s |\n", s.Name, s.Count, strings.Join(names, "; "))
		}
		fmt.Fprintln(f)
	}

	if counts[classPartial] > 0 {
		fmt.Fprintf(f, "## PARTIAL parse details\n\n")
		shown := 0
		for _, r := range results {
			if r.Class != classPartial {
				continue
			}
			fmt.Fprintf(f, "- **%s** — parse_errors: %v\n", r.Name, r.ParseErrors)
			shown++
			if shown >= 30 {
				fmt.Fprintf(f, "- ... (%d more)\n", counts[classPartial]-shown)
				break
			}
		}
		fmt.Fprintln(f)
	}

	writeUncoveredSample(f, sample, sampleSeed)

	if corpusWarnings > 0 {
		fmt.Fprintf(f, "## Astload corpus warnings (first 20)\n\n")
		// Corpus warnings are loader-level, not per-card parse errors.
		// We don't have direct access here without re-loading; the summary count is the key signal.
		fmt.Fprintf(f, "_See loader output; %d warnings total._\n\n", corpusWarnings)
	}

	fmt.Fprintf(f, "## Reproducing this report\n\n")
	fmt.Fprintf(f, "```\ngo run ./cmd/parser-coverage --out docs/parser-coverage-r41.md\n```\n")

	return nil
}

// writeUncoveredSample renders the random-sample section. Output is
// stable across runs for a given (--sample-seed, --sample-classes,
// data corpus) tuple — the sample slice arrives pre-sorted by name
// from sampleUncovered. Oracle text is truncated to ~120 chars on a
// single line so the table stays readable in GitHub markdown
// previews; the full text is recoverable from oracle-cards.json by
// name lookup.
func writeUncoveredSample(f *os.File, sample []result, seed int64) {
	if len(sample) == 0 {
		return
	}
	fmt.Fprintf(f, "## Uncovered card sample (random %d, seed=%d)\n\n", len(sample), seed)
	fmt.Fprintf(f, "Random reservoir-sample of cards whose AST is missing, empty, or only partially parsed.\n")
	fmt.Fprintf(f, "Each entry is a concrete scaffold target — pick one, read its oracle text,\n")
	fmt.Fprintf(f, "and either add the missing parser handler or extend the existing one until\n")
	fmt.Fprintf(f, "this card lands in the OK class. Re-running with the same `--sample-seed`\n")
	fmt.Fprintf(f, "yields the same set, so a follow-up audit can confirm a specific card moved.\n\n")
	fmt.Fprintf(f, "| # | Card | Class | Oracle text (truncated) |\n|---:|---|---|---|\n")
	for i, r := range sample {
		fmt.Fprintf(f, "| %d | %s | %s | %s |\n", i+1, escapeCell(r.Name), r.Class, escapeCell(truncateOracle(r.OracleText, 120)))
	}
	fmt.Fprintln(f)
}

// truncateOracle collapses newlines + truncates to maxLen with an
// ellipsis. Markdown tables don't render embedded newlines well; a
// single-line truncated view is the right shape for at-a-glance
// triage. Callers wanting the full text can grep oracle-cards.json by
// name.
func truncateOracle(text string, maxLen int) string {
	t := strings.TrimSpace(text)
	t = strings.ReplaceAll(t, "\n", " ")
	t = strings.ReplaceAll(t, "\r", " ")
	for strings.Contains(t, "  ") {
		t = strings.ReplaceAll(t, "  ", " ")
	}
	if t == "" {
		return "_(no oracle text — MISSING entry)_"
	}
	if len(t) > maxLen {
		// Trim to maxLen-1 (room for ellipsis) without slicing in the
		// middle of a multi-byte rune.
		cut := maxLen - 1
		for cut > 0 && t[cut]&0xc0 == 0x80 {
			cut--
		}
		t = t[:cut] + "…"
	}
	return t
}

// escapeCell escapes the small set of characters that break markdown
// tables: pipes (cell delimiters) and backticks (would unbalance code
// spans across rows). Anything else is fine in a table cell.
func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "`", "'")
	return s
}
