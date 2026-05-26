package main

import (
	"math"
	"sort"
	"strings"
)

// Discrepancy categories. Adding a new one? Update CategoryOrder so the
// markdown report keeps a deterministic section order.
const (
	CatMissingInOracle    = "missing_in_oracle"
	CatOracleTextDrift    = "oracle_text_drift"
	CatTypeLineDrift      = "type_line_drift"
	CatCMCDrift           = "cmc_drift"
	CatManaCostDrift      = "mana_cost_drift"
	CatKeywordHallucinate = "ast_keyword_hallucination"
)

// CategoryOrder controls section order in the rendered report.
var CategoryOrder = []string{
	CatMissingInOracle,
	CatOracleTextDrift,
	CatTypeLineDrift,
	CatCMCDrift,
	CatManaCostDrift,
	CatKeywordHallucinate,
}

// astEntry mirrors one line of data/rules/ast_dataset.jsonl. The AST
// payload itself is captured as decoded any so the keyword-walk runs
// on the parsed structure without a second pass.
type astEntry struct {
	Name       string  `json:"name"`
	OracleText string  `json:"oracle_text"`
	TypeLine   string  `json:"type_line"`
	ManaCost   string  `json:"mana_cost"`
	CMC        float64 `json:"cmc"`
	AST        any     `json:"ast"`
}

// oracleEntry mirrors one entry of data/rules/oracle-cards.json. Multi-
// face cards (modal DFCs, adventures, transform) carry per-face oracle
// text in CardFaces; the top-level OracleText is empty in that case.
// Use canonicalOracleText / canonicalTypeLine / canonicalManaCost to
// get the comparable single-string view.
type oracleEntry struct {
	Name       string  `json:"name"`
	OracleText string  `json:"oracle_text"`
	TypeLine   string  `json:"type_line"`
	ManaCost   string  `json:"mana_cost"`
	CMC        float64 `json:"cmc"`
	Layout     string  `json:"layout"`
	SetName    string  `json:"set_name"`
	CardFaces  []struct {
		OracleText string `json:"oracle_text"`
		ManaCost   string `json:"mana_cost"`
		TypeLine   string `json:"type_line"`
	} `json:"card_faces"`
}

// Discrepancy is one finding tied to one card.
type Discrepancy struct {
	Name     string
	Category string
	// Details is a short, human-readable note explaining the finding
	// (the keyword that's hallucinated, a snippet of the drift, etc.).
	Details string
}

// canonicalOracleText returns the oracle text to compare against the
// AST dataset's oracle_text field. For multi-face cards, joins each
// face's text with " // " mirroring Scryfall's printed-card convention.
func canonicalOracleText(o oracleEntry) string {
	if strings.TrimSpace(o.OracleText) != "" {
		return o.OracleText
	}
	if len(o.CardFaces) == 0 {
		return ""
	}
	parts := make([]string, 0, len(o.CardFaces))
	for _, f := range o.CardFaces {
		parts = append(parts, f.OracleText)
	}
	return strings.Join(parts, " // ")
}

// canonicalTypeLine returns the type line for comparison. Most multi-
// face cards have a top-level type_line that already joins both faces
// with " // " (e.g., "Creature — Faerie Rogue // Sorcery — Adventure"),
// so we use it as-is when present.
func canonicalTypeLine(o oracleEntry) string {
	if strings.TrimSpace(o.TypeLine) != "" {
		return o.TypeLine
	}
	if len(o.CardFaces) == 0 {
		return ""
	}
	parts := make([]string, 0, len(o.CardFaces))
	for _, f := range o.CardFaces {
		parts = append(parts, f.TypeLine)
	}
	return strings.Join(parts, " // ")
}

// canonicalManaCost returns the mana cost. Same shape as type_line:
// top-level when present, joined card-face costs otherwise.
func canonicalManaCost(o oracleEntry) string {
	if strings.TrimSpace(o.ManaCost) != "" {
		return o.ManaCost
	}
	if len(o.CardFaces) == 0 {
		return ""
	}
	parts := make([]string, 0, len(o.CardFaces))
	for _, f := range o.CardFaces {
		parts = append(parts, f.ManaCost)
	}
	return strings.Join(parts, " // ")
}

