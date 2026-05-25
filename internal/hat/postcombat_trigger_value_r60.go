package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// postcombat_trigger_value_r60.go — R60 round 10+ ChooseAttackers
// audit fix, companion to attack_trigger_value_r60.go (PR #339). The
// previous round folded ATTACK-DECLARATION triggers (Hellrider, Maraxus,
// Goldspan, Cosima) into per-attacker score. This round folds the
// POSTCOMBAT trigger family — triggers that fire AFTER combat damage or
// at end of combat:
//
//   "deals combat damage to a player" triggers (Toski, Cold-Eyed Selkie,
//     Bident of Thassa style) — payoff materializes if the attacker
//     connects. Draw-shaped payoffs are the dominant case in Commander.
//
//   "at the end of combat" triggers, especially extra-combat (Aurelia
//     the Warleader, Scourge of the Throne, Combat Celebrant). Extra
//     combat is explosive — doubles the turn's swing — and easily worth
//     more than any standard postcombat trigger.
//
//   "becomes blocked" triggers (rarer; Battle-Mad Ronin, Skyhunter Cub
//     aura, certain auras). Small bonus.
//
// Crucially: these triggers are DISJOINT from the attack-declaration
// family pinned by attackTriggerBonus — that helper matches
// "when[ever] X attacks", this one matches "deals combat damage" /
// "end of combat" / "becomes blocked". Cards that match BOTH (rare —
// e.g. Maelstrom Wanderer has both an attack trigger and an extra
// combat) get both bonuses, which is correct: the trigger surfaces
// twice in different contexts.
//
// Each bonus is sized parallel to attackTriggerBonus so the relative
// weights stay coherent — and the absolute size sits below the
// commander-tutor +0.60 so neither helper drowns the profitability
// prune downstream.

// postcombatTriggerBonus returns the per-attacker value bonus for cards
// with a postcombat trigger. Categories (priority order — first match
// wins):
//
//   extra-combat trigger (+0.30): "additional combat phase" appears
//     anywhere in the oracle text. Aurelia, Scourge of the Throne,
//     Combat Celebrant. Doubles the turn's swing potential — easily
//     the most valuable postcombat shape.
//
//   combat-damage → draw (+0.20): "deals combat damage to a player"
//     paired with "draw". Toski, Cold-Eyed Selkie, Bident of Thassa
//     bearers. The dominant value-attacker shape in Commander.
//
//   combat-damage → other (+0.15): "deals combat damage to a player"
//     without a recognized draw payoff. Catches generic damage-rider
//     triggers (Tetsuko-shape life loss, search-on-damage, etc.) so
//     we don't completely zero them out.
//
//   end-of-combat trigger (+0.10): "at the end of combat" without
//     extra-combat language. Small bonus — most EOC triggers are
//     bookkeeping (counter removal, exile-return).
//
//   becomes-blocked trigger (+0.10): "becomes blocked" — rare; often
//     pumps the creature so the trade goes the other way.
func postcombatTriggerBonus(card *gameengine.Card) float64 {
	if card == nil {
		return 0
	}
	ot := gameengine.OracleTextLower(card)
	if ot == "" {
		return 0
	}
	// Extra combat — priority bucket. The phrase is unambiguous (no
	// passive-text false positives) and the payoff is the largest.
	if strings.Contains(ot, "additional combat phase") ||
		strings.Contains(ot, "additional combat") {
		return 0.30
	}
	// Combat damage to a player — the core postcombat trigger family.
	// We accept "to a player" / "to an opponent" / "to one of your
	// opponents" / "to each opponent" — all forms appear in the corpus.
	hasCombatDamageTrigger := false
	for _, needle := range []string{
		"deals combat damage to a player",
		"deals combat damage to an opponent",
		"deals combat damage to one of your opponents",
		"deals combat damage to each opponent",
		"deals combat damage to that player",
	} {
		if strings.Contains(ot, needle) {
			hasCombatDamageTrigger = true
			break
		}
	}
	if hasCombatDamageTrigger {
		// Categorize by payoff in priority order.
		if strings.Contains(ot, "draw a card") ||
			strings.Contains(ot, "draw cards") ||
			strings.Contains(ot, "draw two") ||
			strings.Contains(ot, "draw three") ||
			strings.Contains(ot, "draw that many") {
			return 0.20
		}
		return 0.15
	}
	// End-of-combat triggers without extra-combat language. Most are
	// bookkeeping but a few are genuine value (Goblin Goon repeat
	// attack, Battle Plan-style mass pump). Small bonus.
	if strings.Contains(ot, "at the end of combat") ||
		strings.Contains(ot, "at end of combat") {
		return 0.10
	}
	// Becomes-blocked triggers — Bushido-style pumps and rare riders.
	if strings.Contains(ot, "becomes blocked") {
		return 0.10
	}
	return 0
}
