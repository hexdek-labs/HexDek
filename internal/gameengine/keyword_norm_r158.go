package gameengine

import (
	"strings"
	"unicode/utf8"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r158 — (*Permanent).HasKeyword is 12.48% cum of all CPU on the r157
// binary, the largest single named function in the engine. A -list
// attribution of a 30s production profile put the cost here:
//
//	line 58  want := ToLower(TrimSpace(name))        950ms
//	line 61  range p.Card.AST.Abilities              170ms
//	line 62  type-assert *gameast.Keyword            180ms
//	line 63  ToLower(TrimSpace(kw.Name)) == want     820ms
//	line 81  p.Flags["kw:"+want]                    2510ms   <-- NOT THIS ROUND
//	line 94  p.Counters[want]                        390ms
//
// This file addresses the 2.12s across 58/61/62/63 and deliberately
// leaves line 81 alone. Line 81 is a string concatenation plus a map
// hash per call and is the biggest single line, but removing it means
// either moving 135 production `Flags["kw:"...]` sites to a separate
// map or caching HasKeyword's answer — and the answer depends on
// GrantedAbilities, Flags and Counters, all of which mutate at runtime
// with no epoch of their own. r157 is the standing warning against
// building that on an invalidation nobody maintains. Measured and left,
// not overlooked.

// normalizeKeyword returns strings.ToLower(strings.TrimSpace(s)) — but
// returns s itself, with no scan past the first offending byte and no
// allocation, when the string is already trimmed and lowercase ASCII.
// That is the overwhelmingly common case: callers pass compile-time
// literals like "flying", and the corpus stores keyword names the same
// way.
//
// Interior spaces are left alone ("first strike" is one keyword), so
// only the first and last bytes are checked for whitespace.
func normalizeKeyword(s string) string {
	if len(s) == 0 {
		return s
	}
	if isASCIISpace(s[0]) || isASCIISpace(s[len(s)-1]) {
		return strings.ToLower(strings.TrimSpace(s))
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Any uppercase ASCII needs lowering; any non-ASCII byte may be
		// part of a rune that ToLower or TrimSpace would change
		// (U+0085, U+00A0 are space; many runes have lowercase forms),
		// so hand those to the real implementation rather than guess.
		if (c >= 'A' && c <= 'Z') || c >= utf8.RuneSelf {
			return strings.ToLower(strings.TrimSpace(s))
		}
	}
	return s
}

// isASCIISpace mirrors the ASCII half of unicode.IsSpace, which is what
// strings.TrimSpace trims. The non-ASCII half is handled by the
// utf8.RuneSelf check in normalizeKeyword.
func isASCIISpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

// astKeywordNames returns the normalized names of every Keyword ability
// on the card's CURRENT AST, computing them once per AST.
//
// The cache is keyed on the AST pointer, not on a "have we done this"
// bool — a DFC transform reassigns Card.AST in place, and r157 is the
// round that got caught by exactly that. Keying on the pointer means a
// transformed card recomputes by construction.
//
// A card with no keyword abilities caches a nil slice and still records
// the AST, so the scan happens once rather than on every call.
func astKeywordNames(c *Card) []string {
	if c == nil || c.AST == nil {
		return nil
	}
	if c.kwNamesAST == c.AST {
		return c.kwNames
	}
	var out []string
	for _, ab := range c.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok {
			out = append(out, normalizeKeyword(kw.Name))
		}
	}
	c.kwNames = out
	c.kwNamesAST = c.AST
	return out
}
