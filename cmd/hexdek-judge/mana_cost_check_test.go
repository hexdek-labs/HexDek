package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateManaCost_LegalCosts pins the CR §202.2 happy-path
// grammar — every form that real Scryfall data emits must be accepted.
// Documents the full set of valid symbols by example so a future hand
// at adding a new printed cost knows what's already covered.
func TestValidateManaCost_LegalCosts(t *testing.T) {
	cases := []struct {
		name string
		cost string
	}{
		{"empty (land)", ""},
		{"only whitespace", "   "},
		{"single generic", "{1}"},
		{"generic + colored", "{3}{G}"},
		{"all five colors", "{W}{U}{B}{R}{G}"},
		{"colorless", "{C}"},
		{"snow", "{S}"},
		{"variable X", "{X}{X}"},
		{"hybrid allied", "{W/U}"},
		{"hybrid enemy", "{B/G}"},
		{"hybrid all ten — sanity", "{W/U}{U/B}{B/R}{R/G}{G/W}{W/B}{U/R}{B/G}{R/W}{G/U}"},
		{"generic-or-colored", "{2/W}{2/U}{2/B}{2/R}{2/G}"},
		{"phyrexian mono", "{W/P}{U/P}{B/P}{R/P}{G/P}"},
		{"phyrexian hybrid", "{W/U/P}"},
		{"large generic (Khalni Hydra)", "{15}"},
		{"hybrid sort-stable", "{U/W}"}, // {U/W} normalizes to {W/U}
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			bad, kind, reason := validateManaCost(tc.cost)
			if bad != "" {
				t.Errorf("validateManaCost(%q): expected valid, got bad=%q kind=%q reason=%q",
					tc.cost, bad, kind, reason)
			}
		})
	}
}

// TestValidateManaCost_OutOfGrammar pins the failure modes. Each entry
// exercises a distinct rule violation against CR §202.2.
func TestValidateManaCost_OutOfGrammar(t *testing.T) {
	cases := []struct {
		name      string
		cost      string
		wantKind  string
		hintInBad string // substring that should appear in the BadSymbol
	}{
		{"unknown letter",
			"{Q}", "out_of_grammar", "Q"},
		{"unbalanced opening brace",
			"{1{R}", "malformed_brace", ""},
		{"unbalanced closing brace",
			"{1}{R}}", "malformed_brace", ""},
		{"bare characters outside braces",
			"1R", "malformed_brace", "1R"},
		{"out-of-range generic",
			"{99}", "out_of_grammar", "99"},
		{"three-color non-Phyrexian hybrid",
			"{W/G/B}", "out_of_grammar", "W/G/B"}, // CR §107.4 only defines 2-color hybrid pairs
		{"draft-innovation land-drop symbol",
			"{D}", "out_of_grammar", "D"}, // Mystery Booster 2 {D} land-drop — not a CR §202.2 symbol
		{"half-pip (Un-set)",
			"{HW}", "out_of_grammar", "HW"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			bad, kind, reason := validateManaCost(tc.cost)
			if bad == "" {
				t.Fatalf("validateManaCost(%q): expected violation, got valid", tc.cost)
			}
			if kind != tc.wantKind {
				t.Errorf("validateManaCost(%q): kind = %q, want %q (reason=%q bad=%q)",
					tc.cost, kind, tc.wantKind, reason, bad)
			}
			if tc.hintInBad != "" && !strings.Contains(bad, tc.hintInBad) {
				t.Errorf("validateManaCost(%q): expected bad=%q to contain %q",
					tc.cost, bad, tc.hintInBad)
			}
		})
	}
}

