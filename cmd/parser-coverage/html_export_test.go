package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestPrimaryType_StripsLegendary(t *testing.T) {
	if got := primaryType("Legendary Creature — Elf Wraith"); got != "Creature" {
		t.Errorf("Legendary Creature: got %q, want Creature", got)
	}
}

func TestPrimaryType_BasicLand(t *testing.T) {
	if got := primaryType("Basic Land — Forest"); got != "Land" {
		t.Errorf("Basic Land: got %q, want Land", got)
	}
}

func TestPrimaryType_StacksSupertypes(t *testing.T) {
	// Snow + Basic + Land — three supertypes-then-type.
	if got := primaryType("Snow Basic Land — Mountain"); got != "Land" {
		t.Errorf("Snow Basic Land: got %q, want Land", got)
	}
}

func TestPrimaryType_NoSubtype(t *testing.T) {
	if got := primaryType("Artifact"); got != "Artifact" {
		t.Errorf("Artifact only: got %q", got)
	}
}

func TestPrimaryType_Empty(t *testing.T) {
	if got := primaryType(""); got != "" {
		t.Errorf("empty type line: got %q, want empty", got)
	}
}

func TestPrimaryType_Saga(t *testing.T) {
	if got := primaryType("Enchantment — Saga"); got != "Enchantment" {
		t.Errorf("Saga: got %q, want Enchantment", got)
	}
}

func TestPrimaryType_Planeswalker(t *testing.T) {
	if got := primaryType("Legendary Planeswalker — Liliana"); got != "Planeswalker" {
		t.Errorf("Planeswalker: got %q, want Planeswalker", got)
	}
}

func TestExtractYear_HappyPath(t *testing.T) {
	if got := extractYear("2026-05-24"); got != 2026 {
		t.Errorf("2026-05-24: got %d", got)
	}
}

func TestExtractYear_Empty(t *testing.T) {
	if got := extractYear(""); got != 0 {
		t.Errorf("empty: got %d", got)
	}
}

func TestExtractYear_Malformed(t *testing.T) {
	if got := extractYear("not-a-date"); got != 0 {
		t.Errorf("malformed: got %d", got)
	}
}

func TestEraFromYear_AllBuckets(t *testing.T) {
	cases := []struct {
		y    int
		want string
	}{
		{0, "unknown"},
		{1993, "classic (1993-1996)"},
		{1996, "classic (1993-1996)"},
		{1997, "early (1997-2002)"},
		{2002, "early (1997-2002)"},
		{2003, "modern early (2003-2009)"},
		{2009, "modern early (2003-2009)"},
		{2010, "modern late (2010-2017)"},
		{2017, "modern late (2010-2017)"},
		{2018, "contemporary (2018+)"},
		{2026, "contemporary (2018+)"},
	}
	for _, c := range cases {
		if got := eraFromYear(c.y); got != c.want {
			t.Errorf("year %d: got %q, want %q", c.y, got, c.want)
		}
	}
}

func TestResultsToHTMLPayload_ExcludesOKByDefault(t *testing.T) {
	rs := fixture() // 12 missing + 10 empty + 3 partial = 25 uncovered, 4 OK + 1 OKV = 5 ok
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	p := resultsToHTMLPayload(rs, false, now)
	if len(p.Cards) != 25 {
		t.Errorf("default scope: want 25 cards (uncovered only), got %d", len(p.Cards))
	}
	// Counts include OK rows even though cards array doesn't.
	if p.Counts["OK"] != 4 {
		t.Errorf("counts.OK should reflect full audit: want 4, got %d", p.Counts["OK"])
	}
	if p.Counts["OK_VANILLA"] != 1 {
		t.Errorf("counts.OK_VANILLA: want 1, got %d", p.Counts["OK_VANILLA"])
	}
	if p.Counts["MISSING"] != 12 {
		t.Errorf("counts.MISSING: want 12, got %d", p.Counts["MISSING"])
	}
}

func TestResultsToHTMLPayload_IncludeOKEmitsAllCards(t *testing.T) {
	rs := fixture()
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	p := resultsToHTMLPayload(rs, true, now)
	if len(p.Cards) != 30 {
		t.Errorf("include-ok: want all 30 cards, got %d", len(p.Cards))
	}
}

func TestResultsToHTMLPayload_TimestampUTC(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip("timezone db unavailable")
	}
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, loc)
	p := resultsToHTMLPayload(nil, false, now)
	if !strings.HasSuffix(p.GeneratedAt, "Z") {
		t.Errorf("GeneratedAt must be UTC (suffix Z): got %q", p.GeneratedAt)
	}
}

func TestResultsToHTMLPayload_DerivesEraAndPrimaryType(t *testing.T) {
	rs := []result{
		{Name: "Old Card", Class: classEmptyAST, TypeLine: "Legendary Creature — Dragon", ReleasedAt: "1994-08-01", SetName: "Legends"},
	}
	p := resultsToHTMLPayload(rs, false, time.Now())
	if len(p.Cards) != 1 {
		t.Fatalf("want 1 card, got %d", len(p.Cards))
	}
	c := p.Cards[0]
	if c.Year != 1994 {
		t.Errorf("year: want 1994, got %d", c.Year)
	}
	if c.Era != "classic (1993-1996)" {
		t.Errorf("era: want classic, got %q", c.Era)
	}
	if c.PrimaryType != "Creature" {
		t.Errorf("primaryType: want Creature, got %q", c.PrimaryType)
	}
}

