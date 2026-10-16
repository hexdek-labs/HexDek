package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// alt_cost_colour_r128.go — CR §202.1a coloured-pip enforcement for
// ALTERNATIVE costs.
//
//	§601.2f "the total cost is the mana cost or alternative cost (as
//	         determined in rule 601.2b), plus all additional costs and
//	         cost increases, and minus all cost reductions."
//	§107.4a "Colored mana in costs can be paid only with the appropriate
//	         color of mana."
//	§202.1a  paying a cost "requires matching the type of any colored or
//	         colorless mana symbols".
//
// r126 put the colour question on the cast path, but only against the
// card's PRINTED cost: `buildCastableList` asks CanSupplyColoredPips
// before offering a card from hand. An alternative cost is a DIFFERENT
// cost string, and nothing checked it — the alt-cost branches in
// CastSpell compare `EnsureTypedPool(seat).Total()` against an integer
// and nothing else.
//
// That gap is not theoretical, and it is not symmetrical with the
// printed cost. 36 corpus cards demand MORE coloured mana of some colour
// in their alternative cost than their printed cost demands, so passing
// the r126 check proves nothing about the alt cost:
//
//	overload   12 cards   Mizzium Mortars  {1}{R}  -> {3}{R}{R}{R}
//	prototype  21 cards   Goring Warplow   {6}     -> {1}{B}
//	blitz       2 cards   Tenacious Underdog {1}{B} -> {2}{B}{B}
//	spectacle   1 card    Rix Maadi Reveler {1}{R} -> {2}{B}{R}
//
// Prototype is the extreme: every one of the 21 has a COLOURLESS printed
// cost, so the r126 check has no pips to object to and a mono-white board
// could pay {1}{B}. 274 of 1,758 live decks hold at least one of the 36.
//
// DIRECTION OF SAFETY. These gates sit on an OPTIONAL cost. Failing one
// declines the alternative cost and the spell is cast normally; it never
// turns into a cast failure. So the cost of a false negative is "the AI
// did not overload", not "a legal play was blocked" — but the carve-outs
// are kept anyway, because declining is still a behaviour change.

// altCostColourOK reports whether `seat` can pay the coloured pips of the
// alternative cost string `costString`, per §202.1a.
//
// Permissive at every point of doubt, matching CanSupplyColoredPips:
// an empty/unparsed cost string, a pool holding restricted mana
// (§106.4a), and Phyrexian / twobrid pips all read as payable.
func altCostColourOK(seat *Seat, costString string) bool {
	if seat == nil || costString == "" {
		return true
	}
	return CanSupplyColoredPipsForCost(EnsureTypedPool(seat), costString)
}

// keywordArgCostString returns the RAW cost string of a keyword's cost
// argument — "{3}{R}{R}{R}" rather than the 5 that keywordArgCostStrict
// derives from it. The pip detail is thrown away by the int form, and
// the pip detail is the whole question here.
//
// ok=false when the keyword is absent, carries no args, or carries a cost
// the parser stored as a bare number: in that last case the colour of the
// alternative cost is genuinely unknown and the caller must not guess.
func keywordArgCostString(card *Card, keywordName string) (string, bool) {
	if card == nil || card.AST == nil {
		return "", false
	}
	want := strings.ToLower(strings.TrimSpace(keywordName))
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) != want {
			continue
		}
		for _, arg := range kw.Args {
			s, ok := arg.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if strings.HasPrefix(s, "{") {
				return s, true
			}
		}
		return "", false
	}
	return "", false
}

// altCostKeywordColourOK is the one-call form used by the cast-time
// alternative-cost gates in CastSpell: read the keyword's printed cost
// string, and answer the colour question about it. A keyword whose cost
// string the parser never captured reads as payable (see above).
func altCostKeywordColourOK(seat *Seat, card *Card, keywordName string) bool {
	cs, ok := keywordArgCostString(card, keywordName)
	if !ok {
		return true
	}
	return altCostColourOK(seat, cs)
}

// KeywordCostString is the exported form of keywordArgCostString, for
// drivers outside this package that need to colour-check an alternative
// cost before offering it (r132: the escape enumerator in
// internal/tournament). ok=false means the parser captured no cost
// string and the colour of that cost is unknown — callers must treat
// that as "do not judge", never as "unpayable".
func KeywordCostString(card *Card, keywordName string) (string, bool) {
	return keywordArgCostString(card, keywordName)
}