// TestCanonicalSymbol pins the symbol normalization. validManaSymbols
// stores both Scryfall's canonical printed order AND the reversed
// ordering for hybrid pairs, so the lookup never depends on WUBRG-cycle
// awareness — canonicalSymbol's job is just: uppercase, alpha-sort the
// letter set, keep numeric prefix at head, keep Phyrexian /P at tail.
func TestCanonicalSymbol(t *testing.T) {
	checks := []struct {
		in, want string
	}{
		{"W", "W"},
		{"U/W", "U/W"},     // already alphabetical
		{"W/U", "U/W"},     // reversed → same canonical
		{"G/W", "G/W"},
		{"W/G", "G/W"},     // reversed
		{"W/P", "W/P"},     // mono Phyrexian — letter, then /P
		{"2/W", "2/W"},     // generic-or-colored — numeric leads
		{"W/U/P", "U/W/P"}, // hybrid Phyrexian — letters alphabetized, P at tail
		{"u/w/p", "U/W/P"}, // lowercase input is upper-folded
	}
	for _, c := range checks {
		got := canonicalSymbol(c.in)
		if got != c.want {
			t.Errorf("canonicalSymbol(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestRunManaCostCheck_CleanCorpus runs the batch probe against a
// hand-rolled tiny oracle JSON containing only valid mana costs.
// Pins the JSON output shape — Valid=true, no Violations, the section
// counters add up.
func TestRunManaCostCheck_CleanCorpus(t *testing.T) {
	const corpus = `[
		{"name":"Lightning Bolt","set":"lea","mana_cost":"{R}"},
		{"name":"Counterspell","set":"lea","mana_cost":"{U}{U}"},
		{"name":"Plains","set":"lea","mana_cost":""},
		{"name":"Stoneforge Mystic","set":"wwk","mana_cost":"{1}{W}"},
		{"name":"Brazen Borrower // Petty Theft","set":"eld","layout":"adventure","card_faces":[
			{"name":"Brazen Borrower","mana_cost":"{1}{U}"},
			{"name":"Petty Theft","mana_cost":"{1}{U}"}
		]}
	]`
	tmp := t.TempDir()
	oraclePath := filepath.Join(tmp, "oracle.json")
	if err := os.WriteFile(oraclePath, []byte(corpus), 0644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(tmp, "report.json")
	rep, err := runManaCostCheck(oraclePath, outPath)
	if err != nil {
		t.Fatalf("runManaCostCheck: %v", err)
	}
	if !rep.Valid {
		t.Errorf("expected Valid=true on clean corpus, got Valid=false, violations=%+v", rep.Violations)
	}
	if len(rep.Violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(rep.Violations), rep.Violations)
	}
	// Counters: 5 cards total, Plains has empty mana_cost → 1 vanilla;
	// adventure faces are walked individually (2 faces), other 3 are
	// single-face with cost → 5 scanned (3 + 2 faces), 1 vanilla.
	if rep.TotalCards != 5 {
		t.Errorf("TotalCards = %d, want 5", rep.TotalCards)
	}
	if rep.Vanilla != 1 {
		t.Errorf("Vanilla = %d, want 1", rep.Vanilla)
	}
	if rep.Scanned != 5 {
		t.Errorf("Scanned = %d (want 5 — 3 single-face + 2 adventure faces)", rep.Scanned)
	}
	// Verify the report wrote to disk and round-trips.
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var roundtripped ManaCostReport
	if err := json.Unmarshal(raw, &roundtripped); err != nil {
		t.Fatalf("report JSON round-trip: %v", err)
	}
	if !roundtripped.Valid {
		t.Errorf("round-tripped report.Valid = false, want true")
	}
}

// TestRunManaCostCheck_FlagsViolations confirms the batch probe
// actually surfaces a mix of failure modes from a synthetic corpus.
// Verifies the per-kind breakdown counter and the stable
// (name-sorted) violation order.
func TestRunManaCostCheck_FlagsViolations(t *testing.T) {
	const corpus = `[
		{"name":"Valid Card","set":"tst","mana_cost":"{1}{R}"},
		{"name":"Bad Letter","set":"tst","mana_cost":"{Q}"},
		{"name":"Bad Hybrid","set":"tst","mana_cost":"{W/G/B}"},
		{"name":"Half Pip","set":"tst","mana_cost":"{HW}"},
		{"name":"Unbalanced","set":"tst","mana_cost":"{1{R}"}
	]`
	tmp := t.TempDir()
	oraclePath := filepath.Join(tmp, "oracle.json")
	if err := os.WriteFile(oraclePath, []byte(corpus), 0644); err != nil {
		t.Fatal(err)
	}
	rep, err := runManaCostCheck(oraclePath, filepath.Join(tmp, "out.json"))
	if err != nil {
		t.Fatalf("runManaCostCheck: %v", err)
	}
	if rep.Valid {
		t.Fatal("expected Valid=false")
	}
	if len(rep.Violations) != 4 {
		t.Fatalf("expected 4 violations, got %d: %+v", len(rep.Violations), rep.Violations)
	}
	// Stable order (name-sorted): Bad Hybrid, Bad Letter, Half Pip, Unbalanced.
	wantNames := []string{"Bad Hybrid", "Bad Letter", "Half Pip", "Unbalanced"}
	for i, want := range wantNames {
		if rep.Violations[i].CardName != want {
			t.Errorf("violation[%d].CardName = %q, want %q", i, rep.Violations[i].CardName, want)
		}
	}
	// Per-kind tally — 3 out_of_grammar + 1 malformed_brace.
	if rep.ViolationsByKind["out_of_grammar"] != 3 {
		t.Errorf("out_of_grammar count = %d, want 3", rep.ViolationsByKind["out_of_grammar"])
	}
	if rep.ViolationsByKind["malformed_brace"] != 1 {
		t.Errorf("malformed_brace count = %d, want 1", rep.ViolationsByKind["malformed_brace"])
	}
}

// TestRunManaCostCheck_SkipsUnsetAndSilverBorder confirms the
// excludedSetTypes / silver-border filter mirrors the parser-coverage
// audit's denominator scope. Un-set / Alchemy cards have their own
// grammar (half-pips, perpetual costs) that CR §202.2 doesn't cover;
// including them would flood the violation list.
func TestRunManaCostCheck_SkipsUnsetAndSilverBorder(t *testing.T) {
	const corpus = `[
		{"name":"Real Card","set":"lea","mana_cost":"{R}"},
		{"name":"Un-set Half-Pip","set":"ust","set_type":"funny","mana_cost":"{HW}{HW}"},
		{"name":"Silver Border Card","set":"unh","border_color":"silver","mana_cost":"{∞}"},
		{"name":"Mystery Booster Playtest","set":"cmb1","set_type":"funny","mana_cost":"{ALPHA}"},
		{"name":"Alchemy Spellbook","set":"yyer","set_type":"alchemy","mana_cost":"{PERPETUAL}"}
	]`
	tmp := t.TempDir()
	oraclePath := filepath.Join(tmp, "oracle.json")
	if err := os.WriteFile(oraclePath, []byte(corpus), 0644); err != nil {
		t.Fatal(err)
	}
	rep, err := runManaCostCheck(oraclePath, "")
	if err != nil {
		t.Fatalf("runManaCostCheck: %v", err)
	}
	if !rep.Valid {
		t.Errorf("expected clean — filter should drop Un-set / Alchemy / silver — got violations: %+v", rep.Violations)
	}
	// Counters: 5 total cards, 1 scanned (only "Real Card"), 4 excluded
	// before reaching the scan step (not counted as Vanilla).
	if rep.TotalCards != 5 {
		t.Errorf("TotalCards = %d, want 5", rep.TotalCards)
	}
	if rep.Scanned != 1 {
		t.Errorf("Scanned = %d, want 1 — Un-set + Alchemy + silver should all be filtered before scan", rep.Scanned)
	}
}

// TestRunManaCostCheck_ReportJSONShape pins the JSON output schema
// so downstream CI tooling (Make hooks, dashboards) can rely on the
// field names and types. A schema-shape change here would require
// updating consumers.
func TestRunManaCostCheck_ReportJSONShape(t *testing.T) {
	const corpus = `[
		{"name":"Bolt","set":"lea","mana_cost":"{R}"},
		{"name":"Bogus","set":"tst","mana_cost":"{Q}"}
	]`
	tmp := t.TempDir()
	oraclePath := filepath.Join(tmp, "oracle.json")
	outPath := filepath.Join(tmp, "out.json")
	if err := os.WriteFile(oraclePath, []byte(corpus), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runManaCostCheck(oraclePath, outPath); err != nil {
		t.Fatalf("runManaCostCheck: %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	// Required top-level keys.
	for _, key := range []string{
		`"oracle_path"`, `"total_cards"`, `"scanned"`, `"vanilla"`,
		`"violations"`, `"violations_by_kind"`, `"valid"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing required top-level key %s in:\n%s", key, raw)
		}
	}
	// Violation must carry the documented fields.
	for _, key := range []string{
		`"card_name"`, `"mana_cost"`, `"bad_symbol"`, `"kind"`, `"reason"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing required violation field %s in:\n%s", key, raw)
		}
	}
}
