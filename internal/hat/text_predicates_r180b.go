package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r180b — the drain-engine and opponent-graveyard-enabler clusters move
// onto the per-Card text-predicate memo introduced in r164.
//
// strings.Contains is 13.7-14.2% of all production CPU in both clean
// and degraded windows, and scoreDrainEngine + scoreOpponentGraveyard
// are ~16% of that. Both re-scan the same memoized oracle text for the
// same card on every scoring pass; only the ANSWER is cached here, one
// bit per named predicate, keyed on card.AST by TextPredicate.
//
// BIT RESERVATIONS. The r164 const block owns bits 0-5 and is being
// extended concurrently, so this round does NOT touch it:
//
//	bits 0-5    r164/r172, in internal/hat/text_predicates_r164.go
//	bits 6-15   RESERVED to the sibling change editing that file
//	bits 16-25  RESERVED to this change (r180b); 16-21 used below
//
// Values are written out explicitly rather than with iota so that the
// two files cannot drift into overlapping each other.
//
// Every predicate below is a PURE function of the lowered oracle text,
// which is the entry condition for this memo. In particular the drain
// fodder branch's p.IsToken() / p.IsCreature() reads are NOT pure
// oracle text, have no AST epoch to invalidate on, and deliberately
// stay inline in scoreDrainEngine — only the strings.Contains half of
// that branch is cached here (bit 20).
const (
	tpDrainDeathTriggerPayoff    uint = 16
	tpDrainLifegainPayoff        uint = 17
	tpDrainETBPayoff             uint = 18
	tpDrainSacOutlet             uint = 19
	tpDrainFodderDeathTrigger    uint = 20
	tpOpponentReanimationEnabler uint = 21
)

// --- Underlying string predicates -------------------------------------
//
// Each body below is the ORIGINAL expression moved verbatim out of the
// evaluator and given a name, so a reviewer can diff them by eye
// against the pre-r180b scoreDrainEngine / scoreOpponentGraveyard.

// isDrainDeathTriggerPayoff: Blood Artist, Zulaport Cutthroat,
// Bastion of Remembrance, Vindictive Vampire, Syr Konrad, etc.
func isDrainDeathTriggerPayoff(ot string) bool {
	return strings.Contains(ot, "whenever") &&
		(strings.Contains(ot, "creature dies") || strings.Contains(ot, "a creature dies")) &&
		(strings.Contains(ot, "loses") || strings.Contains(ot, "drain") ||
			strings.Contains(ot, "each opponent") || strings.Contains(ot, "life"))
}

// isDrainLifegainPayoff: Dina, Vito, Marauding Blight-Priest.
func isDrainLifegainPayoff(ot string) bool {
	return strings.Contains(ot, "whenever you gain life") &&
		(strings.Contains(ot, "loses") || strings.Contains(ot, "each opponent"))
}

// isDrainETBPayoff: Wayward Servant, Corpse Knight.
func isDrainETBPayoff(ot string) bool {
	return strings.Contains(ot, "whenever") &&
		strings.Contains(ot, "enters") &&
		(strings.Contains(ot, "loses") || strings.Contains(ot, "each opponent"))
}

// isDrainSacOutlet: sacrifice outlets (free or cheap). The trailing
// negation is the whole reason this round carries a differential test.
func isDrainSacOutlet(ot string) bool {
	return (strings.Contains(ot, "sacrifice a creature") || strings.Contains(ot, "sacrifice another") ||
		strings.Contains(ot, "sacrifice an artifact") || strings.Contains(ot, "sacrifice a permanent")) &&
		!strings.Contains(ot, "when")
}

// isDrainFodderDeathTrigger is ONLY the text half of the fodder branch.
// The p.IsToken() / p.IsCreature() guards around it are permanent
// state, not oracle text, and stay in scoreDrainEngine.
func isDrainFodderDeathTrigger(ot string) bool {
	return strings.Contains(ot, "when") && strings.Contains(ot, "dies")
}

// isOpponentReanimationEnabler: an opponent battlefield permanent that
// can put something from a graveyard onto the battlefield.
func isOpponentReanimationEnabler(ot string) bool {
	return (strings.Contains(ot, "return") || strings.Contains(ot, "put")) &&
		strings.Contains(ot, "graveyard") &&
		strings.Contains(ot, "battlefield")
}

// --- Cached wrappers ---------------------------------------------------
//
// These are the only call path the evaluator should use.

func cachedDrainDeathTriggerPayoff(c *gameengine.Card) bool {
	return c.TextPredicate(tpDrainDeathTriggerPayoff, isDrainDeathTriggerPayoff)
}

func cachedDrainLifegainPayoff(c *gameengine.Card) bool {
	return c.TextPredicate(tpDrainLifegainPayoff, isDrainLifegainPayoff)
}

func cachedDrainETBPayoff(c *gameengine.Card) bool {
	return c.TextPredicate(tpDrainETBPayoff, isDrainETBPayoff)
}

func cachedDrainSacOutlet(c *gameengine.Card) bool {
	return c.TextPredicate(tpDrainSacOutlet, isDrainSacOutlet)
}

func cachedDrainFodderDeathTrigger(c *gameengine.Card) bool {
	return c.TextPredicate(tpDrainFodderDeathTrigger, isDrainFodderDeathTrigger)
}

func cachedOpponentReanimationEnabler(c *gameengine.Card) bool {
	return c.TextPredicate(tpOpponentReanimationEnabler, isOpponentReanimationEnabler)
}
