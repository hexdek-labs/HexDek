package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// render_mana — single helper that turns the raw `{W}{U}{B}{R}{G}` style
// mana cost notation embedded in Freya output strings (combo
// descriptions, known-combo blurbs, oracle quotes) into format-
// appropriate glyphs:
//
//   - Text mode (default --format text): Unicode emoji color discs
//     (🟡 W, 🔵 U, ⚫ B, 🔴 R, 🟢 G, ⚪ C). Generic mana / X / T fall
//     back to brace notation since their emoji equivalents render
//     inconsistently across terminals. The emoji discs themselves
//     render natively in modern terminals (iTerm2, Alacritty, GNOME
//     Terminal, Windows Terminal, Discord).
//
//   - Markdown mode (--format markdown): same emoji discs. Discord +
//     GitHub + Reddit all render the Unicode emoji natively.
//
//   - HTML mode (--format html): proper inline SVG with a circle +
//     letter, color-matched to the MTG palette. The SVG width/height
//     use `1em` so the symbol scales with surrounding font size; no
//     external CSS dependency, no font dependency, no remote fetches
//     — the report stays single-file-hostable.
//
// All three modes share one tokenizer (manaTokenRE) so the substring
// surface of "what counts as a mana symbol" lives in one place.

// manaTokenRE matches `{X}` mana symbols. The inner content can be a
// single letter (color, T, X), a digit (generic mana), a multi-letter
// keyword (S = snow, P = phyrexian — though those appear with slashes
// like `{W/P}` which the slash class catches), or a slash-pair hybrid
// like `{W/U}` or `{2/W}`. Captures the inner content for dispatch.
var manaTokenRE = regexp.MustCompile(`\{([A-Za-z0-9/]+)\}`)

// ManaRenderMode picks the output flavor.
type ManaRenderMode int

const (
	// ManaText renders for the default text terminal output: emoji
	// discs for WUBRGC color symbols, brace notation for everything
	// else.
	ManaText ManaRenderMode = iota
	// ManaMarkdown renders for `--format markdown`: same emoji
	// strategy as text mode (Markdown viewers render Unicode emoji
	// natively).
	ManaMarkdown
	// ManaHTML renders for `--format html`: inline SVG with the MTG
	// color palette. Generic mana renders as a grey disc with the
	// digit inside.
	ManaHTML
)

// RenderMana scans a string for `{...}` mana symbol tokens and
// replaces each one with the mode-appropriate glyph. Non-token text
// passes through unchanged. Empty input → empty output.
//
// Examples:
//
//	RenderMana("Pay {R}{R} for damage", ManaText)
//	  → "Pay 🔴🔴 for damage"
//	RenderMana("Cost: {2}{U}{B}", ManaHTML)
//	  → "Cost: <svg ...>2</svg><svg ...>U</svg><svg ...>B</svg>"
//	RenderMana("Cycling {2}", ManaText)
//	  → "Cycling {2}"   (generic mana falls back to brace notation)
func RenderMana(s string, mode ManaRenderMode) string {
	if s == "" {
		return s
	}
	return manaTokenRE.ReplaceAllStringFunc(s, func(token string) string {
		m := manaTokenRE.FindStringSubmatch(token)
		if len(m) < 2 {
			return token
		}
		inner := strings.ToUpper(m[1])
		return renderManaSymbol(inner, mode)
	})
}

// renderManaSymbol resolves a single symbol (e.g. "W", "2", "X",
// "W/U") for the given mode. Color symbols use the rendering tables;
// generic / X / T / hybrid / unknown symbols fall back to brace
// notation (text/markdown) or a generic-grey SVG (HTML).
func renderManaSymbol(sym string, mode ManaRenderMode) string {
	switch mode {
	case ManaText, ManaMarkdown:
		if emoji, ok := manaEmojis[sym]; ok {
			return emoji
		}
		return "{" + sym + "}"
	case ManaHTML:
		if svg, ok := manaSVGs[sym]; ok {
			return svg
		}
		return genericManaSVG(sym)
	}
	return "{" + sym + "}"
}

// manaEmojis maps the canonical mana letters to Unicode emoji discs.
// The disc palette mirrors community convention (white = yellow disc,
// blue = blue disc, black = black disc, red = red disc, green = green
// disc, colorless = white disc). T = tap (clockwise arrow).
var manaEmojis = map[string]string{
	"W": "🟡",
	"U": "🔵",
	"B": "⚫",
	"R": "🔴",
	"G": "🟢",
	"C": "⚪",
	"T": "↻",
}

