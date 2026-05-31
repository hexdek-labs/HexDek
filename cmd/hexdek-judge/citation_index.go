package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// hexdek-judge — Comprehensive CR citation index.
//
// Prior PRs built the citation map piecemeal: invariantCRCitations
// in loki_replay.go maps engine invariants to CR sub-sections, and
// each probe (mana cost / commander / deck construction / SBA)
// embeds its own rule strings in struct fields. That worked for
// per-probe answers but left two gaps:
//
//   1. No INVERSE map. Asking "which judge probe checks §903.4?"
//      required grep across the package source. A tournament judge
//      who wants to verify their copy of hexdek-judge implements a
//      specific CR sub-section had no way to ask programmatically.
//   2. Cross-references between related CR sub-sections (e.g. §704.5p
//      pair removal + §704.5q counter mechanics + §122 counter
//      bookkeeping) lived only as prose in code comments. A single
//      query against the index couldn't surface "all the rules that
//      together govern counter mechanics."
//
// This file consolidates both. The Index is built from the
// invariantCRCitations map (engine-invariant side) PLUS a per-probe
// rule registration (probe → rules it checks) PLUS a hand-curated
// related-rules table that captures the cross-references. Every CR
// citation appearing anywhere in the package surfaces here with:
//
//   - canonical Description (single source of truth — duplicate
//     citations across the codebase fold to one entry)
//   - SectionTitle (the broader CR section the sub-rule belongs to)
//   - CheckedBy (probes + functions that implement it)
//   - RelatedInvariants (engine invariants whose violation triggers
//     a hit against this rule)
//   - RelatedRules (other CR sub-sections that interact with this
//     one — pair-removal + counter bookkeeping, life loss + game-end
//     resolution, etc.)
//
// Use cases:
//   - `judge --citation-index` dumps the full JSON index for offline
//     analysis. Tournament-judge debug, doc generation, CI gates that
//     verify a specific CR sub-section is implemented before a
//     release ships.
//   - `judge --interactive` gets two new intents: `index <rule>` for
//     direct rule lookup, `coverage <probe>` for the inverse —
//     "what does the mana-cost probe check?"
//
// Sync invariant: the BuildCitationIndex constructor walks ALL the
// existing sources (invariantCRCitations + probe registrations) so a
// new entry anywhere automatically surfaces in the index. The
// hand-curated relatedRules table is the only piece a new probe
// author has to update manually — and TestCitationIndex_AllRulesHaveSectionTitle
// pins it.

// CitationIndexEntry is one CR sub-section with all its cross-references.
type CitationIndexEntry struct {
	Rule              string         `json:"rule"`                          // "704.5a"
	Description       string         `json:"description"`                   // canonical description
	SectionTitle      string         `json:"section_title"`                 // "State-Based Actions"
	CheckedBy         []string       `json:"checked_by,omitempty"`          // ["sba_probe", "interactive_what_sbas"]
	RelatedInvariants []string       `json:"related_invariants,omitempty"`  // ["LifeConsistency", "WinCondition"]
	RelatedRules      []string       `json:"related_rules,omitempty"`       // ["704.5b", "104.2"]
	HistoricalFixes   []ClaudemdFix  `json:"historical_fixes,omitempty"`    // CLAUDE.md Resolved-table rows citing this rule
}

// CitationIndex is the comprehensive cross-referenced view of every
// CR citation in the package.
type CitationIndex struct {
	Entries          map[string]*CitationIndexEntry `json:"entries"`          // keyed by rule slug
	ProbeToRules     map[string][]string            `json:"probe_to_rules"`   // probe name → list of rule slugs
	InvariantToRules map[string][]string            `json:"invariant_to_rules"` // engine invariant → list of rule slugs (mirror of invariantCRCitations)
	UnmappedClaudemd []string                       `json:"unmapped_claudemd,omitempty"` // CLAUDE.md cited rule slugs the judge doesn't know about
	Counts           CitationIndexCounts            `json:"counts"`
}

