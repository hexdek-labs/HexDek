package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Synthetic fixtures only — the audit tool's real inputs (50MB+
// JSONL + 173MB JSON) are gitignored and unavailable in CI. Every
// test here uses inline data.

func TestExtractASTKeywords_FlatNode(t *testing.T) {
	raw := `{"__ast_type__":"Keyword","name":"flying"}`
	var node any
	if err := json.Unmarshal([]byte(raw), &node); err != nil {
		t.Fatal(err)
	}
	got := extractASTKeywords(node)
	if len(got) != 1 || got[0] != "flying" {
		t.Errorf("flat keyword: want [flying], got %v", got)
	}
}

func TestExtractASTKeywords_NestedAbilities(t *testing.T) {
	raw := `{
		"__ast_type__":"CardAST",
		"abilities":[
			{"__ast_type__":"Keyword","name":"flying"},
			{"__ast_type__":"Keyword","name":"trample"},
			{"__ast_type__":"Static","modification":{"__ast_type__":"Modification","args":[
				{"__ast_type__":"Keyword","name":"vigilance"}
			]}}
		]
	}`
	var node any
	json.Unmarshal([]byte(raw), &node)
	got := extractASTKeywords(node)
	want := []string{"flying", "trample", "vigilance"}
	if !sliceEq(got, want) {
		t.Errorf("nested: want %v, got %v", want, got)
	}
}

func TestExtractASTKeywords_DedupesRepeated(t *testing.T) {
	raw := `{
		"abilities":[
			{"__ast_type__":"Keyword","name":"scry"},
			{"__ast_type__":"Keyword","name":"scry"},
			{"__ast_type__":"Keyword","name":"scry"}
		]
	}`
	var node any
	json.Unmarshal([]byte(raw), &node)
	got := extractASTKeywords(node)
	if len(got) != 1 {
		t.Errorf("dedupe: want 1 entry, got %d (%v)", len(got), got)
	}
}

func TestExtractASTKeywords_HandlesNilAndEmpty(t *testing.T) {
	if got := extractASTKeywords(nil); len(got) != 0 {
		t.Errorf("nil: want empty, got %v", got)
	}
	if got := extractASTKeywords(map[string]any{}); len(got) != 0 {
		t.Errorf("empty map: want empty, got %v", got)
	}
}

func TestNormalizeText_CollapsesWhitespace(t *testing.T) {
	// All whitespace runs (spaces, tabs, CRLF, repeated newlines)
	// collapse to a single space. Aggressive on purpose — drift
	// detection cares about WORDS, not formatting.
	in := "  Flying.\r\n\nTrample.   \n\n\nVigilance.  "
	want := "flying. trample. vigilance."
	if got := normalizeText(in); got != want {
		t.Errorf("normalize: got %q want %q", got, want)
	}
}

func TestCheckOracleTextDrift_NoDrift(t *testing.T) {
	a := astEntry{Name: "X", OracleText: "Flying.\n\nTrample."}
	o := oracleEntry{Name: "X", OracleText: "Flying.\n\nTrample."}
	if d := checkOracleTextDrift(a, o); d != nil {
		t.Errorf("identical text should produce no drift, got %+v", d)
	}
}

func TestCheckOracleTextDrift_WhitespaceOnlyIgnored(t *testing.T) {
	a := astEntry{Name: "X", OracleText: "  Flying.\n  Trample.  "}
	o := oracleEntry{Name: "X", OracleText: "Flying.\nTrample."}
	if d := checkOracleTextDrift(a, o); d != nil {
		t.Errorf("whitespace-only diff should NOT count as drift, got %+v", d)
	}
}

func TestCheckOracleTextDrift_SubstantiveDriftDetected(t *testing.T) {
	a := astEntry{Name: "X", OracleText: "When this creature enters, gain 2 life."}
	o := oracleEntry{Name: "X", OracleText: "When this creature enters, gain 3 life."}
	d := checkOracleTextDrift(a, o)
	if d == nil {
		t.Fatal("substantive drift should produce a finding")
	}
	if d.Category != CatOracleTextDrift {
		t.Errorf("category: want %q got %q", CatOracleTextDrift, d.Category)
	}
	if !strings.Contains(d.Details, "ast=") || !strings.Contains(d.Details, "oracle=") {
		t.Errorf("Details should show both sides of the divergence: %q", d.Details)
	}
}

