

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
)

func main() {
	corpus, _ := astload.Load("data/rules/ast_dataset.jsonl")

	f, _ := os.Open("data/rules/oracle-cards.json")
	var entries []struct {
		Name       string   `json:"name"`
		TypeLine   string   `json:"type_line"`
		OracleText string   `json:"oracle_text"`
		SetName    string   `json:"set_name"`
		CardFaces  []struct {
			OracleText string `json:"oracle_text"`
		} `json:"card_faces"`
	}
	json.NewDecoder(f).Decode(&entries)
	f.Close()

	unSets := map[string]bool{"Unstable": true, "Unhinged": true, "Unglued": true, "Unsanctioned": true, "Unfinity": true}

	// Tally missing keywords by category
	type driftEntry struct {
		name    string
		missing []string
		oracle  string
		astKW   []string
	}
	var drifts []driftEntry
	tally := map[string]int{}

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
				tally[kw]++
			}
		}

		if len(missing) > 0 {
			drifts = append(drifts, driftEntry{e.Name, missing, oracleText, astKW})
		}
	}

	// Print by keyword category
	type kwGroup struct {
		kw    string
		count int
	}
	var groups []kwGroup
	for kw, c := range tally {
		groups = append(groups, kwGroup{kw, c})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].count > groups[j].count })

	for _, g := range groups {
		fmt.Printf("\n=== %s (%d cards) ===\n", g.kw, g.count)
		shown := 0
		for _, d := range drifts {
			for _, m := range d.missing {
				if m == g.kw && shown < 3 {
					// Show first line of oracle text containing the keyword
					lines := strings.Split(d.oracle, "\n")
					for _, line := range lines {
						if strings.Contains(strings.ToLower(line), g.kw) {
							fmt.Printf("  %s: %s\n", d.name, strings.TrimSpace(line))
							break
						}
					}
					shown++
				}
			}
		}
	}
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
