package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAragornTheUniter wires Aragorn, the Uniter.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Whenever you cast a multicolored spell, choose one that hasn't been
//	chosen for each color of that spell —
//	  • You gain 5 life.
//	  • Scry 2.
//	  • Aragorn deals 3 damage to any target.
//	  • Create a 1/1 green Human Soldier creature token.
//
// Implementation simplification (per task spec):
//   - Trigger on "spell_cast" controlled by Aragorn's controller, gated on
//     a multicolored spell (len(Colors) >= 2).
//   - Fire one mode per WURG color the spell has, mapping
//     W→gain 5, U→scry 2, R→3 damage to chosen opponent, G→Human Soldier.
//   - "Hasn't been chosen" is implicit: each color-mode is fired at most
//     once per spell since each color is a set member. Duplicate WURG
//     entries in Colors are deduped.
//   - Damage target: highest-life living opponent (deterministic, no
//     hat callback needed for the hat to still benefit).
func registerAragornTheUniter(r *Registry) {
	r.OnTrigger("Aragorn, the Uniter", "spell_cast", aragornUniterSpellCast)
}

func aragornUniterSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aragorn_uniter_multicolored_modes"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if len(card.Colors) < 2 {
		return
	}

	colors := map[string]bool{}
	for _, c := range card.Colors {
		colors[strings.ToUpper(strings.TrimSpace(c))] = true
	}

	modes := []string{}
	if colors["W"] {
		gameengine.GainLife(gs, perm.Controller, 5, perm.Card.DisplayName())
		modes = append(modes, "white_gain_5")
	}
	if colors["U"] {
		gameengine.Scry(gs, perm.Controller, 2)
		modes = append(modes, "blue_scry_2")
	}
	if colors["R"] {
		if target := aragornPickDamageTarget(gs, perm.Controller); target >= 0 {
			opp := gs.Seats[target]
			if opp != nil && !opp.Lost {
				opp.Life -= 3
				gs.LogEvent(gameengine.Event{
					Kind:   "damage",
					Seat:   perm.Controller,
					Target: target,
					Source: perm.Card.DisplayName(),
					Amount: 3,
					Details: map[string]interface{}{
						"slug": slug,
						"mode": "red_3_damage",
					},
				})
				modes = append(modes, "red_3_damage")
			}
		}
	}
	if colors["G"] {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Human Soldier",
			[]string{"creature", "human", "soldier", "pip:G"}, 1, 1)
		modes = append(modes, "green_human_soldier")
	}

	if len(modes) == 0 {
		return
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"spell_name": card.DisplayName(),
		"colors":     card.Colors,
		"modes":      modes,
	})
	_ = gs.CheckEnd()
}

// aragornPickDamageTarget returns the seat index of the highest-life
// living opponent, or -1 if none exists.
func aragornPickDamageTarget(gs *gameengine.GameState, controller int) int {
	bestSeat := -1
	bestLife := -1 << 30
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == controller {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			bestSeat = i
		}
	}
	return bestSeat
}