func TestCheckOracleTextDrift_MultiFaceMerges(t *testing.T) {
	// Card faces should be joined with " // " to match Scryfall's
	// printed-card convention. An AST whose oracle_text reflects the
	// joined view shouldn't be flagged.
	a := astEntry{Name: "Adventure Card", OracleText: "Creature face text. // Adventure face text."}
	o := oracleEntry{Name: "Adventure Card"}
	o.CardFaces = []struct {
		OracleText string `json:"oracle_text"`
		ManaCost   string `json:"mana_cost"`
		TypeLine   string `json:"type_line"`
	}{
		{OracleText: "Creature face text."},
		{OracleText: "Adventure face text."},
	}
	if d := checkOracleTextDrift(a, o); d != nil {
		t.Errorf("multi-face merge should match joined AST text, got drift: %+v", d)
	}
}

func TestCheckTypeLineDrift(t *testing.T) {
	a := astEntry{Name: "X", TypeLine: "Creature — Elf"}
	o := oracleEntry{Name: "X", TypeLine: "Creature — Elf Druid"}
	d := checkTypeLineDrift(a, o)
	if d == nil || d.Category != CatTypeLineDrift {
		t.Fatalf("want type_line_drift, got %+v", d)
	}
}

func TestCheckCMCDrift(t *testing.T) {
	a := astEntry{Name: "X", CMC: 4.0}
	o := oracleEntry{Name: "X", CMC: 5.0}
	if d := checkCMCDrift(a, o); d == nil || d.Category != CatCMCDrift {
		t.Fatalf("want cmc_drift, got %+v", d)
	}
	// Matching CMCs: no finding.
	o2 := oracleEntry{Name: "X", CMC: 4.0}
	if d := checkCMCDrift(a, o2); d != nil {
		t.Errorf("matching CMC should not produce drift, got %+v", d)
	}
}

func TestCheckCMCDrift_ToleratesFloatNoise(t *testing.T) {
	a := astEntry{Name: "X", CMC: 4.0000000001}
	o := oracleEntry{Name: "X", CMC: 4.0}
	if d := checkCMCDrift(a, o); d != nil {
		t.Errorf("float-encoding noise should not produce drift, got %+v", d)
	}
}

func TestCheckManaCostDrift(t *testing.T) {
	a := astEntry{Name: "X", ManaCost: "{3}{G}"}
	o := oracleEntry{Name: "X", ManaCost: "{2}{G}{G}"}
	if d := checkManaCostDrift(a, o); d == nil || d.Category != CatManaCostDrift {
		t.Fatalf("want mana_cost_drift, got %+v", d)
	}
}

func TestCheckASTHallucination_FiresOnMissingKeyword(t *testing.T) {
	// AST asserts flying + trample; oracle text only has flying.
	rawAST := `{"abilities":[
		{"__ast_type__":"Keyword","name":"flying"},
		{"__ast_type__":"Keyword","name":"trample"}
	]}`
	var ast any
	json.Unmarshal([]byte(rawAST), &ast)
	a := astEntry{Name: "X", AST: ast}
	d := checkASTHallucination(a, "Flying.")
	if d == nil {
		t.Fatal("want hallucination finding for missing trample")
	}
	if d.Category != CatKeywordHallucinate {
		t.Errorf("category: want %q got %q", CatKeywordHallucinate, d.Category)
	}
	if !strings.Contains(d.Details, "trample") {
		t.Errorf("Details should name the hallucinated keyword, got %q", d.Details)
	}
	if strings.Contains(d.Details, "flying") {
		t.Errorf("Details should NOT name the keyword that IS present, got %q", d.Details)
	}
}

