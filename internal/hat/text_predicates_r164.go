package hat

import "github.com/hexdek/hexdek/internal/gameengine"

// r164 — bit assignments for gameengine's per-Card text-predicate memo.
//
// strings.Contains is 21.13% of all CPU and a 30s production profile
// puts most of it in the predicates below, each re-scanning the same
// memoized oracle text for the same card on every scoring pass.
//
// Each bit caches the ANSWER, keyed on card.AST. Adding a predicate
// means taking the next bit and NOTHING ELSE — but it must be a PURE
// function of the lowered oracle text. A predicate that also reads
// Flags, Counters or GrantedAbilities has no epoch to invalidate on
// and must not come through here.
//
// Bits are append-only. Renumbering them is harmless at runtime (the
// mask is rebuilt per AST) but makes profiles and diffs lie, so don't.
const (
	tpImpulseDrawEngine uint = iota
	tpPersistentScryEngine
	tpPersistentDrawEngine
	tpGraveyardCastKeyword
	tpManaOnlyAbility
)

// The wrappers below are the only call path the evaluator should use.
// Each takes the Card the predicate is about, because the memo lives
// on the Card; the underlying string functions stay exactly as they
// were so their logic remains readable and testable in isolation.

func cachedImpulseDrawEngine(c *gameengine.Card) bool {
	return c.TextPredicate(tpImpulseDrawEngine, isImpulseDrawEngine)
}

func cachedPersistentScryEngine(c *gameengine.Card) bool {
	return c.TextPredicate(tpPersistentScryEngine, isPersistentScryEngine)
}

// cachedPersistentDrawEngine takes the Card rather than the Permanent
// the original did: isPersistentDrawEngine reads nothing but
// p.Card's oracle text, which is what makes it cacheable at all.
func cachedPersistentDrawEngine(c *gameengine.Card) bool {
	return c.TextPredicate(tpPersistentDrawEngine, isPersistentDrawEngineText)
}

func cachedGraveyardCastKeyword(c *gameengine.Card) bool {
	return c.TextPredicate(tpGraveyardCastKeyword, hasGraveyardCastKeyword)
}

func cachedManaOnlyAbility(c *gameengine.Card) bool {
	return c.TextPredicate(tpManaOnlyAbility, isManaOnlyAbility)
}
