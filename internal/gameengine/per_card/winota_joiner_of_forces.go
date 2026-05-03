package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWinotaJoinerOfForces wires Winota, Joiner of Forces.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Whenever a non-Human creature you control attacks, look at the top
//	six cards of your library. You may put a Human creature card from
//	among them onto the battlefield tapped and attacking. It gains
//	indestructible until end of turn. Put the rest of the cards on the
//	bottom of your library in a random order.
//
// Implementation:
//   - "creature_attacks" trigger: filter on attacker controlled by
//     Winota's controller AND the attacker is a non-Human creature.
//     (Winota herself is a Human Soldier, so she does not trigger her
//     own ability.) Pop the top six cards of the controller's library,
//     scan for the first Human creature, and cheat it onto the
//     battlefield tapped, attacking the same defender as the trigger
//     attacker. The cheated Human gains indestructible until end of
//     turn via the "kw:indestructible" Flags key + a one-shot
//     DelayedTrigger at "next_end_step" to revoke it. Remaining cards
//     are returned to the bottom of the library in shuffled order.
//
// emitPartial gaps:
//   - "You may put" — the AI always puts the Human onto the battlefield
//     (pure upside); the optional clause is not modeled.
func registerWinotaJoinerOfForces(r *Registry) {
	r.OnTrigger("Winota, Joiner of Forces", "creature_attacks", winotaAttackTrigger)
}

func winotaAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "winota_non_human_attack_cheat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat != perm.Controller || atk == nil || atk.Card == nil {
		return
	}
	if !atk.IsCreature() {
		return
	}
	// "non-Human" — exclude attackers with the Human subtype.
	for _, t := range atk.Card.Types {
		if strings.EqualFold(t, "human") {
			return
		}
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Look at top six cards.
	look := 6
	if look > len(seat.Library) {
		look = len(seat.Library)
	}
	if look == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"attacker": atk.Card.DisplayName(),
			"reason":   "library_empty",
		})
		return
	}
	top := append([]*gameengine.Card(nil), seat.Library[:look]...)

	// Find first Human creature.
	humanIdx := -1
	for i, c := range top {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if !cardHasType(c, "human") {
			continue
		}
		humanIdx = i
		break
	}

	defenderSeat := -1
	if d, ok := gameengine.AttackerDefender(atk); ok {
		defenderSeat = d
	}
	if defenderSeat < 0 {
		for _, opp := range gs.LivingOpponents(perm.Controller) {
			defenderSeat = opp
			break
		}
	}

	cheated := ""
	if humanIdx >= 0 {
		human := top[humanIdx]
		// Remove the chosen card from the library by pointer.
		for i, c := range seat.Library {
			if c == human {
				seat.Library = append(seat.Library[:i], seat.Library[i+1:]...)
				break
			}
		}
		// Drop from our local snapshot so the rest go to the bottom.
		top = append(top[:humanIdx], top[humanIdx+1:]...)
		look--

		newPerm := enterBattlefieldWithETB(gs, perm.Controller, human, true)
		if newPerm != nil {
			newPerm.SummoningSick = false
			if newPerm.Flags == nil {
				newPerm.Flags = map[string]int{}
			}
			// Grant indestructible until end of turn.
			newPerm.Flags["kw:indestructible"] = 1
			if defenderSeat >= 0 {
				newPerm.Flags["attacking"] = 1
				gameengine.SetAttackerDefender(newPerm, defenderSeat)
			}
			// Register a next_end_step delayed trigger to revoke
			// indestructible, bounding the "until end of turn" window.
			captured := newPerm
			gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
				TriggerAt:      "next_end_step",
				ControllerSeat: perm.Controller,
				SourceCardName: perm.Card.DisplayName() + " (indestructible cleanup)",
				OneShot:        true,
				EffectFn: func(gs *gameengine.GameState) {
					if captured == nil || captured.Flags == nil {
						return
					}
					delete(captured.Flags, "kw:indestructible")
				},
			})
		}
		cheated = human.DisplayName()
		gs.LogEvent(gameengine.Event{
			Kind:   "winota_cheat",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"card":           human.DisplayName(),
				"attacker":       atk.Card.DisplayName(),
				"defender_seat":  defenderSeat,
				"tapped":         true,
				"attacking":      true,
				"indestructible": true,
			},
		})
	}

	// Remaining looked-at cards go to the bottom of the library in random
	// order. Remove the top `look` from the library (already shrunk if we
	// pulled the human).
	rest := append([]*gameengine.Card(nil), seat.Library[:look]...)
	seat.Library = seat.Library[look:]
	if gs.Rng != nil && len(rest) > 1 {
		gs.Rng.Shuffle(len(rest), func(i, j int) {
			rest[i], rest[j] = rest[j], rest[i]
		})
	}
	seat.Library = append(seat.Library, rest...)

	totalLooked := len(rest)
	if cheated != "" {
		totalLooked++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"attacker":      atk.Card.DisplayName(),
		"looked":        totalLooked,
		"cheated":       cheated,
		"bottomed":      len(rest),
		"defender_seat": defenderSeat,
	})
}
