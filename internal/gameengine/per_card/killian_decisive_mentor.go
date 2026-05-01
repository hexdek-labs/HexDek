package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKillianDecisiveMentor wires Killian, Decisive Mentor.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Whenever an enchantment you control enters, tap up to one target
//	creature and goad it. (Until your next turn, that creature attacks
//	each combat if able and attacks a player other than you if able.)
//	Whenever one or more creatures that are enchanted by an Aura you
//	control attack, draw a card.
//
// NOT to be confused with "Killian, Ink Duelist" — different commander.
//
// Implementation:
//   - OnTrigger("permanent_etb"): if the entering permanent is an
//     enchantment under Killian's controller (and not Killian himself),
//     pick the highest-power untapped opponent creature and tap+goad it.
//     "Up to one" — gracefully no-op when no opponent creature exists.
//   - OnTrigger("creature_attacks"): when at least one declared attacker
//     has an Aura attached that is controlled by Killian's controller,
//     Killian's controller draws one card. Deduped per-turn since the
//     oracle reads "one or more creatures... attack" (single trigger per
//     attack declaration).
func registerKillianDecisiveMentor(r *Registry) {
	r.OnTrigger("Killian, Decisive Mentor", "permanent_etb", killianDecisiveMentorEnchantmentETB)
	r.OnTrigger("Killian, Decisive Mentor", "creature_attacks", killianDecisiveMentorAttackDraw)
}

func killianDecisiveMentorEnchantmentETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "killian_decisive_mentor_tap_goad"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entry, _ := ctx["perm"].(*gameengine.Permanent)
	if entry == nil || entry == perm {
		return
	}
	if !permIsType(entry, "enchantment") {
		return
	}

	target := killianPickGoadTarget(gs, perm.Controller)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent_creature", map[string]interface{}{
			"seat":     perm.Controller,
			"trigger":  entry.Card.DisplayName(),
		})
		return
	}

	target.Tapped = true
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["goaded"] = 1

	gs.LogEvent(gameengine.Event{
		Kind:   "goad",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"target_card": target.Card.DisplayName(),
			"slug":        slug,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"enchantment":    entry.Card.DisplayName(),
		"target":         target.Card.DisplayName(),
		"target_seat":    target.Controller,
	})
}

// killianPickGoadTarget returns the best opponent creature to tap+goad.
// Heuristic: prefer untapped, highest-power creatures (a tapped attacker
// has already swung — tapping it does nothing this turn, so untapped
// targets matter more).
func killianPickGoadTarget(gs *gameengine.GameState, controller int) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestScore := -1 << 30
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == controller {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			score := p.Power() * 2
			if !p.Tapped {
				score += 100
			}
			if score > bestScore {
				bestScore = score
				best = p
			}
		}
	}
	return best
}

func killianDecisiveMentorAttackDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "killian_decisive_mentor_aura_attack_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil {
		return
	}

	// "Enchanted by an Aura you control" — scan the battlefield for any
	// Aura controlled by Killian's controller whose AttachedTo == atk.
	aura := killianFindAttachedAura(gs, atk, perm.Controller)
	if aura == nil {
		return
	}

	// Dedupe per turn: the oracle reads "one or more creatures... attack"
	// — single trigger per attack declaration regardless of how many of
	// the attackers are aura-enchanted.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupeKey := fmt.Sprintf("killian_dm_draw_t%d", gs.Turn+1)
	if perm.Flags[dedupeKey] == 1 {
		return
	}
	perm.Flags[dedupeKey] = 1

	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"attacker":     atk.Card.DisplayName(),
		"aura":         aura.Card.DisplayName(),
		"drawn_card":   drawnName,
	})
}

// killianFindAttachedAura returns the first battlefield Aura controlled
// by `controller` that is currently attached to `target`, or nil.
func killianFindAttachedAura(gs *gameengine.GameState, target *gameengine.Permanent, controller int) *gameengine.Permanent {
	if gs == nil || target == nil {
		return nil
	}
	if controller < 0 || controller >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[controller]
	if s == nil {
		return nil
	}
	for _, p := range s.Battlefield {
		if p == nil || !p.IsAura() {
			continue
		}
		if p.AttachedTo == target {
			return p
		}
	}
	return nil
}
