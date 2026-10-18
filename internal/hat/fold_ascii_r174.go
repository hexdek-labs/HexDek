package hat

import (
	"strings"
	"unicode/utf8"
)

// r174 — allocation-free ASCII case folding for the hot evaluator
// helpers.
//
// A production CPU profile put strings.ToLower at 6.24% cumulative.
// Four of its top callers live in this package and all four were
// lowering a SHORT string (a card name or a type token) purely to
// feed an already-lowercase map key or needle:
//
//	fastManaExtraYield  15.73% of ToLower  — ToLower(DisplayName()) per battlefield permanent, per eval
//	typeLineContains     8.60%             — ToLower(type token) + ToLower(needle), ~40 call sites
//	handQualityBonus     7.93%             — ToLower(DisplayName()) per hand card, per eval
//	ritualNetMana        3.76%             — ToLower(DisplayName()) per hand card, per eval
//
// strings.ToLower has an ASCII fast path that returns s untouched when
// there is no uppercase byte, so an already-lowercase input costs only
// a scan — but a CARD NAME is mixed case by construction, so every one
// of those calls allocated a fresh string that was thrown away one map
// lookup later.
//
// The helpers below do the fold in place against a stack buffer (map
// key) or inline during the search (contains), and fall back to the
// exact original expression whenever either operand is not pure ASCII.
//
// EQUIVALENCE. For a pure-ASCII string, strings.ToLower maps each byte
// independently ('A'..'Z' -> +32, everything else unchanged); no byte
// is added, removed or merged. Therefore for pure-ASCII hay/needle:
//
//	strings.Contains(ToLower(hay), ToLower(needle))
//	  <=> exists i: forall j: fold(hay[i+j]) == fold(needle[j])
//
// and for a pure-ASCII name against a map whose keys are ASCII:
//
//	m[ToLower(name)] <=> m[string(foldBytes(name))]
//
// Both are exactly what these functions compute. Anything non-ASCII
// (accented or non-Latin printings — Lim-Dûl, Æther, Japanese alt-art
// names) takes the original code path verbatim, so the Unicode special
// cases that distinguish ToLower from simple ASCII folding (Kelvin sign
// U+212A, long s U+017F, dotted capital I U+0130) cannot be reached by
// the fast path.

// foldASCII lowercases one ASCII byte.
func foldASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// isASCIIStr reports whether s contains only bytes < utf8.RuneSelf.
func isASCIIStr(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// foldContainsASCII is strings.Contains(strings.ToLower(hay),
// strings.ToLower(needle)) without the two intermediate strings.
// Non-ASCII input on either side falls back to exactly that
// expression.
func foldContainsASCII(hay, needle string) bool {
	if !isASCIIStr(hay) || !isASCIIStr(needle) {
		return strings.Contains(strings.ToLower(hay), strings.ToLower(needle))
	}
	if len(needle) == 0 {
		return true
	}
	if len(needle) > len(hay) {
		return false
	}
	n0 := foldASCII(needle[0])
	last := len(hay) - len(needle)
outer:
	for i := 0; i <= last; i++ {
		if foldASCII(hay[i]) != n0 {
			continue
		}
		for j := 1; j < len(needle); j++ {
			if foldASCII(hay[i+j]) != foldASCII(needle[j]) {
				continue outer
			}
		}
		return true
	}
	return false
}

// foldKeyMax bounds the stack buffer used for the map-key fold. The
// longest key in either curated map is "basalt monolith" (15 bytes);
// 64 covers every real card name with room to spare and anything
// longer simply takes the allocating fallback.
const foldKeyMax = 64

// intByFoldedName is m[strings.ToLower(name)] with the lowercased key
// built in a stack buffer. The `m[string(b)]` form is recognised by the
// compiler and does not copy b to the heap, so the lookup allocates
// nothing. A missing key yields 0, exactly as the map index did.
//
// The fold is written inline rather than behind a helper so the buffer
// provably does not escape.
func intByFoldedName(m map[string]int, name string) int {
	if len(name) <= foldKeyMax {
		var buf [foldKeyMax]byte
		ascii := true
		for i := 0; i < len(name); i++ {
			c := name[i]
			if c >= utf8.RuneSelf {
				ascii = false
				break
			}
			buf[i] = foldASCII(c)
		}
		if ascii {
			return m[string(buf[:len(name)])]
		}
	}
	return m[strings.ToLower(name)]
}
