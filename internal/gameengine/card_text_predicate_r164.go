package gameengine

import (
	"sync/atomic"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r164 — memoize the RESULT of a text predicate per Card, keyed on the
// AST it was derived from.
//
// strings.Contains is 21.13% of all CPU on r163 — the single largest
// item in the binary and untouched for fourteen rounds. A 30s profile
// attributes it almost entirely to internal/hat predicates reading the
// memoized lowered oracle text:
//
//	scoreOpponentGraveyard 14.15%   isPersistentDrawEngine  8.71%
//	isManaOnlyAbility       8.23%   isImpulseDrawEngine     7.59%
//	hasGraveyardCastKeyword 6.24%   scoreStaxLock           6.08%
//	scoreThreat             5.92%   isPersistentScryEngine  5.68%
//
// The obvious shape was a per-Card bitmask of NEEDLES, one bit per
// literal. There are 221 distinct needles across 285 call sites, which
// is a large, meaningless table — and the needle is not what anyone
// asks for twice.
//
// What is asked for repeatedly is the PREDICATE. Each of these is a
// pure function of the lowered oracle text, that text is already
// memoized per Card on the AST pointer (r157), and the same card is
// re-evaluated on every scoring pass of every turn. So cache the
// ANSWER: one bit per named predicate instead of one per needle, ~20
// bits instead of 221, each one semantically meaningful.
//
// Keyed on card.AST, never on a "have we computed this" bool — r157
// spent a whole round on exactly that mistake, where a DFC transform
// reassigned Card.AST and three caches kept serving the front face for
// the rest of the game. A transform invalidates every predicate here
// for free.
//
// Concurrency: Card objects are per-game (decks are cloned per game),
// which is the same assumption r157 and r158 ship on, and the hat and
// gameengine race gates cover it.

// predicateBits is the width of the mask. Exceeding it is a programming
// error, not a runtime condition: TextPredicate falls back to computing
// uncached so a new bit can never return a wrong answer, only a slow
// one.
const predicateBits = 64

// TextPredicate returns compute(OracleTextLower(c)), memoized per Card
// under `bit`. Callers own the bit numbering; see internal/hat.
//
// compute must be a PURE function of the lowered oracle text. A
// predicate that also reads mutable permanent state (Flags, Counters,
// GrantedAbilities) must NOT come through here — those have no epoch
// and the cache would go stale invisibly.
func (c *Card) TextPredicate(bit uint, compute func(string) bool) bool {
	if c == nil || compute == nil {
		return false
	}
	if bit >= predicateBits {
		textPredUncached.Add(1)
		return compute(OracleTextLower(c))
	}
	if c.textPredAST != c.AST {
		c.textPredVal = 0
		c.textPredSet = 0
		c.textPredAST = c.AST
	}
	m := uint64(1) << bit
	if c.textPredSet&m != 0 {
		textPredHits.Add(1)
		return c.textPredVal&m != 0
	}
	textPredMisses.Add(1)
	v := compute(OracleTextLower(c))
	c.textPredSet |= m
	if v {
		c.textPredVal |= m
	}
	return v
}

// InvalidateTextPredicates drops the memo for this card. Nothing in
// production needs it — the AST key handles transforms — but a test
// that rewrites oracle text in place without touching the AST does.
func (c *Card) InvalidateTextPredicates() {
	if c == nil {
		return
	}
	c.textPredVal = 0
	c.textPredSet = 0
	c.textPredAST = nil
}

// textPredASTOf exposes the key for tests that must assert the cache
// follows the AST rather than a readiness flag.
func (c *Card) textPredASTOf() *gameast.CardAST { return c.textPredAST }

var (
	textPredHits     atomic.Uint64
	textPredMisses   atomic.Uint64
	textPredUncached atomic.Uint64
)

// TextPredicateStats returns hits, misses and out-of-range fallbacks.
// r155's lesson: publish the hit rate rather than assuming it. My
// charCache prediction was wrong by 4x when it was finally measured.
func TextPredicateStats() (hits, misses, uncached uint64) {
	return textPredHits.Load(), textPredMisses.Load(), textPredUncached.Load()
}

// ResetTextPredicateStatsForTest zeroes the counters.
func ResetTextPredicateStatsForTest() {
	textPredHits.Store(0)
	textPredMisses.Store(0)
	textPredUncached.Store(0)
}