// normalizeText lowercases and collapses every run of whitespace
// (spaces, tabs, newlines, CR) into a single space. This is more
// aggressive than preserving paragraph structure, but it's the right
// shape for drift detection — we want substantive word changes to
// surface, not line-break differences. Reminder-text edits and real
// errata still produce drift findings; CRLF vs LF, double newlines,
// trailing whitespace are filtered out as trivial.
func normalizeText(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true // start as space so leading whitespace is skipped
	for i := 0; i < len(s); i++ {
		c := s[i]
		isSpace := c == ' ' || c == '\t' || c == '\n' || c == '\r'
		if isSpace {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteByte(c)
		prevSpace = false
	}
	out := b.String()
	return strings.TrimRight(out, " ")
}

// extractASTKeywords walks the decoded AST tree and collects every
// `{"__ast_type__": "Keyword", "name": "..."}` node's name. Used by
// checkASTHallucination to find keyword references the parser
// produced that no longer appear in the current oracle text.
//
// The walker is generic over the decoded JSON shape (map vs slice vs
// primitive) so it handles nested abilities, modification args, and
// trigger effects uniformly without re-implementing the AST schema.
func extractASTKeywords(node any) []string {
	var out []string
	walkAST(node, func(v map[string]any) {
		if t, _ := v["__ast_type__"].(string); t == "Keyword" {
			if name, _ := v["name"].(string); strings.TrimSpace(name) != "" {
				out = append(out, name)
			}
		}
	})
	// Dedupe so a Saga with 3 chapter-mentions of "scry" doesn't
	// triple-count.
	sort.Strings(out)
	last := ""
	deduped := out[:0]
	for _, k := range out {
		if k != last {
			deduped = append(deduped, k)
			last = k
		}
	}
	return deduped
}

// walkAST invokes visit on every map[string]any encountered in the
// tree (including the root, if it's a map). Other primitive types
// (strings, numbers, nils) are skipped. Used by extractASTKeywords
// and reusable by future audits that need to scan node kinds.
func walkAST(node any, visit func(map[string]any)) {
	switch x := node.(type) {
	case map[string]any:
		visit(x)
		for _, v := range x {
			walkAST(v, visit)
		}
	case []any:
		for _, v := range x {
			walkAST(v, visit)
		}
	}
}

// checkOracleTextDrift returns a discrepancy when the AST entry's
// cached oracle_text differs from the current oracle corpus's text
// (after whitespace normalization). The Details snippet shows the
// first divergent paragraph so the reader can triage by reading the
// report alone.
func checkOracleTextDrift(a astEntry, o oracleEntry) *Discrepancy {
	want := normalizeText(canonicalOracleText(o))
	got := normalizeText(a.OracleText)
	if want == got {
		return nil
	}
	return &Discrepancy{
		Name:     a.Name,
		Category: CatOracleTextDrift,
		Details:  firstDivergence(got, want),
	}
}

// checkTypeLineDrift / checkCMCDrift / checkManaCostDrift mirror the
// oracle-text shape for the lighter-weight metadata fields.
func checkTypeLineDrift(a astEntry, o oracleEntry) *Discrepancy {
	want := strings.TrimSpace(canonicalTypeLine(o))
	got := strings.TrimSpace(a.TypeLine)
	if want == got {
		return nil
	}
	return &Discrepancy{
		Name:     a.Name,
		Category: CatTypeLineDrift,
		Details:  "AST=" + got + " | oracle=" + want,
	}
}

func checkCMCDrift(a astEntry, o oracleEntry) *Discrepancy {
	// CMC is float per Scryfall (X-cost cards report 0). A tolerance
	// of 1e-6 avoids float-encoding round-trip noise (3.0 vs 3.0000001
	// could otherwise drop in).
	if math.Abs(a.CMC-o.CMC) < 1e-6 {
		return nil
	}
	return &Discrepancy{
		Name:     a.Name,
		Category: CatCMCDrift,
		Details:  cmcStr(a.CMC) + " (AST) vs " + cmcStr(o.CMC) + " (oracle)",
	}
}

func checkManaCostDrift(a astEntry, o oracleEntry) *Discrepancy {
	want := strings.TrimSpace(canonicalManaCost(o))
	got := strings.TrimSpace(a.ManaCost)
	if want == got {
		return nil
	}
	return &Discrepancy{
		Name:     a.Name,
		Category: CatManaCostDrift,
		Details:  "AST=" + got + " | oracle=" + want,
	}
}

// checkASTHallucination: every Keyword node in the AST whose name
// doesn't appear (case-insensitive substring) in the CURRENT oracle
// text is a parser hallucination — the parser produced a structured
// ability reference that the printed card doesn't actually have.
//
// Conservative on purpose: returns nil for keywords we can't reliably
// detect via simple substring (those that wrap a deeper template). The
// "skipList" carries cases where the keyword name is a fragment / abbreviation
// that the oracle text never spells out verbatim — false positives we'd
// have to filter anyway. Better to under-report than swamp the report.
func checkASTHallucination(a astEntry, oracleTxt string) *Discrepancy {
	keywords := extractASTKeywords(a.AST)
	if len(keywords) == 0 {
		return nil
	}
	lowOracle := strings.ToLower(oracleTxt)
	var hallucinated []string
	for _, k := range keywords {
		kw := strings.ToLower(strings.TrimSpace(k))
		if kw == "" || keywordSkipList[kw] {
			continue
		}
		if !strings.Contains(lowOracle, kw) {
			hallucinated = append(hallucinated, k)
		}
	}
	if len(hallucinated) == 0 {
		return nil
	}
	return &Discrepancy{
		Name:     a.Name,
		Category: CatKeywordHallucinate,
		Details:  "AST keywords absent from oracle: " + strings.Join(hallucinated, ", "),
	}
}

// keywordSkipList covers Keyword.name values that are parser-internal
// shorthand (not the printed word). The parser-coverage tooling uses
// the same list — these aren't hallucinations, they're aliases.
var keywordSkipList = map[string]bool{
	// Parser-internal mechanics-tag aliases that don't appear verbatim
	// in oracle text:
	"first_strike":  true,
	"double_strike": true,
	"basic_land":    true,
	"snow_land":     true,
	// "evergreen" / placeholder aliases used to mark inherited keywords
	// (creature types that imply but don't print keywords).
	"placeholder": true,
	"unknown":     true,
}

// firstDivergence returns a one-line snippet of `got` showing the
// first ~100 chars after the byte where it diverges from `want`. Both
// inputs are pre-normalized lowercase; the output is what makes the
// drift report readable at a glance.
func firstDivergence(got, want string) string {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	div := 0
	for div < n && got[div] == want[div] {
		div++
	}
	// Show a window starting at div with up to 100 chars from got,
	// and the corresponding stretch from want, separated by " | ".
	end := div + 100
	if end > len(got) {
		end = len(got)
	}
	gSnip := got[div:end]
	endW := div + 100
	if endW > len(want) {
		endW = len(want)
	}
	wSnip := want[div:endW]
	return "AST≠oracle starting at byte " + itoa(div) + ": ast=" + truncate(gSnip, 60) + " | oracle=" + truncate(wSnip, 60)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

func cmcStr(c float64) string {
	if c == math.Trunc(c) {
		return itoa(int(c))
	}
	// Two-decimal display covers Scryfall's X-cost edge cases.
	return strings.TrimRight(strings.TrimRight(strings.TrimSpace(formatFloat(c, 2)), "0"), ".")
}

func formatFloat(f float64, prec int) string {
	// Minimal localized FormatFloat to avoid importing strconv (kept
	// out so the package's runtime surface is tighter).
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "?"
	}
	neg := f < 0
	if neg {
		f = -f
	}
	mul := math.Pow(10, float64(prec))
	scaled := math.Round(f * mul)
	intPart := int(scaled / mul)
	frac := int(scaled - float64(intPart)*mul)
	out := itoa(intPart)
	if prec > 0 {
		out += "."
		// Left-pad the fractional digits with zeros to prec width.
		fStr := itoa(frac)
		for len(fStr) < prec {
			fStr = "0" + fStr
		}
		out += fStr
	}
	if neg {
		out = "-" + out
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
