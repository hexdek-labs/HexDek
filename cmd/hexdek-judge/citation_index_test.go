package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// citation_index_test.go — pins the CR citation index invariants:
//
//   1. Every Rule that appears in invariantCRCitations also exists in
//      the consolidated index (forward-completeness).
//   2. Every probe registered in probeRules has every rule it claims
//      to check present in the index (reverse-completeness).
//   3. The probe→rule and rule→probe maps are bidirectional (if probe
//      P checks rule R, then index.Entries[R].CheckedBy contains P,
//      and index.ProbeToRules[P] contains R).
//   4. The invariant→rule and rule→invariant maps are bidirectional.
//   5. Every rule has a SectionTitle classified by classifySection.
//   6. relatedRules is SYMMETRIC — if A lists B, B lists A.
//   7. relatedRules contains no self-loops.
//   8. FuzzySearch returns the expected hits for canonical topics.
//   9. LookupByRule resolves "§" prefixes and case-insensitive sub-letters.
//  10. The JSON dump round-trips and includes the load-bearing fields.
//
// These tests are the maintenance contract for the index: whenever a
// new probe lands or invariantCRCitations grows, one of these will
// fail until the maintainer wires the new entry into probeRules /
// relatedRules. That's the point.

func TestCitationIndex_ForwardCompleteness_InvariantCitations(t *testing.T) {
	idx := BuildCitationIndex()
	for inv, cites := range invariantCRCitations {
		for _, c := range cites {
			e := idx.LookupByRule(c.Rule)
			if e == nil {
				t.Errorf("invariant %q cites §%s but no index entry exists for that rule",
					inv, c.Rule)
				continue
			}
			// The invariant should appear in the entry's RelatedInvariants.
			found := false
			for _, ri := range e.RelatedInvariants {
				if ri == inv {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("entry §%s missing invariant %q in RelatedInvariants: %v",
					c.Rule, inv, e.RelatedInvariants)
			}
		}
	}
}

func TestCitationIndex_ReverseCompleteness_ProbeRules(t *testing.T) {
	idx := BuildCitationIndex()
	for probe, rules := range probeRules {
		for _, rule := range rules {
			e := idx.LookupByRule(rule)
			if e == nil {
				t.Errorf("probe %q claims to check §%s but no index entry exists for that rule",
					probe, rule)
				continue
			}
			// Probe should appear in entry.CheckedBy.
			found := false
			for _, c := range e.CheckedBy {
				if c == probe {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("entry §%s missing probe %q in CheckedBy: %v",
					rule, probe, e.CheckedBy)
			}
		}
	}
}

func TestCitationIndex_ProbeToRulesBidirectional(t *testing.T) {
	idx := BuildCitationIndex()
	// For every probe → rules edge, the reverse rule → probes edge must exist.
	for probe, rules := range idx.ProbeToRules {
		for _, rule := range rules {
			e := idx.LookupByRule(rule)
			if e == nil {
				t.Fatalf("ProbeToRules references missing rule §%s", rule)
			}
			contained := false
			for _, c := range e.CheckedBy {
				if c == probe {
					contained = true
					break
				}
			}
			if !contained {
				t.Errorf("ProbeToRules[%s] → §%s but Entries[§%s].CheckedBy = %v",
					probe, rule, rule, e.CheckedBy)
			}
		}
	}
	// And the reverse — every CheckedBy entry must list the rule in ProbeToRules.
	for rule, e := range idx.Entries {
		for _, probe := range e.CheckedBy {
			found := false
			for _, r := range idx.ProbeToRules[probe] {
				if r == rule {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Entries[§%s].CheckedBy contains %q but ProbeToRules[%s] = %v",
					rule, probe, probe, idx.ProbeToRules[probe])
			}
		}
	}
}

func TestCitationIndex_InvariantToRulesBidirectional(t *testing.T) {
	idx := BuildCitationIndex()
	for inv, rules := range idx.InvariantToRules {
		for _, rule := range rules {
			e := idx.LookupByRule(rule)
			if e == nil {
				t.Fatalf("InvariantToRules references missing rule §%s", rule)
			}
			contained := false
			for _, ri := range e.RelatedInvariants {
				if ri == inv {
					contained = true
					break
				}
			}
			if !contained {
				t.Errorf("InvariantToRules[%s] → §%s but Entries[§%s].RelatedInvariants = %v",
					inv, rule, rule, e.RelatedInvariants)
			}
		}
	}
}

func TestCitationIndex_AllRulesHaveSectionTitle(t *testing.T) {
	idx := BuildCitationIndex()
	for rule, e := range idx.Entries {
		if e.SectionTitle == "" {
			t.Errorf("entry §%s has empty SectionTitle", rule)
		}
		// Verify classifySection agrees with stored title.
		if want := classifySection(rule); e.SectionTitle != want {
			t.Errorf("entry §%s SectionTitle=%q, classifySection=%q (drift)",
				rule, e.SectionTitle, want)
		}
	}
}

func TestCitationIndex_RelatedRulesAreSymmetric(t *testing.T) {
	for a, related := range relatedRules {
		for _, b := range related {
			if a == b {
				t.Errorf("relatedRules[%q] contains self-loop %q", a, b)
				continue
			}
			revList, ok := relatedRules[b]
			if !ok {
				t.Errorf("relatedRules: %q → %q exists but no reverse edge — relatedRules[%q] missing entirely",
					a, b, b)
				continue
			}
			found := false
			for _, r := range revList {
				if r == a {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("relatedRules: %q → %q is one-way; relatedRules[%q] = %v missing %q",
					a, b, b, revList, a)
			}
		}
	}
}

func TestCitationIndex_LookupByRule_StripsPrefixAndCase(t *testing.T) {
	idx := BuildCitationIndex()
	// "§704.5A" should resolve same as "704.5a".
	a := idx.LookupByRule("704.5a")
	b := idx.LookupByRule("§704.5a")
	c := idx.LookupByRule("704.5A")
	d := idx.LookupByRule("§704.5A")
	if a == nil {
		t.Fatal("baseline lookup 704.5a returned nil")
	}
	if b != a {
		t.Errorf("§ prefix should be stripped; got different entries")
	}
	if c != a {
		t.Errorf("uppercase sub-letter should fold; got different entries")
	}
	if d != a {
		t.Errorf("§ prefix + uppercase should fold; got different entries")
	}
}

func TestCitationIndex_FuzzySearch_CanonicalTopics(t *testing.T) {
	idx := BuildCitationIndex()
	cases := []struct {
		term     string
		wantRule string
	}{
		{"life", "704.5a"},
		{"poison", "704.5c"},
		{"legendary", "704.5i"},
		{"attached", "704.5k"},
		{"replacement", "614.1"},
		{"combat", "506.2"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.term, func(t *testing.T) {
			hits := idx.FuzzySearch(tc.term)
			if len(hits) == 0 {
				t.Errorf("FuzzySearch(%q) returned no hits", tc.term)
				return
			}
			found := false
			for _, h := range hits {
				if h == tc.wantRule {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("FuzzySearch(%q) missing expected §%s; got %v",
					tc.term, tc.wantRule, hits)
			}
		})
	}
}

func TestCitationIndex_KnownSpotChecks(t *testing.T) {
	idx := BuildCitationIndex()
	cases := []struct {
		rule        string
		wantSection string
		wantCheckedBy   []string  // expected substring matches
		wantRelatedRule string    // one related rule we expect to see
	}{
		{
			rule:            "704.5a",
			wantSection:     "State-Based Actions",
			wantCheckedBy:   []string{"sba_probe", "interactive_what_sbas"},
			wantRelatedRule: "104.2",
		},
		{
			rule:        "202.2",
			wantSection: "Mana Cost and Color",
			wantCheckedBy: []string{"mana_cost_check"},
		},
		{
			rule:            "903.4",
			wantSection:     "Commander Format",
			wantCheckedBy:   []string{"commander_check", "deck_construction_check"},
			wantRelatedRule: "903.5",
		},
		{
			rule:            "506.2",
			wantSection:     "Combat Phase",
			wantCheckedBy:   []string{"interactive_is_combat_legal"},
			wantRelatedRule: "508",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.rule, func(t *testing.T) {
			e := idx.LookupByRule(tc.rule)
			if e == nil {
				t.Fatalf("expected entry for §%s, got nil", tc.rule)
			}
			if e.SectionTitle != tc.wantSection {
				t.Errorf("SectionTitle = %q, want %q", e.SectionTitle, tc.wantSection)
			}
			for _, want := range tc.wantCheckedBy {
				found := false
				for _, c := range e.CheckedBy {
					if c == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("CheckedBy missing %q; got %v", want, e.CheckedBy)
				}
			}
			if tc.wantRelatedRule != "" {
				found := false
				for _, r := range e.RelatedRules {
					if r == tc.wantRelatedRule {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("RelatedRules missing §%s; got %v",
						tc.wantRelatedRule, e.RelatedRules)
				}
			}
		})
	}
}

func TestCitationIndex_JSONDumpRoundTrips(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "index.json")
	idx, err := runCitationIndexDump(out)
	if err != nil {
		t.Fatalf("runCitationIndexDump: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// Required top-level keys.
	for _, key := range []string{
		`"entries"`, `"probe_to_rules"`, `"invariant_to_rules"`, `"counts"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing top-level key %s in JSON dump", key)
		}
	}
	// Required per-entry keys (at least one entry should carry each).
	for _, key := range []string{
		`"rule"`, `"description"`, `"section_title"`,
		`"checked_by"`, `"related_invariants"`, `"related_rules"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing per-entry field %s in JSON dump", key)
		}
	}
	// Round-trip + counts sanity.
	var rt CitationIndex
	if err := json.Unmarshal(raw, &rt); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if rt.Counts.TotalRules != idx.Counts.TotalRules {
		t.Errorf("round-trip TotalRules = %d, want %d",
			rt.Counts.TotalRules, idx.Counts.TotalRules)
	}
	if rt.Counts.TotalProbes != idx.Counts.TotalProbes {
		t.Errorf("round-trip TotalProbes = %d, want %d",
			rt.Counts.TotalProbes, idx.Counts.TotalProbes)
	}
}

func TestCitationIndex_CountsMatchSourceMaps(t *testing.T) {
	idx := BuildCitationIndex()
	if idx.Counts.TotalProbes != len(probeRules) {
		t.Errorf("Counts.TotalProbes = %d, want %d (len(probeRules))",
			idx.Counts.TotalProbes, len(probeRules))
	}
	if idx.Counts.TotalInvariants != len(invariantCRCitations) {
		t.Errorf("Counts.TotalInvariants = %d, want %d (len(invariantCRCitations))",
			idx.Counts.TotalInvariants, len(invariantCRCitations))
	}
}

// ---------------------------------------------------------------------------
// Interactive REPL — new intents (index / coverage / list probes)
// ---------------------------------------------------------------------------

func TestInteractive_IndexLookup_KnownRule(t *testing.T) {
	out := runScript(t, emptyCtx(), "index 704.5a\n")
	for _, want := range []string{
		"§704.5a", "State-Based Actions", "checked by:",
		"related invariants:", "related rules:",
		"LifeConsistency", "§104.2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("index lookup missing %q in:\n%s", want, out)
		}
	}
}

func TestInteractive_IndexLookup_StripsSectionMark(t *testing.T) {
	// "index §704.5a" should resolve the same as "index 704.5a".
	out := runScript(t, emptyCtx(), "index §704.5a\n")
	if !strings.Contains(out, "State-Based Actions") {
		t.Errorf("index lookup with § prefix didn't resolve; got:\n%s", out)
	}
}

func TestInteractive_IndexLookup_UnknownRule(t *testing.T) {
	out := runScript(t, emptyCtx(), "index 999.99z\n")
	if !strings.Contains(out, "no index entry for") {
		t.Errorf("expected 'no index entry for' on unknown rule; got:\n%s", out)
	}
}

func TestInteractive_CoverageProbe(t *testing.T) {
	out := runScript(t, emptyCtx(), "coverage sba_probe\n")
	// SBA probe registers 9 sub-rules; surface at least the load-bearing ones.
	for _, want := range []string{
		"sba_probe checks", "§704.5a", "§704.5c", "§704.5f", "§704.6c",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("coverage sba_probe missing %q in:\n%s", want, out)
		}
	}
}

func TestInteractive_CoverageProbe_Unknown(t *testing.T) {
	out := runScript(t, emptyCtx(), "coverage no_such_probe\n")
	if !strings.Contains(out, "no probe named") {
		t.Errorf("expected 'no probe named' on unknown probe; got:\n%s", out)
	}
}

func TestInteractive_ListProbes(t *testing.T) {
	out := runScript(t, emptyCtx(), "list probes\n")
	// Every registered probe should appear.
	for probe := range probeRules {
		if !strings.Contains(out, probe) {
			t.Errorf("list probes missing %q in:\n%s", probe, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Determinism — slice fields are sorted for stable JSON output.
// ---------------------------------------------------------------------------

func TestCitationIndex_DeterministicSliceOrder(t *testing.T) {
	idx := BuildCitationIndex()
	for rule, e := range idx.Entries {
		if !sort.StringsAreSorted(e.CheckedBy) {
			t.Errorf("entry §%s CheckedBy not sorted: %v", rule, e.CheckedBy)
		}
		if !sort.StringsAreSorted(e.RelatedInvariants) {
			t.Errorf("entry §%s RelatedInvariants not sorted: %v", rule, e.RelatedInvariants)
		}
		if !sort.StringsAreSorted(e.RelatedRules) {
			t.Errorf("entry §%s RelatedRules not sorted: %v", rule, e.RelatedRules)
		}
	}
	for probe, rules := range idx.ProbeToRules {
		if !sort.StringsAreSorted(rules) {
			t.Errorf("ProbeToRules[%s] not sorted: %v", probe, rules)
		}
	}
}
