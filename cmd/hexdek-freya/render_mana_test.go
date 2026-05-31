package main

import (
	"strings"
	"testing"
)

// TestRenderMana_TextEmoji pins the text-mode emoji disc mapping for
// every canonical color symbol. These are the glyphs Discord +
// modern terminals render natively as colored circles.
func TestRenderMana_TextEmoji(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"{W}", "🟡"},
		{"{U}", "🔵"},
		{"{B}", "⚫"},
		{"{R}", "🔴"},
		{"{G}", "🟢"},
		{"{C}", "⚪"},
		// Lowercase normalizes to uppercase.
		{"{w}", "🟡"},
		// Tap symbol.
		{"{T}", "↻"},
	}
	for _, c := range cases {
		got := RenderMana(c.in, ManaText)
		if got != c.want {
			t.Errorf("RenderMana(%q, text) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestRenderMana_TextNonColorKeepsBraces pins the fallback path:
// generic mana ({2}), X-mana ({X}), hybrid pairs ({W/U}), and
// unknown letters keep their brace notation in text mode. Their
// emoji equivalents render inconsistently across terminals, so
// brace-text is the safer default.
func TestRenderMana_TextNonColorKeepsBraces(t *testing.T) {
	cases := []string{"{2}", "{X}", "{W/U}", "{2/W}", "{S}"}
	for _, c := range cases {
		got := RenderMana(c, ManaText)
		if got != c {
			t.Errorf("RenderMana(%q, text) = %q, want %q (unchanged)", c, got, c)
		}
	}
}

// TestRenderMana_InlineCost pins multi-symbol cost strings — a real
// known-combo description like "Pay {1}{U} to blink Drake" must
// render with the color symbol replaced but the generic kept.
func TestRenderMana_InlineCost(t *testing.T) {
	got := RenderMana("Pay {1}{U} to blink Drake", ManaText)
	want := "Pay {1}🔵 to blink Drake"
	if got != want {
		t.Errorf("RenderMana inline cost = %q, want %q", got, want)
	}
}

// TestRenderMana_MarkdownSameAsText pins the markdown-mode contract:
// emoji rendering is identical to text mode (Markdown viewers handle
// the emoji natively).
func TestRenderMana_MarkdownSameAsText(t *testing.T) {
	in := "Pay {R}{R} for an additional combat phase."
	if RenderMana(in, ManaMarkdown) != RenderMana(in, ManaText) {
		t.Errorf("markdown and text should produce identical output for color symbols")
	}
}

// TestRenderMana_HTMLEmitsSVG pins HTML-mode output: each color
// symbol becomes an inline <svg> with the color disc + letter. The
// SVG carries a class hook so a host page can restyle if needed.
func TestRenderMana_HTMLEmitsSVG(t *testing.T) {
	got := RenderMana("{W}{U}{B}{R}{G}", ManaHTML)
	for _, want := range []string{
		`<svg class="mana-symbol mana-svg"`,
		`aria-label="W mana"`,
		`aria-label="U mana"`,
		`aria-label="B mana"`,
		`aria-label="R mana"`,
		`aria-label="G mana"`,
		`<circle cx="10" cy="10" r="9"`,
		`>W</text>`,
		`>U</text>`,
		`>B</text>`,
		`>R</text>`,
		`>G</text>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML output missing %q\n---\n%s", want, got)
		}
	}
	// 5 SVGs for 5 symbols.
	if strings.Count(got, "</svg>") != 5 {
		t.Errorf("expected 5 </svg> closures, got %d", strings.Count(got, "</svg>"))
	}
}

// TestRenderMana_HTMLGenericFallback pins the grey-disc fallback for
// non-color symbols in HTML mode. Generic mana ({2}, {X}) still
// renders as an SVG — just with the grey palette — so the inline
// shape stays consistent across a mana cost.
func TestRenderMana_HTMLGenericFallback(t *testing.T) {
	got := RenderMana("{2}{X}", ManaHTML)
	if !strings.Contains(got, `aria-label="2 mana"`) {
		t.Errorf("HTML generic should label the digit: %q", got)
	}
	if !strings.Contains(got, `aria-label="X mana"`) {
		t.Errorf("HTML generic should label X: %q", got)
	}
	if !strings.Contains(got, `fill="#d3d3d3"`) {
		t.Errorf("HTML generic should use grey fill: %q", got)
	}
	if strings.Count(got, "</svg>") != 2 {
		t.Errorf("expected 2 SVGs (one per symbol), got %d", strings.Count(got, "</svg>"))
	}
}

// TestRenderMana_HTMLColorPalette pins the per-color fill values so a
// future palette tweak surfaces here, not as a "mana symbols look
// off" report from a downstream consumer.
func TestRenderMana_HTMLColorPalette(t *testing.T) {
	cases := []struct {
		sym  string
		fill string
	}{
		{"{W}", "#f8e7b9"},
		{"{U}", "#b3ceea"},
		{"{B}", "#a69f9d"},
		{"{R}", "#eb9f82"},
		{"{G}", "#c4d3ca"},
		{"{C}", "#d3d3d3"},
	}
	for _, c := range cases {
		got := RenderMana(c.sym, ManaHTML)
		if !strings.Contains(got, "fill=\""+c.fill+"\"") {
			t.Errorf("color %s should use fill=%s; got %q", c.sym, c.fill, got)
		}
	}
}

// TestRenderManaHTMLSafe_EscapesText pins the HTML-safe combined
// path: non-mana text MUST be HTML-escaped while mana SVGs pass
// through raw. This is the contract the printHTML combo renderer
// relies on — naively chaining html.EscapeString around
// RenderMana(..., ManaHTML) would produce literal &lt;svg&gt; text.
func TestRenderManaHTMLSafe_EscapesText(t *testing.T) {
	in := "Pay {R}{R} for an extra combat (5 + R + R cost) <attack>"
	got := RenderManaHTMLSafe(in)
	// The mana SVGs render raw.
	if !strings.Contains(got, "<svg") {
		t.Errorf("expected raw <svg> in HTML-safe output: %q", got)
	}
	if !strings.Contains(got, "aria-label=\"R mana\"") {
		t.Errorf("expected R mana SVG: %q", got)
	}
	// The literal "<attack>" must be escaped.
	if !strings.Contains(got, "&lt;attack&gt;") {
		t.Errorf("expected raw HTML to be escaped: %q", got)
	}
	if strings.Contains(got, "<attack>") {
		t.Errorf("unescaped <attack> would be an injection bug: %q", got)
	}
	// Parens and digits are passed through (not HTML-special).
	if !strings.Contains(got, "(5 + R + R cost)") {
		t.Errorf("parenthesized text should survive: %q", got)
	}
}

// TestRenderManaHTMLSafe_ApostropheCard pins the "Yawgmoth's Will"
// type case — descriptions referencing card names with apostrophes
// must escape the apostrophe properly so the surrounding HTML
// attributes stay well-formed.
func TestRenderManaHTMLSafe_ApostropheCard(t *testing.T) {
	in := "Sensei's Divining Top + {1}: redraw the top of the library"
	got := RenderManaHTMLSafe(in)
	if strings.Contains(got, "Sensei's") {
		t.Errorf("apostrophe should be HTML-escaped: %q", got)
	}
	if !strings.Contains(got, "Sensei&#39;s") {
		t.Errorf("expected escaped apostrophe: %q", got)
	}
	// And the {1} still rendered as an SVG.
	if !strings.Contains(got, "aria-label=\"1 mana\"") {
		t.Errorf("expected {1} → SVG: %q", got)
	}
}

// TestRenderManaHTMLSafe_NoTokensIsEscape pins the no-mana path:
// a description with no `{X}` tokens passes through pure html.Escape.
func TestRenderManaHTMLSafe_NoTokensIsEscape(t *testing.T) {
	got := RenderManaHTMLSafe("plain text with <html> & special chars")
	want := "plain text with &lt;html&gt; &amp; special chars"
	if got != want {
		t.Errorf("RenderManaHTMLSafe (no tokens) = %q, want %q", got, want)
	}
}

// TestRenderMana_EmptyInput pins the empty-input defensive path: all
// three modes must return "" for "" without panicking.
func TestRenderMana_EmptyInput(t *testing.T) {
	for _, mode := range []ManaRenderMode{ManaText, ManaMarkdown, ManaHTML} {
		if got := RenderMana("", mode); got != "" {
			t.Errorf("RenderMana(\"\", %d) = %q, want empty", mode, got)
		}
	}
	if got := RenderManaHTMLSafe(""); got != "" {
		t.Errorf("RenderManaHTMLSafe(\"\") = %q, want empty", got)
	}
}

// TestRenderManaForFormat pins the dispatcher: a caller with the
// format string ("text" / "markdown" / "html") gets the right mode
// without reimplementing the switch.
func TestRenderManaForFormat(t *testing.T) {
	in := "{R} for damage"
	if renderManaForFormat(in, "text") != RenderMana(in, ManaText) {
		t.Error("text format should match ManaText")
	}
	if renderManaForFormat(in, "markdown") != RenderMana(in, ManaMarkdown) {
		t.Error("markdown format should match ManaMarkdown")
	}
	if renderManaForFormat(in, "html") != RenderMana(in, ManaHTML) {
		t.Error("html format should match ManaHTML")
	}
	// Unknown formats fall back to text mode.
	if renderManaForFormat(in, "yaml-not-real") != RenderMana(in, ManaText) {
		t.Error("unknown format should fall back to text")
	}
}

// TestRenderMana_RealKnownCombo runs the helper on actual combo
// description strings from known_combos.go so a real downstream
// consumer can rely on the contract. These are the exact strings
// users see in the freya text output.
func TestRenderMana_RealKnownCombo(t *testing.T) {
	cases := []struct {
		name string
		in   string
		// substrings the rendered output must contain (text mode)
		wantTextHas []string
	}{
		{
			name: "Bear Umbra + Hellkite Charger",
			in:   "Attack with Hellkite Charger → Bear Umbra untaps lands. Pay {5}{R}{R} for an additional combat.",
			wantTextHas: []string{
				"{5}🔴🔴", // {5} keeps braces; R,R become emoji
				"additional combat",
			},
		},
		{
			name: "Soulbond Deadeye + Drake",
			in:   "Pay {1}{U} to blink Drake → untap 5 lands → net +3 mana per loop.",
			wantTextHas: []string{
				"{1}🔵", // {1} keeps braces; U becomes emoji
				"net +3 mana per loop",
			},
		},
		{
			name: "Gravecrawler",
			in:   "Sac Gravecrawler for {B}, recast from graveyard.",
			wantTextHas: []string{
				"⚫",
				"recast from graveyard",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RenderMana(c.in, ManaText)
			for _, want := range c.wantTextHas {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in rendered output\n---\n%s", want, got)
				}
			}
		})
	}
}

// TestManaTokenRE_DoesNotMatchBraceJSON pins the regex shape against
// a false-positive risk: braces in JSON / structured text. The
// tokenizer matches `{[A-Za-z0-9/]+}` so a JSON object like
// `{"name": ...}` (with a quote and colon inside) is NOT picked up.
func TestManaTokenRE_DoesNotMatchBraceJSON(t *testing.T) {
	in := `{"name": "Bolt", "cost": "{R}"}`
	got := RenderMana(in, ManaText)
	// The JSON object braces themselves shouldn't be touched — they
	// contain a colon + quote, neither of which is in the character
	// class. But the inner "{R}" (within the JSON string value) IS a
	// real mana symbol token from the tokenizer's POV. This test
	// pins that the OUTER {…} braces survive verbatim.
	if !strings.Contains(got, `{"name":`) {
		t.Errorf("outer JSON braces shouldn't be matched as mana: %q", got)
	}
	// And the inner {R} still becomes an emoji.
	if !strings.Contains(got, "🔴") {
		t.Errorf("inner {R} should still render: %q", got)
	}
}