// CitationIndexCounts is the headline summary.
type CitationIndexCounts struct {
	TotalRules      int `json:"total_rules"`
	TotalProbes     int `json:"total_probes"`
	TotalInvariants int `json:"total_invariants"`
	TotalFixes      int `json:"total_fixes"`      // sum of HistoricalFixes across all entries
	RulesWithFixes  int `json:"rules_with_fixes"` // count of entries with non-empty HistoricalFixes
}

// sectionTitles classifies each CR sub-rule to its broader section.
// First-level grouping by the rule's integer prefix; sub-rule
// granularity comes from the Description field.
var sectionTitles = map[string]string{
	"100": "General Game Concepts",
	"104": "Ending the Game",
	"105": "Life",
	"106": "Mana",
	"109": "Objects",
	"117": "Timing and Priority",
	"119": "Life",
	"122": "Counters",
	"202": "Mana Cost and Color",
	"303": "Auras",
	"305": "Lands",
	"400": "Zones, General",
	"405": "Stack",
	"406": "Exile",
	"500": "Beginning Phase / Turn Structure",
	"506": "Combat Phase",
	"508": "Declare Attackers Step",
	"509": "Declare Blockers Step",
	"603": "Handling Triggered Abilities",
	"608": "Resolving Spells and Abilities",
	"611": "Continuous Effects",
	"613": "Interaction of Continuous Effects",
	"614": "Replacement Effects",
	"702": "Keyword Abilities",
	"704": "State-Based Actions",
	"903": "Commander Format",
}

// classifySection returns the broader CR section title for a rule
// slug. "704.5a" → "State-Based Actions". Unknown rules fall back to
// "Comprehensive Rules".
func classifySection(rule string) string {
	// Strip sub-letters / decimals to get the integer prefix.
	prefix := ""
	for _, r := range rule {
		if r >= '0' && r <= '9' {
			prefix += string(r)
		} else {
			break
		}
	}
	if title, ok := sectionTitles[prefix]; ok {
		return title
	}
	return "Comprehensive Rules"
}

// probeRules registers each judge probe with the CR sub-sections it
// checks. The right side of each pair is the canonical rule slug
// (matches the Rule field in invariantCRCitations / probe code).
//
// Sync invariant: when a new probe lands, append its rule coverage
// here. TestCitationIndex_ProbeToRulesBidirectional pins the
// bidirectional consistency.
var probeRules = map[string][]string{
	"mana_cost_check": {
		"202.2", // CR §202.2 mana symbol grammar
	},
	"commander_check": {
		"903.4",  // color identity vs deck
		"903.5",  // commander format card count
		"903.5a", // legendary creature commander
		"903.5b", // "can be your commander" designation
		"903.6",  // commander format legality
	},
	"deck_construction_check": {
		"903.4", // color identity
		"903.5", // card count (100 / 98+2 partners)
	},
	"sba_probe": {
		"704.5a", // life ≤ 0
		"704.5c", // 10+ poison
		"704.5f", // creature toughness ≤ 0
		"704.5g", // marked damage ≥ toughness
		"704.5h", // planeswalker 0 loyalty
		"704.5i", // legend rule
		"704.5j", // world rule
		"704.5p", // un-paired +1/+1 + -1/-1
		"704.6c", // 21+ commander damage
	},
	"loki_replay_analysis": {
		"700", // replay analyzer surfaces ALL the citations from invariantCRCitations
	},
	"interactive_what_sbas": {
		// Mirrors sba_probe (calls the same detector).
		"704.5a", "704.5c", "704.5f", "704.5g", "704.5h",
		"704.5i", "704.5j", "704.5p", "704.6c",
	},
	"interactive_is_combat_legal": {
		"506.2", "508", "509",
	},
}