func TestCheckASTHallucination_NoFalsePositiveWhenAllPresent(t *testing.T) {
	rawAST := `{"abilities":[
		{"__ast_type__":"Keyword","name":"flying"},
		{"__ast_type__":"Keyword","name":"trample"}
	]}`
	var ast any
	json.Unmarshal([]byte(rawAST), &ast)
	a := astEntry{Name: "X", AST: ast}
	if d := checkASTHallucination(a, "Flying, trample."); d != nil {
		t.Errorf("all keywords present should not flag: %+v", d)
	}
}

func TestCheckASTHallucination_SkipsParserInternalAliases(t *testing.T) {
	// first_strike is the parser's snake_case alias for the printed
	// "first strike" — must not flag even when "first_strike" is not
	// a substring of the oracle text.
	rawAST := `{"abilities":[
		{"__ast_type__":"Keyword","name":"first_strike"}
	]}`
	var ast any
	json.Unmarshal([]byte(rawAST), &ast)
	a := astEntry{Name: "X", AST: ast}
	if d := checkASTHallucination(a, "First strike."); d != nil {
		t.Errorf("alias 'first_strike' must be skipped, got finding: %+v", d)
	}
}

func TestCheckASTHallucination_CaseInsensitive(t *testing.T) {
	rawAST := `{"abilities":[
		{"__ast_type__":"Keyword","name":"Flying"}
	]}`
	var ast any
	json.Unmarshal([]byte(rawAST), &ast)
	a := astEntry{Name: "X", AST: ast}
	if d := checkASTHallucination(a, "flying."); d != nil {
		t.Errorf("case-insensitive match should not flag, got: %+v", d)
	}
}

func TestAuditEntry_MissingInOracleCategory(t *testing.T) {
	a := astEntry{Name: "Ghost Card"}
	findings := auditEntry(a, map[string]oracleEntry{})
	if len(findings) != 1 {
		t.Fatalf("missing card should produce exactly 1 finding, got %d", len(findings))
	}
	if findings[0].Category != CatMissingInOracle {
		t.Errorf("category: want %q got %q", CatMissingInOracle, findings[0].Category)
	}
}

func TestAuditEntry_MultipleFindingsAccumulate(t *testing.T) {
	rawAST := `{"abilities":[{"__ast_type__":"Keyword","name":"ward"}]}`
	var ast any
	json.Unmarshal([]byte(rawAST), &ast)
	a := astEntry{
		Name:       "Drifted Card",
		OracleText: "Old text.",
		TypeLine:   "Old type",
		CMC:        2.0,
		ManaCost:   "{1}{U}",
		AST:        ast,
	}
	o := oracleEntry{
		Name:       "Drifted Card",
		OracleText: "Brand new text.", // text drift
		TypeLine:   "New type",        // type drift
		CMC:        3.0,                // cmc drift
		ManaCost:   "{2}{U}",          // mana drift
	}
	findings := auditEntry(a, map[string]oracleEntry{"Drifted Card": o})
	cats := map[string]bool{}
	for _, d := range findings {
		cats[d.Category] = true
	}
	for _, want := range []string{
		CatOracleTextDrift,
		CatTypeLineDrift,
		CatCMCDrift,
		CatManaCostDrift,
		CatKeywordHallucinate, // ward not in oracle text
	} {
		if !cats[want] {
			t.Errorf("multi-drift card missing category %q in findings %v", want, findings)
		}
	}
}

func TestWriteReport_ShapeAndCounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	findings := []Discrepancy{
		{Name: "Alpha", Category: CatOracleTextDrift, Details: "alpha-detail"},
		{Name: "Beta", Category: CatOracleTextDrift, Details: "beta-detail"},
		{Name: "Gamma", Category: CatMissingInOracle, Details: "gamma-detail"},
	}
	if err := writeReport(path, findings, 100, 200, 50); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)

	for _, want := range []string{
		"# Audit: AST Dataset vs Scryfall Oracle Corpus",
		"AST entries scanned | 100",
		"Oracle entries indexed | 200",
		"Total discrepancies | 3",
		"`missing_in_oracle` — 1 findings",
		"`oracle_text_drift` — 2 findings",
		"Alpha",
		"Beta",
		"Gamma",
		"Reading this report",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n---\n%s", want, out)
		}
	}
}

