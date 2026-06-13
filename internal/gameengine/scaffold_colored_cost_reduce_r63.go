package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// scaffold_colored_cost_reduce_r63.go — generic handler for the inert
// `colored_cost_reduce` AST scaffold KIND (r63 scaffold-kind coverage,
// ~99 cards): a battlefield permanent with a static
//
//	"<filter> spells you cast cost {N} less to cast."
//
// e.g. Oketra's Monument (white creature, {1}), Mocking Sprite (instant
// and sorcery, {1}), Brighthearth Banneret (elemental + warrior, {1}),
// Undead Warchief-style tribal reducers, the medallions, etc. The AST
// emits Modification{kind:"colored_cost_reduce", args:[filterStr, "{N}"]}.
//
// Before this, only the ~20 reducers with a NAMED case in
// ScanCostModifiersWithContext's battlefield switch worked; every other
// card carrying colored_cost_reduce was inert (the static was logged and
// dropped). This handler is wired as the switch's `default:` branch, so it
// fires ONLY for permanents whose name has no dedicated case — the named
// reducers (medallions, Goblin Electromancer, …) keep their precise
// handling and are never double-counted.
//
// Conservatism: the filter is matched by a strict vocabulary (colors,
// card types, plus a subtype fallback that only matches when the spell
// genuinely has that subtype). If the amount isn't a plain {N} the
// reducer is skipped. A reduction is only ever produced for spells the
// reducer's controller is casting ("you cast"), and only when the spell
// matches the filter — so an unparseable or non-matching filter yields no
// modifier (the card stays inert, never a wrong reduction).

// appendColoredCostReduce inspects perm's AST for colored_cost_reduce
// static abilities and, when perm's controller (seatIdx) is casting a
// matching spell, appends a CostModReduction. Returns the (possibly
// extended) mods slice. perm is assumed to have no dedicated name-case in
// the caller's switch (it is invoked from the switch default).
func appendColoredCostReduce(mods []CostModifier, card *Card, perm *Permanent, seatIdx int) []CostModifier {
	if card == nil || perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return mods
	}
	// "spells you cast" — only the reducer's own controller benefits.
	if perm.Controller != seatIdx {
		return mods
	}
	for _, ab := range perm.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "colored_cost_reduce" {
			continue
		}
		args := st.Modification.Args
		if len(args) < 2 {
			continue
		}
		filter, _ := args[0].(string)
		amtStr, _ := args[1].(string)
		amount := parseBraceAmount(amtStr)
		if filter == "" || amount <= 0 {
			continue
		}
		if !cardMatchesCostFilter(card, filter) {
			continue
		}
		mods = append(mods, CostModifier{
			Kind:   CostModReduction,
			Amount: amount,
			Source: perm.Card.DisplayName() + " (colored_cost_reduce)",
		})
	}
	return mods
}

// parseBraceAmount turns "{2}" into 2. Returns 0 for non-numeric / {X}
// forms (those reducers are skipped — we never guess a variable amount).
func parseBraceAmount(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// costFilterCardTypes is the set of words treated as card types in a
// cost-reduce filter. Anything not a color or a card type is tried as a
// subtype (a safe, genuine membership check).
var costFilterCardTypes = map[string]bool{
	"creature": true, "artifact": true, "enchantment": true,
	"instant": true, "sorcery": true, "planeswalker": true,
	"land": true, "battle": true, "kindred": true, "tribal": true,
}

var costFilterColors = map[string]string{
	"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G",
}

// cardMatchesCostFilter reports whether card matches a colored_cost_reduce
// filter descriptor. Grammar (conservative):
//   - " and " separates ALTERNATIVES (OR): "instant and sorcery",
//     "elemental and warrior", "artifact and enchantment".
//   - within an alternative, space-separated words are conjuncts (AND):
//     "white creature" = white AND creature.
//   - each word is a color, a card type, "noncreature", or (fallback) a
//     subtype. Noise words ("spells", "you", "cast", "cost", "to") are
//     dropped.
// A card matches if it satisfies ANY alternative. Unrecognized words fall
// through to a subtype check, which fails closed (no false reduction).
func cardMatchesCostFilter(card *Card, descriptor string) bool {
	d := strings.ToLower(strings.TrimSpace(descriptor))
	for _, alt := range strings.Split(d, " and ") {
		if costAlternativeMatches(card, alt) {
			return true
		}
	}
	return false
}

var costFilterNoise = map[string]bool{
	"spells": true, "spell": true, "you": true, "cast": true,
	"cost": true, "to": true, "less": true, "card": true, "cards": true,
	"": true,
}

func costAlternativeMatches(card *Card, alt string) bool {
	words := strings.Fields(alt)
	matchedAnyConjunct := false
	for _, w := range words {
		if costFilterNoise[w] {
			continue
		}
		matchedAnyConjunct = true
		if w == "noncreature" {
			if cardHasType(card, "creature") {
				return false
			}
			continue
		}
		if col, ok := costFilterColors[w]; ok {
			if !CardHasColor(card, col) {
				return false
			}
			continue
		}
		if costFilterCardTypes[w] {
			if !cardHasType(card, w) {
				return false
			}
			continue
		}
		// Fallback: treat as a subtype (genuine membership check, fails
		// closed). Strip a trailing 's' for plurals like "elementals".
		sub := strings.TrimSuffix(w, "s")
		if !cardHasSubtype(card, w) && !cardHasSubtype(card, sub) {
			return false
		}
	}
	// An alternative made only of noise (no real conjunct) doesn't match.
	return matchedAnyConjunct
}