// relatedRules captures cross-references between CR sub-sections
// that interact mechanically. The map is symmetric in convention
// (if A lists B, B should list A) — TestCitationIndex_RelatedRulesAreSymmetric
// enforces this.
//
// Edges are hand-curated from CR cross-references; a small but
// meaningful set, focused on the rules a tournament judge would
// actually consult together (life loss + game-end, counter
// pair-removal + counter mechanics, attachment rules + Aura
// targeting).
var relatedRules = map[string][]string{
	// Game-end resolution chain — §104.2 (game ends) consults §704.5a/b/c (loss SBAs).
	"104.2":  {"104.3", "704.5a", "704.5b", "704.5c"},
	"104.3":  {"104.2", "704.5a", "704.5b", "704.5c"},
	"704.5a": {"104.2", "104.3", "119"},
	"704.5b": {"104.2", "104.3"},
	"704.5c": {"104.2", "104.3"},

	// Zone integrity — §400.6 + §109.3 + §400.7 form the object-identity chain;
	// §704.5d/e are the ephemeral-cleanup mirror.
	"400.6":  {"109.3", "400.7", "704.5d", "704.5e"},
	"109.3":  {"400.6", "400.7"},
	"400.7":  {"400.6", "109.3", "406.6"},
	"704.5d": {"400.6", "704.5e"},
	"704.5e": {"400.6", "704.5d"},
	"406.6":  {"400.7"},

	// Counter mechanics — §122 + §704.5p (pair removal trigger) + §704.5q (mechanics).
	"122":    {"122.1", "704.5p", "704.5q", "105", "106"},
	"122.1":  {"122", "704.5p", "704.5q"},
	"704.5p": {"704.5q", "122", "122.1"},
	"704.5q": {"704.5p", "122", "122.1"},

	// Attachment legality — §704.5k/m/n attachment SBAs + §303.4 Aura legality.
	"704.5k": {"704.5m", "704.5n", "303.4"},
	"704.5m": {"704.5k", "704.5n", "303.4"},
	"704.5n": {"704.5k", "704.5m", "303.4"},
	"303.4":  {"704.5k", "704.5m", "704.5n"},

	// Triggered abilities — §603.2 trigger event + §603.3 stack placement + §608 resolution.
	"603.2": {"603.3", "608"},
	"603.3": {"603.2", "608", "608.2"},
	"608":   {"603.3", "608.2", "405", "603.2", "117"},
	"608.2": {"608", "603.3"},
	"405":   {"117", "608"},
	"117":   {"405", "608"},

	// Commander format — §903.4 (color identity) + §903.5 (singleton + count) +
	// §903.5a/b (commander designation) + §903.6 (format legality).
	"903.4":  {"903.5", "903.5a", "903.5b", "903.6"},
	"903.5":  {"903.4", "903.5a", "903.5b", "903.6"},
	"903.5a": {"903.4", "903.5", "903.5b", "903.6"},
	"903.5b": {"903.4", "903.5", "903.5a", "903.6"},
	"903.6":  {"903.4", "903.5", "903.5a", "903.5b"},

	// Replacement effects — §614.1 (apply replacements) + §614.6 (source presence).
	"614.1": {"614.6"},
	"614.6": {"614.1"},

	// Combat — §506.2 (combat structure) + §508 (attackers) + §509 (blockers).
	"506.2": {"508", "509"},
	"508":   {"506.2", "509"},
	"509":   {"506.2", "508"},

	// Turn structure — §500.1 / §500.4.
	"500.1": {"500.4"},
	"500.4": {"500.1"},

	// Resource conservation umbrella — §105 (life) + §106 (mana) + §122 (counters).
	"105": {"106", "122", "119"},
	"106": {"105", "122"},
	"119": {"105", "704.5a"},

	// Continuous-effect duration chain — §611 (continuous effects) + §613 (layers) + §702 (keyword grants).
	"611": {"613", "702"},
	"613": {"611"},
	"702": {"611", "702.12"},
	"702.12": {"702"},
}

