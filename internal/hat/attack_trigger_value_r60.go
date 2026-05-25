package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// attack_trigger_value_r60.go — R60 round 9+ ChooseAttackers audit fix.
// The pre-fix per-attacker value loop scored evasion, keywords, value-
// engine status, death-payoff triggers, and sac-fuel — and a SPECIFIC
// "commander attack-trigger tutor" bonus (+0.60 for Zur / Narset / Etali
// shape). But it had no general handling for the "when this attacks" /
// "whenever this attacks" payoff family that defines whole archetypes:
//
//   Hellrider     — "Whenever Hellrider attacks, deals 1 damage to
//                    defending player for each attacking creature"
//   Goldspan      — "Whenever Goldspan attacks ... create a Treasure"
//   Lord of the
//     Forsaken    — "Whenever a creature you control attacks ... opponent
//                    loses 1 life"
//   Maraxus       — "Whenever Maraxus attacks, gets +X/+0 ..."
//   Cosima        — "Whenever you attack ... draw a card"
//
// All of these gave a 0 attack-bonus pre-fix, so a 2/2 Maraxus with 4
// untapped creatures attacking (real swing power = 6) would HOLD on a
// turn where it should have swung; a 3/3 Hellrider with two other
// attackers (real swing damage = 3 + 3 chip damage = 6) ranked
// identically to a vanilla 3/3.
//
// Signal — attackTriggerBonus: scan the attacker's own oracle text for a
// "when[ever] {self} attacks" trigger, then categorize the payoff and
// return a graduated bonus. Conservative on false positives: only fires
// when the attack-trigger and a recognized payoff phrase both appear in
// the same card's text. Static effects ("attacking creatures get +1/+0")
// from another permanent are NOT in scope here — they're an anthem
// signal, not a per-attacker trigger, and require a different scan.

// attackTriggerBonus returns the per-attacker value bonus for cards with
// a self-attack trigger. Categories (priority order — first match wins
// so a damage trigger isn't double-counted with a pump trigger that
// happens to mention both):
//
//   damage trigger (+0.20): "deals N damage" payoffs (Hellrider,
//     Master of Cruelties). Direct life-loss for opponents, scaling
//     with attacker count for board-wide effects.
//
//   tutor / cast-from-top (+0.25): "search your library", "exile the
//     top", "look at the top" — Etali, Maelstrom Wanderer-shape. For
//     COMMANDER attackers, the caller already adds +0.60 separately,
//     so this only fires for non-commander cards in that family
//     (e.g. an Etali copy via Mirror Gallery, Wanderer in 99).
//
//   pump trigger (+0.15): "gets +N/+M" — Maraxus, Bloodthirster shape.
//     Self-pump scales with state; +0.15 is a base bump.
//
//   draw trigger (+0.15): "draw a card" / "draw N cards" — Cosima,
//     Edric, Akiri Line-Slinger.
//
//   token / treasure trigger (+0.10): "create a Treasure", "create N
//     ... tokens" — Goldspan Dragon, Adeline. Smaller than damage
//     because token value materializes next turn, not on the swing.
//
//   life-loss trigger (+0.15): "opponent[s] loses N life" / "lose N
//     life" — Lord of the Forsaken-shape drain payoffs.
//
// Returns 0 when no attack trigger is detected or the payoff doesn't
// match a recognized category.
func attackTriggerBonus(card *gameengine.Card) float64 {
	if card == nil {
		return 0
	}
	ot := gameengine.OracleTextLower(card)
	if !hasSelfAttackTrigger(ot) {
		return 0
	}
	// Categorize by payoff. Priority order matters — a single card with
	// "deals 1 damage AND creates a treasure" should score under
	// damage, the more impactful clause.
	switch {
	case strings.Contains(ot, "deals") && strings.Contains(ot, "damage"):
		return 0.20
	case strings.Contains(ot, "search your library"),
		strings.Contains(ot, "exile the top"),
		strings.Contains(ot, "look at the top"):
		return 0.25
	case strings.Contains(ot, "gets +"):
		return 0.15
	case strings.Contains(ot, "draw a card"),
		strings.Contains(ot, "draw two"),
		strings.Contains(ot, "draw cards"),
		strings.Contains(ot, "draw three"):
		return 0.15
	case strings.Contains(ot, "loses") && strings.Contains(ot, "life"),
		strings.Contains(ot, "lose") && strings.Contains(ot, "life"):
		return 0.15
	case strings.Contains(ot, "create a treasure"),
		strings.Contains(ot, "create") && strings.Contains(ot, "treasure"),
		strings.Contains(ot, "create") && strings.Contains(ot, "token"):
		return 0.10
	}
	return 0
}

// hasSelfAttackTrigger returns true when the oracle text contains a
// trigger clause that fires on THIS creature attacking. Matches:
//   - "when this creature attacks"     (post-One-Ring rules text)
//   - "whenever this creature attacks"
//   - "when ~ attacks" (engine renders self-reference as ~ in some loaders)
//   - "whenever ~ attacks"
//   - "when you attack" / "whenever you attack" (Cosima, Edric — fires
//     when ANY attacker is declared by us, so the bonus is valid on
//     each attacker that would otherwise tip the swing)
//
// Card-name-specific triggers ("whenever Hellrider attacks") aren't
// matched here — OracleTextLower from AST.Raw uses the card name as the
// trigger source verbatim, so the name appears in the trigger phrase.
// We sidestep that with a name-check: if the card's lowercased name
// appears followed by "attacks", treat as self-trigger.
func hasSelfAttackTrigger(ot string) bool {
	if strings.Contains(ot, "when this creature attacks") ||
		strings.Contains(ot, "whenever this creature attacks") ||
		strings.Contains(ot, "when ~ attacks") ||
		strings.Contains(ot, "whenever ~ attacks") ||
		strings.Contains(ot, "when you attack") ||
		strings.Contains(ot, "whenever you attack") {
		return true
	}
	// Fallback: match "whenever [name-token] attacks" where name-token
	// is any sequence of words that isn't "a", "an", "the", "another"
	// (which would signal a board-wide trigger, not a self trigger).
	// Cheap approximation: search for " attacks" preceded by a non-
	// generic-article word.
	idx := strings.Index(ot, " attacks")
	for idx > 0 {
		// Walk back to the previous space to find the word right before "attacks".
		before := ot[:idx]
		spaceIdx := strings.LastIndex(before, " ")
		word := before
		if spaceIdx >= 0 {
			word = before[spaceIdx+1:]
		}
		// Generic article = "another", "a", "an", "the", or a creature-type
		// word ("creature") — these aren't self triggers.
		if word != "another" && word != "a" && word != "an" && word != "the" &&
			word != "creature" && word != "creatures" && word != "" {
			// Look for the trigger keyword preceding this phrase.
			triggerWindow := before
			if start := len(triggerWindow) - 40; start > 0 {
				triggerWindow = triggerWindow[start:]
			}
			if strings.Contains(triggerWindow, "when") ||
				strings.Contains(triggerWindow, "whenever") {
				return true
			}
		}
		// Move to next " attacks" occurrence.
		next := strings.Index(ot[idx+1:], " attacks")
		if next < 0 {
			break
		}
		idx = idx + 1 + next
	}
	return false
}
