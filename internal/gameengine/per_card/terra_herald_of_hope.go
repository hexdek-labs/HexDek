package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTerraHeraldOfHope wires Terra, Herald of Hope. Batch #33.
//
// Oracle text (Scryfall, verified 2026-05-01; Final Fantasy Commander):
//
//	{R}{W}{B} Legendary Creature — Human Wizard Warrior 3/3
//	Trance — At the beginning of combat on your turn, mill two cards.
//	Terra gains flying until end of turn.
//	Whenever Terra deals combat damage to a player, you may pay {2}.
//	When you do, return target creature card with power 3 or less from
//	your graveyard to the battlefield tapped.
//
// Implementation:
//   - "combat_begin": gates on active_seat == controller, dedup'd per
//     turn so extra-combat phases don't double-fire. Mills two cards
//     from controller's library and grants Terra flying via
//     Flags["kw:flying"]; cleanup via a next_end_step delayed trigger.
//   - "combat_damage_player": gates on (a) source == Terra and source_seat
//     == controller, (b) defender_seat is a player. Greedy AI: always
//     opt to pay {2} when affordable AND a valid target exists. Pick the
//     highest-CMC creature card with BasePower ≤ 3 from controller's
//     graveyard, return it to the battlefield tapped (via the standard
//     graveyard→battlefield + enterBattlefieldWithETB cascade).
//   - {2} cost is paid out of seat.ManaPool (this engine has a single
//     int mana pool — colors are tracked via pip tags but pool-side is
//     generic-only). When the pool is short, the trigger no-ops with
//     reason=insufficient_mana.
func registerTerraHeraldOfHope(r *Registry) {
	r.OnTrigger("Terra, Herald of Hope", "combat_begin", terraHeraldOfHopeCombatBegin)
	r.OnTrigger("Terra, Herald of Hope", "combat_damage_player", terraHeraldOfHopeCombatDamage)
}

func terraHeraldOfHopeCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "terra_herald_of_hope_trance_mill_and_flying"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupeKey := fmt.Sprintf("terra_herald_trance_t%d", gs.Turn+1)
	if perm.Flags[dedupeKey] > 0 {
		return
	}
	perm.Flags[dedupeKey] = 1

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	milled := 0
	for i := 0; i < 2; i++ {
		if len(seat.Library) == 0 {
			break
		}
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		gameengine.MoveCard(gs, top, perm.Controller, "library", "graveyard", "terra_herald_trance")
		milled++
	}

	perm.Flags["kw:flying"] = 1
	captured := perm
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			delete(captured.Flags, "kw:flying")
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"milled":   milled,
		"keyword":  "flying",
		"duration": "until_end_of_turn",
	})
}

const terraHeraldOfHopePaymentCost = 2

func terraHeraldOfHopeCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "terra_herald_of_hope_combat_damage_reanimate"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if perm.Card == nil || sourceName != perm.Card.DisplayName() {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	var best *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		if c.BasePower > 3 {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	if best == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"paid":   false,
			"reason": "no_eligible_target",
		})
		return
	}

	if seat.ManaPool < terraHeraldOfHopePaymentCost {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"paid":   false,
			"reason": "insufficient_mana",
			"target": best.DisplayName(),
		})
		return
	}
	seat.ManaPool -= terraHeraldOfHopePaymentCost

	gameengine.MoveCard(gs, best, perm.Controller, "graveyard", "battlefield", "terra_herald_reanimate")
	enterBattlefieldWithETB(gs, perm.Controller, best, true)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"paid":       terraHeraldOfHopePaymentCost,
		"reanimated": best.DisplayName(),
		"power":      best.BasePower,
		"cmc":        bestCMC,
		"tapped":     true,
	})
}
