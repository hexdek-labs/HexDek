// Command dump_drift reports keyword-parser drift: oracle-text keywords that
// the Scryfall corpus emits but Thor's gameast.Keyword extractor never landed
// on. The output is a triage list for the parser-coverage backlog.
//
// Three breakdowns are produced:
//
//   1. Per-keyword tally (legacy view): one section per missing keyword,
//      with up to three example cards per keyword. Tells the operator
//      WHICH keyword the parser is bleeding on.
//
//   2. Per-era tally (r60 addition): groups missing-keyword counts by
//      the same era buckets Thor's corpus_audit uses (era 1-4 by mechanic
//      heuristic). Tells the operator WHEN the bleed is — eras with high
//      gap counts are candidates for the next scaffold sweep.
//
//   3. Per-card-type tally (r60 addition): groups missing-keyword counts
//      by primary card type (creature, instant, sorcery, enchantment,
//      artifact, land, planeswalker). Tells the operator WHAT the bleed
//      is on — creature gaps usually mean combat keywords; instant/sorcery
//      gaps usually mean alt-cost or storm-family mechanics.
//
// The analysis loop is split out of main into Analyze so cmd/dump_drift can
// run against synthetic corpora in unit tests without touching disk.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
)

const (
	astPath    = "data/rules/ast_dataset.jsonl"
	oraclePath = "data/rules/oracle-cards.json"
)

// OracleEntry is the slice of Scryfall data Analyze cares about. Defined
// at package scope (rather than inline in main) so tests can construct
// hand-built corpora without depending on json decode internals.
type OracleEntry struct {
	Name       string   `json:"name"`
	TypeLine   string   `json:"type_line"`
	OracleText string   `json:"oracle_text"`
	SetName    string   `json:"set_name"`
	CardFaces  []struct {
		OracleText string `json:"oracle_text"`
	} `json:"card_faces"`
}

// ASTLookup abstracts the corpus side of Analyze so tests can pass a
// hand-built map without instantiating an astload.Corpus.
type ASTLookup interface {
	Get(name string) (*gameast.CardAST, bool)
}

// DriftEntry captures one card whose oracle text contained at least one
// keyword the AST missed. The exported shape lets tests assert on the
// triage list contents.
type DriftEntry struct {
	Name     string
	TypeLine string
	Era      string
	Missing  []string
	Oracle   string
	ASTKW    []string
}

// Report bundles the three drift breakdowns. Each tally maps a bucket
// label to the number of MISSING-KEYWORD events that fell in that bucket
// (one card with N missing keywords contributes N events, not 1).
type Report struct {
	Entries      []DriftEntry
	PerKeyword   map[string]int
	PerEra       map[string]int
	PerCardType  map[string]int
	CardsScanned int
}

// Analyze walks the oracle entries, looks each one up in the AST corpus,
// and reports keywords the parser dropped. unSets is the {set name → true}
// filter for sets that should be excluded (UN-sets, joke sets). Pure
// function — no IO, no globals.
func Analyze(entries []OracleEntry, corpus ASTLookup, unSets map[string]bool) Report {
	r := Report{
		PerKeyword:  map[string]int{},
		PerEra:      map[string]int{},
		PerCardType: map[string]int{},
	}
	seen := map[string]bool{}

	for _, e := range entries {
		if e.Name == "" || seen[e.Name] || unSets[e.SetName] {
			continue
		}
		seen[e.Name] = true

		ast, ok := corpus.Get(e.Name)
		if !ok {
			continue
		}
		r.CardsScanned++

		oracleText := e.OracleText
		if oracleText == "" && len(e.CardFaces) > 0 {
			oracleText = e.CardFaces[0].OracleText
		}

		oracleKW := extractOracleKW(oracleText)
		astKW := extractASTKW(ast)
		astSet := toSet(astKW)

		var missing []string
		for _, kw := range oracleKW {
			if !astSet[kw] && !hasPrefixMatch(kw, astKW) {
				missing = append(missing, kw)
			}
		}
		if len(missing) == 0 {
			continue
		}

		era := classifyEra(oracleText, e.TypeLine)
		ct := primaryCardType(e.TypeLine)

		for _, m := range missing {
			r.PerKeyword[m]++
			r.PerEra[era]++
			r.PerCardType[ct]++
		}

		r.Entries = append(r.Entries, DriftEntry{
			Name:     e.Name,
			TypeLine: e.TypeLine,
			Era:      era,
			Missing:  missing,
			Oracle:   oracleText,
			ASTKW:    astKW,
		})
	}
	return r
}