// manaSVGs holds prebaked inline SVG strings for the six canonical
// mana colors. Each is a 1em-tall circle with the letter inside.
// Generic / hybrid / unknown symbols are rendered on demand by
// genericManaSVG.
var manaSVGs = map[string]string{
	"W": buildManaSVG("W", "#f8e7b9", "#222"),
	"U": buildManaSVG("U", "#b3ceea", "#222"),
	"B": buildManaSVG("B", "#a69f9d", "#222"),
	"R": buildManaSVG("R", "#eb9f82", "#222"),
	"G": buildManaSVG("G", "#c4d3ca", "#222"),
	"C": buildManaSVG("C", "#d3d3d3", "#222"),
}

// buildManaSVG produces an inline SVG string for one mana symbol. The
// SVG is self-contained — no <defs>, no class hooks required for
// rendering (a `mana-symbol mana-svg` CSS hook is still attached so a
// host page can restyle if desired).
//
// 1em width/height makes the symbol scale with surrounding font size;
// the viewBox 0 0 20 20 + circle r=9 leaves a 1px stroke margin so
// adjacent symbols don't overlap when rendered inline.
func buildManaSVG(label, fill, textColor string) string {
	return fmt.Sprintf(
		`<svg class="mana-symbol mana-svg" viewBox="0 0 20 20" width="1em" height="1em" `+
			`role="img" aria-label="%s mana">`+
			`<circle cx="10" cy="10" r="9" fill="%s" stroke="#222" stroke-width="0.7"/>`+
			`<text x="10" y="14.2" text-anchor="middle" font-family="sans-serif" `+
			`font-size="11" font-weight="bold" fill="%s">%s</text>`+
			`</svg>`,
		label, fill, textColor, label)
}

// genericManaSVG produces an on-demand SVG for symbols outside the
// canonical color set — generic digits ({0}-{9}), {X}, hybrid pairs
// ({W/U}), Phyrexian ({W/P}). Renders as a grey disc with the symbol
// inside. Used for cost notation like "{2}{U}{B}" where the leading
// generic must still render as a symbol-like badge.
func genericManaSVG(sym string) string {
	// Hybrid like W/U is wider than a single letter — keep the SVG
	// shape constant but let the text squeeze; SVG handles overflow
	// gracefully via the central anchor.
	return buildManaSVG(sym, "#d3d3d3", "#222")
}

// RenderManaHTMLSafe renders a string for HTML output where the
// non-mana text MUST be HTML-escaped (defensive against card names
// like "Yawgmoth's Will" / descriptions with `<`, `>`, `&`) but mana
// SVGs MUST pass through raw. RenderMana(s, ManaHTML) alone is unsafe
// to feed through html.EscapeString — that would turn the SVG tags
// into literal `&lt;svg…` text. This helper splits the string at mana
// token boundaries so text chunks get escaped and SVG chunks don't.
//
// Callers in the HTML renderer should use this instead of
// `html.EscapeString(RenderMana(...))` or `RenderMana(html.EscapeString(...))`
// — both of those produce broken output.
func RenderManaHTMLSafe(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	idx := 0
	for _, loc := range manaTokenRE.FindAllStringIndex(s, -1) {
		// Text before this token — escape.
		if loc[0] > idx {
			b.WriteString(html.EscapeString(s[idx:loc[0]]))
		}
		// The token itself — render as SVG, emit raw.
		token := s[loc[0]:loc[1]]
		m := manaTokenRE.FindStringSubmatch(token)
		if len(m) >= 2 {
			b.WriteString(renderManaSymbol(strings.ToUpper(m[1]), ManaHTML))
		} else {
			// Defensive — shouldn't be reachable since the loc came
			// from the same regex, but if it does, escape the raw
			// token to stay safe.
			b.WriteString(html.EscapeString(token))
		}
		idx = loc[1]
	}
	// Tail after the last token.
	if idx < len(s) {
		b.WriteString(html.EscapeString(s[idx:]))
	}
	return b.String()
}

// renderManaForFormat is a thin dispatcher used by report renderers
// that already know their format string ("text" / "markdown" / "html").
// Keeps the format-string -> ManaRenderMode mapping centralized so
// callers don't reimplement the switch.
func renderManaForFormat(s, format string) string {
	switch format {
	case "markdown":
		return RenderMana(s, ManaMarkdown)
	case "html":
		return RenderMana(s, ManaHTML)
	default:
		return RenderMana(s, ManaText)
	}
}
