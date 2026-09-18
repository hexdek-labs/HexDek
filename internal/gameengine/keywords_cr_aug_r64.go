package gameengine

// keywords_cr_aug_r64.go — primitives for keyword actions / abilities and
// defined terms added in the Aug 7 2026 Comprehensive Rules edition. Each is a
// small, reusable rule primitive (the modular rule-focused approach); wiring
// the AST nodes that invoke them is tracked in docs/cr-aug-impact-scope.md.

// HealDamage removes marked damage from a permanent (CR §701.69a). "Heal N
// damage" removes up to n; n <= 0 removes ALL marked damage (the "damage ...
// is healed" form). Marked damage is wiped at end of turn regardless; this is
// the mid-turn heal action.
func HealDamage(perm *Permanent, n int) {
	if perm == nil {
		return
	}
	if n <= 0 || n >= perm.MarkedDamage {
		perm.MarkedDamage = 0
		return
	}
	perm.MarkedDamage -= n
}

// IsWorthy reports whether a card is a "worthy" creature (CR §700.16, Aug 7
// 2026 edition): a creature that is legendary, isn't a Villain, and is red
// and/or white.
//
// NOTE: the "isn't a Villain" clause is enforced but currently inert — the
// engine does not yet tag the Villain card type, so no card matches it. Once
// Villain typing exists this predicate excludes them with no further change.
func IsWorthy(c *Card) bool {
	if c == nil {
		return false
	}
	if !cardHasType(c, "creature") || !cardHasType(c, "legendary") {
		return false
	}
	if cardHasType(c, "villain") {
		return false
	}
	return cardHasColor(c, "R") || cardHasColor(c, "W")
}
