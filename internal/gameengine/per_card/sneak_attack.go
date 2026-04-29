package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSneakAttack wires up Sneak Attack.
//
// Oracle text:
//
//	{R}: You may put a creature card from your hand onto the battlefield.
//	That creature gains haste. Sacrifice it at the beginning of the next
//	end step.
//
// Implementation:
//   - OnActivated (ability index 0): put a creature from hand onto
//     battlefield, grant haste, register a delayed trigger to sacrifice
//     at next end step.
func registerSneakAttack(r *Registry) {
	r.OnActivated("Sneak Attack", sneakAttackActivated)
}

func sneakAttackActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sneak_attack_cheat_in"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if len(s.Hand) == 0 {
		emitFail(gs, slug, "Sneak Attack", "no_cards_in_hand", nil)
		return
	}

	// Find the best creature in hand (highest power for aggression).
	var bestIdx int = -1
	var bestPow int = -1
	for i, c := range s.Hand {
		if c == nil {
			continue
		}
		isCreature := false
		for _, t := range c.Types {
			if t == "creature" {
				isCreature = true
				break
			}
		}
		if !isCreature {
			continue
		}
		if c.BasePower > bestPow {
			bestPow = c.BasePower
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, "Sneak Attack", "no_creature_in_hand", nil)
		return
	}

	// Remove from hand and put onto battlefield with haste.
	card := s.Hand[bestIdx]
	s.Hand = append(s.Hand[:bestIdx], s.Hand[bestIdx+1:]...)

	perm := enterBattlefieldWithETB(gs, seat, card, false)
	if perm == nil {
		return
	}
	perm.SummoningSick = false // haste
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["kw:haste"] = 1

	// Register delayed trigger: sacrifice at next end step.
	capturedPerm := perm
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seat,
		SourceCardName: "Sneak Attack",
		EffectFn: func(gs *gameengine.GameState) {
			// Sacrifice the creature if it's still on the battlefield.
			for _, p := range gs.Seats[seat].Battlefield {
				if p == capturedPerm {
					gameengine.SacrificePermanent(gs, capturedPerm, "sneak_attack_end_step")
					return
				}
			}
		},
	})

	emit(gs, slug, "Sneak Attack", map[string]interface{}{
		"seat":     seat,
		"creature": card.DisplayName(),
		"haste":    true,
		"delayed":  "sacrifice_at_next_end_step",
	})
}