func TestWriteReport_TruncatesToTopN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	var findings []Discrepancy
	for i := 0; i < 100; i++ {
		findings = append(findings, Discrepancy{
			Name:     "Card-" + string(rune('A'+i%26)) + itoa(i),
			Category: CatOracleTextDrift,
			Details:  "drift",
		})
	}
	if err := writeReport(path, findings, 100, 100, 10); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	if !strings.Contains(out, "`oracle_text_drift` — 100 findings") {
		t.Error("count should reflect ALL findings, not just shown")
	}
	if !strings.Contains(out, "90 more") {
		t.Errorf("expected truncation marker '90 more' for top=10 of 100, got:\n%s", out)
	}
}

func TestWriteReport_EmptyCategoryNoneDetected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	if err := writeReport(path, nil, 100, 200, 50); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	// Each category section should render "_None detected._" when empty.
	if !strings.Contains(out, "_None detected._") {
		t.Errorf("empty findings should produce 'None detected' markers, got:\n%s", out)
	}
}

func TestAuditAST_StreamsAndDecodes(t *testing.T) {
	// Build a 3-line synthetic JSONL fixture exercising the streaming
	// loader + per-entry audit pipeline end-to-end.
	dir := t.TempDir()
	astPath := filepath.Join(dir, "ast.jsonl")
	lines := []string{
		`{"name":"Card A","oracle_text":"Flying.","type_line":"Creature","cmc":1.0,"mana_cost":"{W}","ast":{"abilities":[{"__ast_type__":"Keyword","name":"flying"}]}}`,
		`{"name":"Card B","oracle_text":"Trample.","type_line":"Creature","cmc":2.0,"mana_cost":"{1}{G}","ast":{"abilities":[{"__ast_type__":"Keyword","name":"trample"}]}}`,
		`{"name":"Ghost","oracle_text":"x","type_line":"x","cmc":1.0,"ast":{}}`,
	}
	if err := os.WriteFile(astPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oracle := map[string]oracleEntry{
		"Card A": {Name: "Card A", OracleText: "Flying.", TypeLine: "Creature", CMC: 1.0, ManaCost: "{W}"},
		"Card B": {Name: "Card B", OracleText: "Trample.", TypeLine: "Creature", CMC: 2.0, ManaCost: "{1}{G}"},
		// Ghost is intentionally missing from oracle.
	}
	findings, scanned, err := auditAST(astPath, oracle)
	if err != nil {
		t.Fatal(err)
	}
	if scanned != 3 {
		t.Errorf("scanned: want 3, got %d", scanned)
	}
	// Card A and Card B match cleanly; Ghost produces one finding.
	if len(findings) != 1 {
		t.Errorf("findings: want 1 (Ghost missing), got %d (%+v)", len(findings), findings)
	}
	if findings[0].Category != CatMissingInOracle || findings[0].Name != "Ghost" {
		t.Errorf("expected missing_in_oracle for Ghost, got %+v", findings[0])
	}
}

func TestSummarizeCounts_AllCategoriesListed(t *testing.T) {
	findings := []Discrepancy{
		{Category: CatOracleTextDrift},
		{Category: CatOracleTextDrift},
		{Category: CatCMCDrift},
	}
	got := summarizeCounts(findings)
	for _, want := range CategoryOrder {
		if !strings.Contains(got, want+"=") {
			t.Errorf("summary missing %q: %q", want, got)
		}
	}
	if !strings.Contains(got, "oracle_text_drift=2") {
		t.Errorf("oracle_text_drift count: %q", got)
	}
	if !strings.Contains(got, "cmc_drift=1") {
		t.Errorf("cmc_drift count: %q", got)
	}
}

// sliceEq compares two string slices for equality after sorting.
// extractASTKeywords returns sorted output but the test's `want`
// might list in declaration order.
func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
