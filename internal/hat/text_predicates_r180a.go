package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r180a — two more hot predicate clusters onto the per-Card text-predicate
// memo introduced in r164 (see internal/gameengine/card_text_predicate_r164.go
// and internal/hat/text_predicates_r164.go).
//
// strings.Contains is 13.7-14.2% of all production CPU in both the clean
// and the degraded profiling windows. scoreStaxLock and
// bestOpponentRemovalScore together are ~19% of that: scoreStaxLock alone
// runs an ordered six-case switch of up to 16 strings.Contains calls over
// the oracle text of EVERY permanent on EVERY evaluation, and every case is
// a pure function of the lowered oracle text.
//
// BIT RESERVATIONS. Bits 0-5 belong to r164 (see text_predicates_r164.go).
// Bits 6-15 are reserved to THIS change (r180a). Bits 16-25 are reserved to
// the sibling change landing in parallel — do not take them here. Explicit
// numeric values rather than a continuing iota, deliberately: the r164 const
// block lives in another file that is being edited concurrently and an iota
// chain across files would silently renumber on merge.
//
// As in r164, a predicate routed through this memo MUST be a pure function
// of the lowered oracle text. Anything that also reads Flags, Counters,
// GrantedAbilities or game state has no epoch to invalidate on and must not
// come through here.
const (
	// scoreStaxLock's six switch cases, in switch order.
	tpStaxNonlandDontUntap  uint = 6
	tpStaxCantCast          uint = 7
	tpStaxAdditionalCostPay uint = 8
	tpStaxEntersTappedOpp   uint = 9
	tpStaxWheneverOppLoses  uint = 10
	tpStaxSacEachOpponent   uint = 11

	// bestOpponentRemovalScore's "has triggered abilities → engine piece"
	// disjunction, cached as one bit.
	tpHasTriggerWording uint = 12

	// bits 13-15 free, still reserved to r180a.
)

// -----------------------------------------------------------------------------
// scoreStaxLock case predicates
//
// Each function below holds the ORIGINAL switch-case expression verbatim,
// just moved into a named func, so the rewrite can be diffed by eye. The
// switch remains ordered: first match wins, and these are consulted in the
// same order they appeared.
// -----------------------------------------------------------------------------

func isStaxNonlandDontUntap(ot string) bool {
	return strings.Contains(ot, "nonland permanents") && strings.Contains(ot, "don't untap")
}

func isStaxCantCast(ot string) bool {
	return strings.Contains(ot, "can't cast") || (strings.Contains(ot, "each opponent") && strings.Contains(ot, "can't"))
}

func isStaxAdditionalCostPay(ot string) bool {
	return strings.Contains(ot, "additional") && strings.Contains(ot, "cost") && strings.Contains(ot, "pay")
}

func isStaxEntersTappedOpp(ot string) bool {
	return strings.Contains(ot, "enters the battlefield tapped") && strings.Contains(ot, "opponents")
}

func isStaxWheneverOppLoses(ot string) bool {
	return strings.Contains(ot, "whenever") && strings.Contains(ot, "opponent") && (strings.Contains(ot, "loses") || strings.Contains(ot, "damage"))
}

func isStaxSacEachOpponent(ot string) bool {
	return strings.Contains(ot, "sacrifice") && strings.Contains(ot, "each") && strings.Contains(ot, "opponent")
}

// hasTriggerWording is bestOpponentRemovalScore's inline disjunction.
func hasTriggerWording(ot string) bool {
	return strings.Contains(ot, "whenever") || strings.Contains(ot, "at the beginning")
}

// -----------------------------------------------------------------------------
// Cached wrappers — the only call path the scorers should use.
// -----------------------------------------------------------------------------

func cachedStaxNonlandDontUntap(c *gameengine.Card) bool {
	return c.TextPredicate(tpStaxNonlandDontUntap, isStaxNonlandDontUntap)
}

func cachedStaxCantCast(c *gameengine.Card) bool {
	return c.TextPredicate(tpStaxCantCast, isStaxCantCast)
}

func cachedStaxAdditionalCostPay(c *gameengine.Card) bool {
	return c.TextPredicate(tpStaxAdditionalCostPay, isStaxAdditionalCostPay)
}

func cachedStaxEntersTappedOpp(c *gameengine.Card) bool {
	return c.TextPredicate(tpStaxEntersTappedOpp, isStaxEntersTappedOpp)
}

func cachedStaxWheneverOppLoses(c *gameengine.Card) bool {
	return c.TextPredicate(tpStaxWheneverOppLoses, isStaxWheneverOppLoses)
}

func cachedStaxSacEachOpponent(c *gameengine.Card) bool {
	return c.TextPredicate(tpStaxSacEachOpponent, isStaxSacEachOpponent)
}

func cachedHasTriggerWording(c *gameengine.Card) bool {
	return c.TextPredicate(tpHasTriggerWording, hasTriggerWording)
}
