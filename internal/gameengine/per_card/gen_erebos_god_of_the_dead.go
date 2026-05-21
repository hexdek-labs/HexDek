package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerErebosGodOfTheDead wires Erebos, God of the Dead.
//
// Oracle text:
//
//	Indestructible
//	As long as your devotion to black is less than five, Erebos isn't a creature. (Each {B} in the mana costs of permanents you control counts toward your devotion to black.)
//	Your opponents can't gain life.
//	{1}{B}, Pay 2 life: Draw a card.
//
// R49 stub-batch-E port (defensive utility — lifegain denial):
//   - "Your opponents can't gain life" — register a §614
//     `would_gain_life` replacement on ETB. Applies when target seat is
//     an opponent; sets ev.Count(0) so the gain is reduced to zero.
//     UnregisterReplacementsForPermanent on LTB auto-cleans.
//   - Existing {1}{B}, Pay 2 life: Draw a card activation kept intact.
//   - Indestructible + devotion creature-toggle: AST keyword + layer-7
//     CDA territory.
func registerErebosGodOfTheDead(r *Registry) {
	r.OnETB("Erebos, God of the Dead", erebosETBRegisterLifegainDenial)
	r.OnActivated("Erebos, God of the Dead", erebosGodOfTheDeadActivate)
}

func erebosETBRegisterLifegainDenial(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "erebos_opponents_cant_gain_life"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_gain_life",
		HandlerID:      "erebos_god_of_the_dead:no_opp_lifegain:" + perm.Card.DisplayName(),
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev.TargetSeat == perm.Controller {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			prev := ev.Count()
			ev.SetCount(0)
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   perm.Controller,
				Source: "Erebos, God of the Dead",
				Details: map[string]interface{}{
					"rule":           "614",
					"effect":         "opp_cant_gain_life",
					"original_count": prev,
					"target_seat":    ev.TargetSeat,
				},
			})
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func erebosGodOfTheDeadActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "erebos_god_of_the_dead_activate"
	if gs == nil || src == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}
	// Cost gate: {1}{B}, Pay 2 life. Verify the seat can pay both the
	// mana and life portions before debiting either, so a half-paid
	// activation can never give a free draw.
	const manaCost = 2 // {1}{B} → 2 generic-equivalent
	const lifeCost = 2
	if seat.Life <= lifeCost {
		// Must have STRICTLY more life than the cost so the seat doesn't
		// kill itself paying — CR §119.4 (cost can't reduce life below 0
		// and an activation only resolves if all costs are paid in full).
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_life", map[string]interface{}{
			"life":      seat.Life,
			"life_cost": lifeCost,
		})
		return
	}
	if !gameengine.PayGenericCost(gs, seat, manaCost, "activated", "erebos_activate", src.Card.DisplayName()) {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seat.ManaPool,
			"mana_cost": manaCost,
		})
		return
	}
	gameengine.LoseLife(gs, seatIdx, lifeCost, src.Card.DisplayName())
	drawOne(gs, seatIdx, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"mana_paid": manaCost,
		"life_paid": lifeCost,
	})
}
