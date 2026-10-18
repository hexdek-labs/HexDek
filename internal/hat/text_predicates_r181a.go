package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r181a — scoreThreat's two oracle-text clusters move onto the per-Card
// text-predicate memo introduced in r164 (see
// internal/gameengine/card_text_predicate_r164.go and
// internal/hat/text_predicates_r164.go).
//
// strings.Contains is 13.71% of all production CPU in a clean profiling
// window and 14.18% in a degraded one. scoreThreat is the single largest
// consumer at 11.37% of that: it walks EVERY permanent on EVERY opponent's
// battlefield on EVERY evaluation and runs eight strings.Contains calls
// over oracle text that never changes, plus a further six per permanent in
// the low-library mill scan. Every one of those decisions is a pure
// function of the lowered oracle text, so only the ANSWER is cached here —
// one bit per named predicate, keyed on card.AST by TextPredicate.
//
// BIT RESERVATIONS across the whole memo (64 bits total):
//
//	bits 0-5    r164/r172  internal/hat/text_predicates_r164.go
//	bits 6-12   r180a      internal/hat/text_predicates_r180a.go
//	bits 13-15  free, reserved to r180a
//	bits 16-21  r180b      internal/hat/text_predicates_r180b.go
//	bits 22-26  r181a      THIS FILE
//	bit  27     free, reserved to r181a
//	bits 28-63  unallocated
//
// Values are written out explicitly rather than with a continuing iota,
// deliberately: the const blocks live in separate files and an iota chain
// across them would silently renumber on merge.
//
// As in every prior round, a predicate routed through this memo MUST be a
// pure function of the lowered oracle text. Anything that also reads Flags,
// Counters, GrantedAbilities or game state has no epoch to invalidate on
// and must not come through here. scoreThreat's other per-permanent reads
// (p.HasKeyword("infect"/"toxic") in the poison block, DisplayName matching
// in the hoser block, Power/Toughness/Tapped/PhasedOut in the R60 blocks)
// are therefore deliberately left inline and uncached.
const (
	// scoreThreat's four independent dangerousPermanents tests. These are
	// NOT a switch — a single card can satisfy several of them and each
	// one that fires adds its own weight.
	tpThreatCreatureEntersDamage uint = 22
	tpThreatCreatureDiesLoses    uint = 23
	tpThreatEachOpponentLoses    uint = 24
	tpThreatDoubleDamage         uint = 25

	// scoreThreat's mill-threat disjunction, inside the myLibSize < 15
	// block. Cached as one bit: the library-size tiers that select the
	// penalty are game state, but the "is this permanent a mill threat
	// at all" question is pure text.
	tpThreatMillSource uint = 26

	// bit 27 free, still reserved to r181a.
)

// --- Underlying string predicates -------------------------------------
//
// Each body below is the ORIGINAL expression moved verbatim out of
// scoreThreat and given a name, so a reviewer can diff them by eye against
// the pre-r181a evaluator.

// isThreatCreatureEntersDamage: Purphoros, Warstorm Surge, Terror of the
// Peaks. Weight 0.3.
func isThreatCreatureEntersDamage(ot string) bool {
	return strings.Contains(ot, "whenever a creature enters") && strings.Contains(ot, "damage")
}

// isThreatCreatureDiesLoses: Blood Artist, Zulaport Cutthroat. Weight 0.25.
func isThreatCreatureDiesLoses(ot string) bool {
	return strings.Contains(ot, "whenever a creature dies") && strings.Contains(ot, "loses")
}

// isThreatEachOpponentLoses: symmetrical drain effects. Weight 0.2.
func isThreatEachOpponentLoses(ot string) bool {
	return strings.Contains(ot, "each opponent") && strings.Contains(ot, "loses")
}

// isThreatDoubleDamage: Furnace of Rath, Gratuitous Violence, Torbran-style
// damage amplifiers. Weight 0.3.
func isThreatDoubleDamage(ot string) bool {
	return strings.Contains(ot, "double") && strings.Contains(ot, "damage")
}

// isThreatMillSource is the whole mill disjunction from the myLibSize < 15
// block, verbatim. The myLibSize tiers (0.8 / 0.4 / 0.2) stay in
// scoreThreat because library size is game state, not oracle text.
func isThreatMillSource(ot string) bool {
	return (strings.Contains(ot, "mill") || strings.Contains(ot, "cards from the top of") ||
		(strings.Contains(ot, "library") && strings.Contains(ot, "graveyard"))) &&
		(strings.Contains(ot, "opponent") || strings.Contains(ot, "target player") || strings.Contains(ot, "each player"))
}

// --- Cached wrappers ---------------------------------------------------
//
// These are the only call path scoreThreat should use.

func cachedThreatCreatureEntersDamage(c *gameengine.Card) bool {
	return c.TextPredicate(tpThreatCreatureEntersDamage, isThreatCreatureEntersDamage)
}

func cachedThreatCreatureDiesLoses(c *gameengine.Card) bool {
	return c.TextPredicate(tpThreatCreatureDiesLoses, isThreatCreatureDiesLoses)
}

func cachedThreatEachOpponentLoses(c *gameengine.Card) bool {
	return c.TextPredicate(tpThreatEachOpponentLoses, isThreatEachOpponentLoses)
}

func cachedThreatDoubleDamage(c *gameengine.Card) bool {
	return c.TextPredicate(tpThreatDoubleDamage, isThreatDoubleDamage)
}

func cachedThreatMillSource(c *gameengine.Card) bool {
	return c.TextPredicate(tpThreatMillSource, isThreatMillSource)
}
