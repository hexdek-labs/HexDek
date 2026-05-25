package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// etb_trigger_value_r60.go — R60 round 14+ cardHeuristic audit fix.
// Companion to attack_trigger_value_r60.go (PR #339, attack triggers),
// postcombat_trigger_value_r60.go (PR #357, combat-damage triggers),
// and hasDeathPayoffValue (yggdrasil.go:3017, dies triggers). This
// round folds the ETB-trigger family into cast-order scoring.
//
// Audit: cardHeuristic already has Freya CatRamp / CatDraw / CatRemoval
// bonuses sized small (+0.10 at Develop phase), plus archetype-specific
// bumps + commander-theme matches + curse DNA. But cards whose value
// lives ENTIRELY in their ETB trigger were underweighted:
//
//   - Multi-mode ETBs (Acidic Slime: destroy artifact/enchantment/land,
//     Solemn Simulacrum: ramp + dies-draw) don't fit a single Freya
//     category cleanly.
//   - Search/tutor ETBs (Stoneforge Mystic, Recruiter of the Guard,
//     Imperial Recruiter) score as Threat/Utility by default.
//   - Recursion ETBs (Eternal Witness, Reveillark, Sun Titan) score
//     as Threat — but their ETB IS the value.
//
// Detected via hasSelfEtbTrigger and categorized in etbTriggerBonus.
// Disjoint from the attack / postcombat / dies families: detection
// keys on "when this enters" / "when ~ enters" / "enters the
// battlefield" rather than the other families' trigger phrases.

// etbTriggerBonus returns the per-card value bonus for cards with a
// self-referential ETB trigger. Categories (priority order — first
// match wins):
//
//   tutor (+0.30): "search your library" on ETB — Stoneforge Mystic,
//     Recruiter of the Guard, Imperial Recruiter, Recruiter of the Wits.
//     Tutoring on ETB is the strongest ETB family — it adds a card to
//     hand without spending a card.
//
//   draw (+0.20): "draw N cards" — Mulldrifter, Cloudblazer, Sea Gate
//     Oracle, Mentor of the Meek. Direct card advantage on ETB.
//
//   removal (+0.20): "destroy target" / "exile target" / "deals N
//     damage" — Reclamation Sage, Acidic Slime, Bonecrusher adventure
//     side, Flametongue Kavu. Spot removal stapled to a body.
//
//   ramp (+0.15): "search your library for a ... land" — Wood Elves,
//     Solemn Simulacrum, Farhaven Elf. Smaller than draw because the
//     mana materializes next turn, not on cast.
//
//   token / treasure (+0.10): "create" something on ETB — Trostani's
//     Summoner, Mentor of the Meek (token side), Sai Master Thopterist.
//
//   recursion (+0.10): "return target card from your graveyard" /
//     "return target ... from your graveyard" — Eternal Witness,
//     Reveillark, Sun Titan. Small bonus — value depends on
//     graveyard state which the helper can't model.
//
// Returns 0 when no self-ETB trigger is detected. Anthem-shape ETB
// triggers from OTHER permanents ("whenever a creature enters") are
// NOT matched — those affect ALL ETBs and are a separate signal that
// would need its own helper.
func etbTriggerBonus(card *gameengine.Card) float64 {
	if card == nil {
		return 0
	}
	ot := gameengine.OracleTextLower(card)
	if ot == "" {
		return 0
	}
	if !hasSelfEtbTrigger(ot) {
		return 0
	}
	// Categorize by payoff in priority order. A card with multiple
	// payoffs (e.g. Solemn Simulacrum has both ramp ETB and draw on
	// death) scores under the first matching ETB bucket since that's
	// the immediate-cast value; the dies-trigger value is captured
	// separately by hasDeathPayoffValue.
	switch {
	case strings.Contains(ot, "search your library"):
		return 0.30
	case strings.Contains(ot, "draw a card"),
		strings.Contains(ot, "draw two"),
		strings.Contains(ot, "draw cards"),
		strings.Contains(ot, "draw three"):
		return 0.20
	case strings.Contains(ot, "destroy target"),
		strings.Contains(ot, "exile target"),
		strings.Contains(ot, "deals") && strings.Contains(ot, "damage to"):
		return 0.20
	case strings.Contains(ot, "put a land") ||
		strings.Contains(ot, "put that land"):
		// Land-ramp ETB without an explicit "search your library"
		// phrase (some cards phrase it as "put a basic land onto
		// the battlefield").
		return 0.15
	case strings.Contains(ot, "create a treasure"),
		strings.Contains(ot, "create") && strings.Contains(ot, "treasure"),
		strings.Contains(ot, "create") && strings.Contains(ot, "token"):
		return 0.10
	case strings.Contains(ot, "return target") && strings.Contains(ot, "graveyard"):
		return 0.10
	}
	return 0
}

// hasSelfEtbTrigger returns true when the oracle text contains a trigger
// clause that fires when THIS permanent enters the battlefield. Matches:
//   - "when this creature enters"   (post-One-Ring rules text)
//   - "when this enters"
//   - "when ~ enters"               (AST renders self-reference as ~)
//   - "whenever this/~ enters"
//   - "[name] enters" (fallback name-search, mirrors hasSelfAttackTrigger)
//
// "Whenever a creature enters" / "whenever another creature enters"
// (Soul Warden, Impact Tremors, Vituperate) are rejected via the
// generic-article filter so anthem-shape ETB triggers don't false-
// positive as self-triggers.
func hasSelfEtbTrigger(ot string) bool {
	if strings.Contains(ot, "when this creature enters") ||
		strings.Contains(ot, "when this enters") ||
		strings.Contains(ot, "when ~ enters") ||
		strings.Contains(ot, "whenever this creature enters") ||
		strings.Contains(ot, "whenever this enters") ||
		strings.Contains(ot, "whenever ~ enters") {
		return true
	}
	// Fallback: match "[trigger-word] [non-generic-article] enters" so
	// "when [name] enters the battlefield" catches cards whose oracle
	// text uses the literal name. Generic articles
	// (a/an/the/another/creature/permanent) are rejected — those are
	// anthem-shape board triggers, not self-triggers.
	for _, phrase := range []string{" enters the battlefield", " enters,"} {
		idx := strings.Index(ot, phrase)
		for idx > 0 {
			before := ot[:idx]
			spaceIdx := strings.LastIndex(before, " ")
			word := before
			if spaceIdx >= 0 {
				word = before[spaceIdx+1:]
			}
			if word != "another" && word != "a" && word != "an" && word != "the" &&
				word != "creature" && word != "creatures" && word != "permanent" &&
				word != "permanents" && word != "" {
				// Confirm a trigger keyword precedes within ~40 chars
				// (same heuristic as hasSelfAttackTrigger).
				triggerWindow := before
				if start := len(triggerWindow) - 40; start > 0 {
					triggerWindow = triggerWindow[start:]
				}
				if strings.Contains(triggerWindow, "when") ||
					strings.Contains(triggerWindow, "whenever") {
					return true
				}
			}
			next := strings.Index(ot[idx+1:], phrase)
			if next < 0 {
				break
			}
			idx = idx + 1 + next
		}
	}
	return false
}
