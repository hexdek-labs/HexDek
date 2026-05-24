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
	"os"
	"sort"
	"strings"

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
	ParseErrors []string
}

func main() {
	oraclePath := flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards JSON")
	astPath := flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL")
	outPath := flag.String("out", "docs/parser-coverage-r41.md", "markdown report output path")
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

	if err := writeReport(*outPath, results, len(entries), corpus.Count(), len(corpus.ParseWarnings)); err != nil {
		log.Fatalf("writeReport: %v", err)
	}
	log.Printf("wrote %s", *outPath)
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
		return result{Name: e.Name, Class: classMissing, OracleText: e.OracleText}
	}
	text := strings.TrimSpace(e.OracleText)
	vanilla := text == "" || isBasicLand(e.TypeLine)
	if !card.FullyParsed || len(card.ParseErrors) > 0 {
		return result{Name: e.Name, Class: classPartial, OracleText: text, ParseErrors: card.ParseErrors}
	}
	if len(card.Abilities) == 0 {
		if vanilla {
			return result{Name: e.Name, Class: classOKVanilla, OracleText: text}
		}
		return result{Name: e.Name, Class: classEmptyAST, OracleText: text}
	}
	return result{Name: e.Name, Class: classOK, OracleText: text}
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

func writeReport(path string, results []result, total, corpusCount, corpusWarnings int) error {
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