// PrintReport writes the three drift sections to w (typically os.Stdout).
// Sections appear in operator-priority order: per-keyword (deepest signal,
// per-era (where to focus the next scaffold sweep), per-card-type (what
// shape of mechanic). Extracted so tests can assert on stable text output
// when they care; most tests assert on the Report directly.
func PrintReport(w *os.File, r Report) {
	type kvPair struct {
		k string
		v int
	}
	sorted := func(m map[string]int) []kvPair {
		out := make([]kvPair, 0, len(m))
		for k, v := range m {
			out = append(out, kvPair{k, v})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].v != out[j].v {
				return out[i].v > out[j].v
			}
			return out[i].k < out[j].k
		})
		return out
	}

	fmt.Fprintf(w, "# Parser-drift report\n\n")
	fmt.Fprintf(w, "- Cards scanned (oracle ∩ AST): **%d**\n", r.CardsScanned)
	fmt.Fprintf(w, "- Cards with at least one missing keyword: **%d**\n", len(r.Entries))
	fmt.Fprintf(w, "- Total missing-keyword events: **%d**\n", sumValues(r.PerKeyword))

	for _, g := range sorted(r.PerKeyword) {
		fmt.Fprintf(w, "\n=== %s (%d cards) ===\n", g.k, g.v)
		shown := 0
		for _, d := range r.Entries {
			for _, m := range d.Missing {
				if m == g.k && shown < 3 {
					lines := strings.Split(d.Oracle, "\n")
					for _, line := range lines {
						if strings.Contains(strings.ToLower(line), g.k) {
							fmt.Fprintf(w, "  %s: %s\n", d.Name, strings.TrimSpace(line))
							break
						}
					}
					shown++
				}
			}
		}
	}

	fmt.Fprintf(w, "\n## Per-era drift\n")
	for _, p := range sorted(r.PerEra) {
		fmt.Fprintf(w, "- `%s` × %d\n", p.k, p.v)
	}

	fmt.Fprintf(w, "\n## Per-card-type drift\n")
	for _, p := range sorted(r.PerCardType) {
		fmt.Fprintf(w, "- `%s` × %d\n", p.k, p.v)
	}
}

func sumValues(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func main() {
	corpus, err := astload.Load(astPath)
	if err != nil {
		log.Fatalf("load AST corpus %s: %v", astPath, err)
	}

	f, err := os.Open(oraclePath)
	if err != nil {
		log.Fatalf("open oracle corpus %s: %v", oraclePath, err)
	}
	defer f.Close()

	var entries []OracleEntry
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		log.Fatalf("decode oracle corpus %s: %v", oraclePath, err)
	}

	unSets := map[string]bool{
		"Unstable":     true,
		"Unhinged":     true,
		"Unglued":      true,
		"Unsanctioned": true,
		"Unfinity":     true,
	}

	report := Analyze(entries, corpus, unSets)
	PrintReport(os.Stdout, report)
}

// classifyEra mirrors cmd/hexdek-thor/corpus_audit.go::classifyCardEra so
// dump_drift can label cards without importing main from another binary.
// Keep the keyword lists in sync — if Thor's classifier picks up a new
// era marker, add it here too.
func classifyEra(oracleText, typeLine string) string {
	text := strings.ToLower(oracleText)
	types := strings.ToLower(typeLine)
	for _, kw := range []string{"discover", "descend", "battle", "prototype", "craft",
		"role token", "finality counter", "the ring"} {
		if strings.Contains(text, kw) || strings.Contains(types, kw) {
			return "era4"
		}
	}
	for _, kw := range []string{"daybound", "nightbound", "disturb", "cleave", "decayed",
		"exploit", "companion", "mutate", "foretell", "learn", "ward", "perpetual", "conjure"} {
		if strings.Contains(text, kw) {
			return "era3"
		}
	}
	for _, kw := range []string{"partner", "experience counter", "eminence", "energy counter",
		"crew", "adapt", "amass", "afterlife", "spectacle", "riot"} {
		if strings.Contains(text, kw) {
			return "era2"
		}
	}
	return "era1"
}