// BuildCitationIndex constructs the consolidated cross-referenced
// index from invariantCRCitations + probeRules + relatedRules.
//
// Construction algorithm:
//   1. Walk invariantCRCitations — every (invariant, [citations])
//      pair contributes one entry per citation with the invariant
//      in RelatedInvariants and the canonical Description from the
//      citation. Duplicate rule slugs across invariants merge
//      (description from the first mention wins; the others are
//      consistent by code review).
//   2. Walk probeRules — add the probe name to each rule's CheckedBy.
//   3. Apply relatedRules — populate RelatedRules from the hand-
//      curated map.
//   4. Set SectionTitle from classifySection.
//   5. Sort all slice fields for deterministic JSON output.
func BuildCitationIndex() *CitationIndex {
	idx := &CitationIndex{
		Entries:          map[string]*CitationIndexEntry{},
		ProbeToRules:     map[string][]string{},
		InvariantToRules: map[string][]string{},
	}

	// Step 1: walk invariant → citation map.
	for invName, cites := range invariantCRCitations {
		for _, c := range cites {
			e := idx.Entries[c.Rule]
			if e == nil {
				e = &CitationIndexEntry{
					Rule:        c.Rule,
					Description: c.Description,
				}
				idx.Entries[c.Rule] = e
			}
			// Track the invariant linkage.
			if !contains(e.RelatedInvariants, invName) {
				e.RelatedInvariants = append(e.RelatedInvariants, invName)
			}
			if !contains(idx.InvariantToRules[invName], c.Rule) {
				idx.InvariantToRules[invName] = append(idx.InvariantToRules[invName], c.Rule)
			}
		}
	}

	// Step 2: walk probe → rule map.
	for probe, rules := range probeRules {
		for _, rule := range rules {
			e := idx.Entries[rule]
			if e == nil {
				// Probe-only rule — not surfaced via any invariant
				// citation. Common for the §202.2 / §903.4 / §903.5a
				// family that the deck-side probes check but no
				// game-engine invariant directly mirrors. Build a
				// minimal entry with a default description.
				e = &CitationIndexEntry{
					Rule:        rule,
					Description: defaultDescription(rule),
				}
				idx.Entries[rule] = e
			}
			if !contains(e.CheckedBy, probe) {
				e.CheckedBy = append(e.CheckedBy, probe)
			}
			if !contains(idx.ProbeToRules[probe], rule) {
				idx.ProbeToRules[probe] = append(idx.ProbeToRules[probe], rule)
			}
		}
	}

	// Step 3 + 4: cross-references + section titles.
	for rule, e := range idx.Entries {
		if related, ok := relatedRules[rule]; ok {
			for _, r := range related {
				if r == rule {
					continue
				}
				if !contains(e.RelatedRules, r) {
					e.RelatedRules = append(e.RelatedRules, r)
				}
			}
		}
		e.SectionTitle = classifySection(rule)
	}

	// Step 5: deterministic ordering.
	for _, e := range idx.Entries {
		sort.Strings(e.CheckedBy)
		sort.Strings(e.RelatedInvariants)
		sort.Strings(e.RelatedRules)
	}
	for k := range idx.ProbeToRules {
		sort.Strings(idx.ProbeToRules[k])
	}
	for k := range idx.InvariantToRules {
		sort.Strings(idx.InvariantToRules[k])
	}

	// Step 6: cross-reference CLAUDE.md Resolved-table historical
	// fixes. Best-effort — if CLAUDE.md is missing (running outside
	// the repo) the index still builds without the historical surface.
	// Errors silently skipped so `judge --citation-index` never fails
	// on a missing or malformed doc.
	if fixes, err := parseClaudemdResolvedFixes(claudemdPath()); err == nil {
		idx.UnmappedClaudemd = MergeClaudemdFixesIntoIndex(idx, fixes)
	}

	idx.Counts = CitationIndexCounts{
		TotalRules:      len(idx.Entries),
		TotalProbes:     len(idx.ProbeToRules),
		TotalInvariants: len(idx.InvariantToRules),
	}
	for _, e := range idx.Entries {
		if n := len(e.HistoricalFixes); n > 0 {
			idx.Counts.TotalFixes += n
			idx.Counts.RulesWithFixes++
		}
	}
	return idx
}

