package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_prototype.go — CR §702.160 / §718 Prototype (The Brothers' War,
// 2022).
//
// "Prototype [cost] — [P/T]" is an alternative cost. A prototype card may be
// cast either normally (its full printed mana cost, color, and P/T) or
// "prototyped" — for the smaller prototype mana cost, in which case the spell
// and the permanent it becomes have the prototype's mana cost, color, and P/T
// while keeping its name, types, and abilities (CR §718.3b, §718.5). These
// alternative values are COPIABLE values (CR §718.2a) — a copy of a prototyped
// object copies the prototype stats (CR §718.3c/d).
//
// MODEL (7174n1c product decision — "reverse kicker", copiable, ALL zones):
// the prototype override is stored on the Card as a non-destructive
// PrototypeState (Card.Proto) that leaves the printed BasePower/BaseToughness/
// CMC/Colors fields untouched. Because the same *Card travels across zones,
// the reduced color + mana value are visible in EVERY zone — devotion,
// color-matters, and mana-value-matters payoffs all see the prototype values
// even off the battlefield. A normal cast clears Card.Proto, so the full
// printed version is restored.
//
// NOTE on the rules nuance: CR 718.4 restricts the prototype characteristics
// to the stack + battlefield (other zones show printed). HexDek deliberately
// applies them in ALL zones per the product decision above; this is an
// intentional divergence from 718.4 in the other-zone case, not an oversight.
//
// The dangerous in-place mutator described in the implementation plan
// (mechanic-prototype-r63.md §2) — which clobbered the shared Card's printed
// P/T/CMC and dropped color entirely — is deliberately NOT used. ApplyPrototype
// below is the safe non-destructive replacement.

// PrototypeState holds a prototype card's alternative (copiable) mana cost,
// mana value, color, and P/T. nil on Card.Proto means "not cast prototyped"
// (the normal printed version is in force).
type PrototypeState struct {
	ManaCostString string   // e.g. "{3}{R}{R}"
	CMC            int      // mana value of ManaCostString (CR §718.3a)
	Power          int      // alternative power
	Toughness      int      // alternative toughness
	Colors         []string // WUBRG letters derived from ManaCostString (CR §718.3b)
}

// clone deep-copies a PrototypeState (including its Colors slice) so a copied
// Card never shares the slice with its source.
func (s *PrototypeState) clone() *PrototypeState {
	if s == nil {
		return nil
	}
	cp := *s
	cp.Colors = append([]string(nil), s.Colors...)
	return &cp
}

// HasPrototype reports whether the card carries the prototype keyword.
func HasPrototype(card *Card) bool {
	return cardHasKeywordByName(card, "prototype")
}

// PrototypeProfile reads the prototype keyword's args and builds the
// PrototypeState. The engine-side arg contract is:
//
//	Args[0] = prototype mana-cost string ("{3}{R}{R}")  — MV + colors derived
//	Args[1] = prototype power
//	Args[2] = prototype toughness
//
// Tolerant fallbacks: Args[0] may instead be a bare number (legacy / parser
// gap) which is treated as the prototype CMC with no colored pips. Returns
// ok=false when the card has no prototype keyword or the args are missing, so
// the cast path declines to offer prototype rather than fabricating a value.
func PrototypeProfile(card *Card) (*PrototypeState, bool) {
	if !HasPrototype(card) || card == nil || card.AST == nil {
		return nil, false
	}
	var kw *gameast.Keyword
	for _, ab := range card.AST.Abilities {
		k, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k.Name), "prototype") {
			kw = k
			break
		}
	}
	if kw == nil || len(kw.Args) < 3 {
		return nil, false
	}

	st := &PrototypeState{}
	switch v := kw.Args[0].(type) {
	case string:
		st.ManaCostString = v
		st.CMC = cmcFromManaCostString(v)
		st.Colors = colorsFromManaCostString(v)
	case float64:
		st.CMC = int(v)
	case int:
		st.CMC = v
	default:
		return nil, false
	}
	st.Power = argToInt(kw.Args[1])
	st.Toughness = argToInt(kw.Args[2])
	return st, true
}

// ApplyPrototype sets the card's non-destructive prototype override from its
// keyword profile. This is the SAFE replacement for the plan's dangerous
// in-place mutator: it never touches the printed BasePower/BaseToughness/CMC/
// Colors. Returns true if the override was applied. Used by the cast path when
// the controller chooses to cast for prototype, and available to per-card /
// test callers that need to force the prototype mode.
func ApplyPrototype(card *Card) bool {
	profile, ok := PrototypeProfile(card)
	if !ok {
		return false
	}
	card.Proto = profile
	return true
}

// ClearPrototype removes any prototype override, restoring the printed
// version. Called on a normal cast of a prototype card.
func ClearPrototype(card *Card) {
	if card != nil {
		card.Proto = nil
	}
}

// argToInt coerces a JSON-decoded keyword arg (float64) or a raw int into an
// int. Returns 0 for anything else.
func argToInt(a interface{}) int {
	switch v := a.(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// cmcFromManaCostString computes the converted mana value of a Scryfall-style
// mana-cost string ("{3}{R}{R}" → 5). Generic {N} adds N; {X}/{S} add 0; every
// other single symbol (colored, hybrid, twobrid, Phyrexian) adds 1.
func cmcFromManaCostString(cost string) int {
	total := 0
	i := 0
	for i < len(cost) {
		if cost[i] != '{' {
			i++
			continue
		}
		end := strings.IndexByte(cost[i:], '}')
		if end < 0 {
			break
		}
		inner := strings.ToUpper(strings.TrimSpace(cost[i+1 : i+end]))
		i += end + 1
		switch {
		case inner == "" || inner == "X" || inner == "Y" || inner == "Z" || inner == "S":
			// X/variable and snow contribute 0 to a known MV.
		case isAllDigits(inner):
			if n, ok := atoiSafe(inner); ok {
				total += n
			}
		case strings.Contains(inner, "/"):
			// Hybrid / twobrid / Phyrexian: a single symbol = 1 MV. (Twobrid
			// {2/C} is printed as a 2-generic-or-one-colored symbol; its MV is
			// 2, but prototype costs don't use twobrid, so the +1 approximation
			// is harmless here and keeps the common colored-hybrid case right.)
			total++
		default:
			// A single colored or colorless mana symbol.
			total++
		}
	}
	return total
}

// colorsFromManaCostString returns the WUBRG color letters present in a
// mana-cost string, in canonical WUBRG order (CR §718.3b / §105.2). Colorless
// costs return nil.
func colorsFromManaCostString(cost string) []string {
	var out []string
	for _, letter := range DevotionColors { // W,U,B,R,G
		if DevotionPipsFromManaCost(cost, letter) > 0 {
			out = append(out, letter)
		}
	}
	return out
}