func TestSafeJSON_EscapesClosingScriptTag(t *testing.T) {
	// Defense in depth: an oracle string containing "</script>" must
	// not survive verbatim into the rendered HTML, or the inline JS
	// payload would end prematurely.
	js, err := safeJSON(map[string]string{"oracle": "evil </script><img src=x>"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(js), "</script>") {
		t.Errorf("safeJSON leaked literal </script> into output: %s", js)
	}
	// encoding/json's default escapes < and > to \u003c / \u003e, so
	// we expect the unicode escape to appear in the marshaled bytes
	// rather than a literal angle bracket.
	if !strings.Contains(string(js), `\u003c`) {
		t.Errorf("expected literal \\u003c escape sequence in output: %s", js)
	}
	// Round-trips through JSON parse to the original string.
	var decoded map[string]string
	if err := json.Unmarshal([]byte(js), &decoded); err != nil {
		t.Fatalf("decode round-trip: %v", err)
	}
	if decoded["oracle"] != "evil </script><img src=x>" {
		t.Errorf("round trip mangled: %q", decoded["oracle"])
	}
}

func TestSafeJSON_PreservesNormalContent(t *testing.T) {
	js, err := safeJSON(map[string]int{"a": 1, "b": 2})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]int
	if err := json.Unmarshal([]byte(js), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["a"] != 1 || decoded["b"] != 2 {
		t.Errorf("normal content mangled: %v", decoded)
	}
}

func TestWriteHTMLReport_SelfContainedHTML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.html")
	rs := fixture()
	if err := writeHTMLReport(path, rs, false, time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("writeHTMLReport: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(got)
	for _, want := range []string{
		"<!DOCTYPE html>",
		"<title>Parser Coverage — Interactive</title>",
		"const DATA =",
		`"counts"`,
		`"cards"`,
		"id=\"filters-class\"",
		"id=\"filters-era\"",
		"id=\"filters-type\"",
		"id=\"set-select\"",
		"id=\"search\"",
		"2026-05-24T12:00:00Z",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	// No external resources — defends against accidentally adding a
	// CDN link or external image which would break offline use.
	for _, banned := range []string{"https://cdn", "<script src=", "<link rel=\"stylesheet\""} {
		if strings.Contains(html, banned) {
			t.Errorf("HTML must be self-contained; found %q", banned)
		}
	}
}

func TestWriteHTMLReport_EmbeddedDataParsesAsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.html")
	if err := writeHTMLReport(path, fixture(), false, time.Now()); err != nil {
		t.Fatalf("writeHTMLReport: %v", err)
	}
	html, _ := os.ReadFile(path)
	// Extract the inline JSON between `const DATA = ` and the trailing
	// `;` on the same expression. The template emits a single line
	// per the html/template engine; we just match the literal.
	re := regexp.MustCompile(`const DATA = (.+?);`)
	m := re.FindStringSubmatch(string(html))
	if m == nil {
		t.Fatal("could not find DATA literal in HTML output")
	}
	var payload htmlPayload
	if err := json.Unmarshal([]byte(m[1]), &payload); err != nil {
		t.Fatalf("embedded JSON failed to parse: %v", err)
	}
	if len(payload.Cards) != 25 {
		t.Errorf("embedded payload expected 25 uncovered cards, got %d", len(payload.Cards))
	}
	if payload.Counts["MISSING"] != 12 {
		t.Errorf("embedded counts.MISSING expected 12, got %d", payload.Counts["MISSING"])
	}
}

func TestWriteHTMLReport_IncludeOKWidensCardArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.html")
	if err := writeHTMLReport(path, fixture(), true, time.Now()); err != nil {
		t.Fatalf("writeHTMLReport: %v", err)
	}
	html, _ := os.ReadFile(path)
	re := regexp.MustCompile(`const DATA = (.+?);`)
	m := re.FindStringSubmatch(string(html))
	var payload htmlPayload
	json.Unmarshal([]byte(m[1]), &payload)
	if len(payload.Cards) != 30 {
		t.Errorf("include-ok: want all 30 in HTML payload, got %d", len(payload.Cards))
	}
}

func TestWriteHTMLReport_BadPathReturnsError(t *testing.T) {
	err := writeHTMLReport("/this/path/definitely/does/not/exist/out.html", fixture(), false, time.Now())
	if err == nil {
		t.Fatal("expected error for unwritable path, got nil")
	}
}

func TestWriteHTMLReport_OracleWithScriptCloseDoesNotBreakHTML(t *testing.T) {
	// End-to-end version of the safeJSON test — a real card-shaped
	// result whose oracle text contains "</script>" must round-trip
	// through the embed without prematurely closing the inline script.
	dir := t.TempDir()
	path := filepath.Join(dir, "out.html")
	evil := []result{{
		Name:       "Eval the Magic",
		Class:      classEmptyAST,
		TypeLine:   "Sorcery",
		OracleText: "Stop the </script><img src=x> spell.",
	}}
	if err := writeHTMLReport(path, evil, false, time.Now()); err != nil {
		t.Fatalf("writeHTMLReport: %v", err)
	}
	html, _ := os.ReadFile(path)
	if strings.Contains(string(html), "</script><img") {
		t.Errorf("oracle text leaked verbatim into HTML, breaking the script tag:\n%s", html)
	}
	// The closing tag at the natural end of the inline script block
	// must still exist exactly once: tally `</script>` occurrences.
	if n := strings.Count(string(html), "</script>"); n != 1 {
		t.Errorf("expected exactly 1 </script> (the legitimate script close), got %d", n)
	}
}