// LookupByRule returns the index entry for a rule slug, or nil when
// the slug isn't tracked. Case-insensitive on the leading character —
// "704.5A" and "704.5a" resolve to the same entry.
func (idx *CitationIndex) LookupByRule(rule string) *CitationIndexEntry {
	rule = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(rule), "§"))
	return idx.Entries[rule]
}

// LookupByProbe returns the slice of rule slugs a probe checks, in
// canonical-sorted order. Returns nil when the probe isn't tracked.
func (idx *CitationIndex) LookupByProbe(probe string) []string {
	return idx.ProbeToRules[probe]
}

// LookupByInvariant returns the slice of rule slugs cited for an
// engine invariant. Returns nil when the invariant isn't in the
// citation map.
func (idx *CitationIndex) LookupByInvariant(name string) []string {
	return idx.InvariantToRules[name]
}

// FuzzySearch returns rule slugs whose description, section title,
// or rule slug contains the lowercase search term. Returns slugs in
// canonical-sorted order.
func (idx *CitationIndex) FuzzySearch(term string) []string {
	low := strings.ToLower(strings.TrimSpace(term))
	if low == "" {
		return nil
	}
	var hits []string
	for rule, e := range idx.Entries {
		if strings.Contains(rule, low) ||
			strings.Contains(strings.ToLower(e.Description), low) ||
			strings.Contains(strings.ToLower(e.SectionTitle), low) {
			hits = append(hits, rule)
		}
	}
	sort.Strings(hits)
	return hits
}

// defaultDescription synthesizes a description for a probe-only rule
// (one that isn't surfaced via any engine invariant citation). Falls
// back to the section title — better than an empty string.
func defaultDescription(rule string) string {
	defaults := map[string]string{
		"202.2":  "Mana cost is a sequence of mana symbols per the §107.4 grammar",
		"903.4":  "Commander color identity — every deck card's CI is a subset of the commander's",
		"903.5":  "Commander deck size — exactly 100 cards including commander(s)",
		"903.5a": "A commander must be a legendary creature",
		"903.5b": "A card with 'can be your commander' text may be a commander",
		"903.6":  "Commander must be format-legal (not banned, not banned-as-commander)",
		"700":    "State-based actions and game-state integrity (umbrella)",
		"704.5d": "Token in non-battlefield zone ceases to exist",
		"704.5e": "Copy of a spell/ability in non-stack zone ceases to exist",
		"704.5q": "+1/+1 and -1/-1 counter pair removal mechanics",
		// SBA sub-rules covered by sba_probe but not surfaced via any
		// invariant citation — descriptions used by FuzzySearch.
		"704.5f": "A creature with toughness ≤ 0 goes to its owner's graveyard",
		"704.5g": "A creature with marked damage greater than or equal to its toughness is destroyed",
		"704.5h": "A planeswalker with 0 loyalty goes to its owner's graveyard",
		"704.5i": "Legend rule — if a player controls 2+ legendary permanents with the same name, that player chooses one to keep",
		"704.5j": "World rule — only one world permanent on the battlefield at a time",
		"704.6c": "A player who has been dealt 21+ combat damage from a single commander loses the game",
	}
	if d, ok := defaults[rule]; ok {
		return d
	}
	return classifySection(rule)
}

// contains is a tiny string-slice membership helper used throughout
// the index builder to keep slices duplicate-free.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// runCitationIndexDump emits the full index as indented JSON. The
// public surface for `judge --citation-index --check-out <path>`.
func runCitationIndexDump(outPath string) (*CitationIndex, error) {
	idx := BuildCitationIndex()
	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return idx, fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(idx); err != nil {
		return idx, fmt.Errorf("encode index: %w", err)
	}
	return idx, nil
}
