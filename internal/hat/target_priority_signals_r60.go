package hat

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// target_priority_signals_r60.go — R60 round 13+ ChooseTarget audit fix.
//
// Survey: ChooseTarget's permanent-targeting branch (yggdrasil.go:6852)
// already runs a sophisticated tier-based scorer — power, oracle-text
// features (draw / whenever / each opponent), planeswalker loyalty
// proximity to ultimate, commander tag, combo pieces (+3.0), finishers
// (+2.0), value engines (+1.5), archetype-aware enchantment/artifact
// bias for combo opps, threat EvalScore fold, kingmaker + momentum,
// and politics leader bias. Far more than "biggest creature".
//
// Two real gaps the audit identified:
//
//   1. Indestructible targets. "Destroy target creature" silently
//      no-op's on indestructible permanents (CR §702.12b — damage
//      doesn't destroy them, and "destroy" instructions also fail).
//      Picking an indestructible target wastes the entire removal
//      spell. Nothing in the existing scoring caught this.
//
//   2. Death-payoff targets. hasDeathPayoffValue (line 3017) already
//      recognizes persist/undying/unearth, "when this dies", "if this
//      dies" — used by the attacker-scoring loop to identify "trick
//      attackers" that WANT to die. Removal pointed at those same
//      creatures GIVES the opponent value (re-ETB, token spawn,
//      recursion). The existing target scorer treated them like any
//      other creature.
//
// Both signals are additive penalties on the existing tier-based
// score. Indestructible is large (-5.0) because it's a binary "this
// spell does literally nothing"; death-payoff is moderate (-1.5)
// because the value is real but the body still goes away.

// removalTargetIndestructiblePenalty returns -5.0 when the target
// permanent has indestructible. The penalty is sized so it dominates
// even high-tier targeting bonuses (combo +3.0, finisher +2.0,
// kingmaker +2.0) — an indestructible combo piece is still pointless
// to "destroy" because the spell does nothing; a bounce / counter /
// exile spell would be the right call.
//
// Conservative on detection: only the evergreen indestructible
// keyword. Doesn't try to handle "gains indestructible until end of
// turn" (Heroic Intervention timing) since that's a stack-resolution
// race the heuristic can't model accurately.
//
// Returns 0 when the permanent is nil or non-indestructible.
func removalTargetIndestructiblePenalty(p *gameengine.Permanent) float64 {
	if p == nil {
		return 0
	}
	if p.HasKeyword("indestructible") {
		return -5.0
	}
	return 0
}

// removalTargetDeathPayoffPenalty returns -1.5 when the target's
// oracle text marks it as a death-payoff creature per the existing
// hasDeathPayoffValue helper (persist, undying, unearth, "when this
// dies" / "when this creature dies" / "if this dies"). Killing such
// a creature hands the opp value (token spawn, re-ETB pings, recursion
// loop). Sized to demote them below comparable threats that just die,
// without dropping them below outright non-threats — sometimes the
// Hangarback Walker IS the highest-impact target on the board even
// counting its dies-trigger value.
//
// Cards that don't trigger the helper (vanilla creatures, ETB-only
// value, attack-trigger value) get 0 — only the "dying generates
// permanent value to opp" case is penalized.
func removalTargetDeathPayoffPenalty(p *gameengine.Permanent) float64 {
	if p == nil || p.Card == nil {
		return 0
	}
	if hasDeathPayoffValue(p.Card) {
		return -1.5
	}
	return 0
}