// primaryCardType extracts the dominant card type from a Scryfall type
// line. The line shape is "Supertype Type — Subtype …" (e.g. "Legendary
// Creature — Human Wizard"); we look for the first type keyword AFTER any
// supertype prefix. Multi-type cards (e.g. "Artifact Creature") return
// the most-mechanically-distinctive type — creature beats artifact
// because keyword coverage gaps cluster heavily in combat keywords.
func primaryCardType(typeLine string) string {
	t := strings.ToLower(typeLine)
	// Strip subtype tail.
	if i := strings.Index(t, "—"); i >= 0 {
		t = t[:i]
	}
	// Order matters: creature wins over artifact/enchantment (the dual-
	// type forms always still need creature-keyword coverage). Land sits
	// last because most land "abilities" aren't oracle-keyword-shaped.
	for _, ct := range []string{"creature", "planeswalker", "instant", "sorcery",
		"enchantment", "artifact", "land"} {
		if strings.Contains(t, ct) {
			return ct
		}
	}
	return "other"
}

var reReminder = regexp.MustCompile(`\([^)]*\)`)
var reGrants = regexp.MustCompile(`(?i)\b(?:gains?|has|have|gets?|with)\s+`)

var knownKW = []string{
	"flying", "trample", "first strike", "double strike", "deathtouch",
	"defender", "flash", "haste", "hexproof", "indestructible",
	"lifelink", "menace", "reach", "shroud", "vigilance",
	"ward", "protection", "intimidate", "fear", "shadow", "skulk",
	"flanking", "banding", "horsemanship", "phasing", "rampage",
	"cycling", "equip", "flashback", "kicker", "multikicker",
	"cascade", "convoke", "delve", "devoid", "emerge", "escape",
	"exploit", "fabricate", "madness", "morph", "megamorph",
	"mutate", "ninjutsu", "overload", "persist", "prowess",
	"rebound", "scavenge", "storm", "suspend", "undying", "unearth",
	"affinity", "annihilator", "bestow", "changeling", "cipher",
	"crew", "cumulative upkeep", "dash", "dredge", "embalm",
	"enchant", "entwine", "epic", "eternalize", "evoke", "exalted",
	"extort", "forecast", "fortify", "frenzy", "graft", "gravestorm",
	"hideaway", "infect", "living weapon", "miracle", "modular",
	"myriad", "offering", "outlast", "partner", "poisonous",
	"populate", "prowl", "recover", "reinforce", "renown", "replicate",
	"retrace", "ripple", "soulbond", "soulshift", "splice", "split second",
	"sunburst", "surge", "totem armor", "transfigure", "transmute", "tribute",
	"undaunted", "vanishing", "wither",
	"adapt", "afterlife", "amass", "companion", "craft",
	"demonstrate", "disturb", "foretell", "learn", "lesson",
	"cleave", "connive", "blitz", "casualty", "ravenous",
	"read ahead", "reconfigure", "squad", "toxic", "backup",
	"bargain", "celebrate", "descend", "discover",
	"for mirrodin!", "incubate", "living metal", "map",
	"prototype", "role", "saddle", "collect evidence",
	"cloak", "disguise", "plot", "spree", "offspring",
	"impending", "manifest dread",
	"swampwalk", "islandwalk", "forestwalk", "mountainwalk", "plainswalk",
	"daybound", "nightbound", "bushido", "buyback", "encore", "ascend",
	"level up", "aftermath", "warp", "station", "sneak",
	"choose a background", "start your engines!",
	"modular", "reinforce", "tribute", "devour", "bloodthirst",
	"absorb", "amplify", "fading", "haunt", "enlist",
	"boast", "channel", "champion", "conspire",
	"domain", "dethrone", "evolve", "explore",
}

func extractOracleKW(oracleText string) []string {
	clean := reReminder.ReplaceAllString(oracleText, "")
	found := map[string]bool{}
	lines := strings.Split(strings.ToLower(clean), "\n")
	for _, kw := range knownKW {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			parts := strings.Split(trimmed, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == kw || strings.HasPrefix(part, kw+" ") || strings.HasPrefix(part, kw+"—") {
					kwIdx := strings.Index(trimmed, kw)
					if kwIdx > 0 {
						before := trimmed[:kwIdx]
						if reGrants.MatchString(before) {
							continue
						}
					}
					found[kw] = true
				}
			}
		}
	}
	var r []string
	for kw := range found {
		r = append(r, kw)
	}
	sort.Strings(r)
	return r
}

func extractASTKW(ast *gameast.CardAST) []string {
	seen := map[string]bool{}
	for _, ab := range ast.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok {
			seen[strings.ToLower(kw.Name)] = true
		}
	}
	var r []string
	for kw := range seen {
		r = append(r, kw)
	}
	sort.Strings(r)
	return r
}

func toSet(ss []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func hasPrefixMatch(oracle string, astKWs []string) bool {
	for _, ak := range astKWs {
		if ak == oracle || strings.HasPrefix(ak, oracle+" ") {
			return true
		}
	}
	return false
}
