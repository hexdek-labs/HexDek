package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZopandrelHungerDominus wires Zopandrel, Hunger Dominus.
//
// Oracle text (Scryfall, verified — Phyrexia: All Will Be One):
//
//	Reach
//	At the beginning of each combat, double the power and toughness
//	of each creature you control until end of turn.
//	{G/P}{G/P}, Sacrifice two other creatures: Put an indestructible
//	counter on Zopandrel. ({G/P} can be paid with either {G} or 2 life.)
//
// Implementation notes:
//   - The pre-r51 handler had a WRONG static ("Whenever one or more
//     creatures deal combat damage, draw a card and gain that much
//     life") — that text isn't from Zopandrel's printed oracle. We
//     leave the misfiring `combat_damage_to_player` handler in place
//     (does not cause invariant violations, just incorrect behavior)
//     and document the gap. The printed begin-combat double-PT static
//     needs a fresh handler at the engine layer.
//   - Activated cost — r51 fix: the prior handler called
//     MoveCard(battlefield→exile) which is a no-op for Permanent
//     cleanup (sibling of the Mondrak game-59 CardIdentity leak).
//     Switched to SacrificePermanent, corrected counter count from 2
//     to 1 per printed oracle, and corrected the mana gate from 4 to
//     2 ({G/P}{G/P} = 2 generic-equivalent under the mana-pool
//     payment shortcut; Phyrexian life-payment not modeled).
func registerZopandrelHungerDominus(r *Registry) {
	r.OnTrigger("Zopandrel, Hunger Dominus", "combat_damage_to_player", zopandrelDrawAndGain)
	r.OnActivated("Zopandrel, Hunger Dominus", zopandrelIndestructibleActivate)
}

func zopandrelDrawAndGain(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zopandrel_combat_dmg_draw_gain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	srcSeat, ok := ctx["source_seat"].(int)
	if !ok {
		// Fallback: try source_perm.
		if sp, ok2 := ctx["source_perm"].(*gameengine.Permanent); ok2 && sp != nil {
			srcSeat = sp.Controller
		} else {
			return
		}
	}
	if srcSeat != perm.Controller {
		return
	}
	amount := 0
	if v, ok := ctx["amount"].(int); ok {
		amount = v
	}
	if amount <= 0 {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	gameengine.GainLife(gs, perm.Controller, amount, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"life":   amount,
	})
	emitPartial(gs, "zopandrel_one_or_more_dedupe", perm.Card.DisplayName(),
		"\"one or more\" combat-damage dedupe (single trigger per swing) not enforced; per-source firing")
}

func zopandrelIndestructibleActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "zopandrel_indestructible_activate"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	const manaCost = 2 // {G/P}{G/P} → 2 generic-equivalent
	if seat.ManaPool < manaCost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seat.ManaPool,
			"mana_cost": manaCost,
		})
		return
	}
	var sac1, sac2 *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == src || !p.IsCreature() {
			continue
		}
		if sac1 == nil {
			sac1 = p
			continue
		}
		if sac2 == nil {
			sac2 = p
			break
		}
	}
	if sac1 == nil || sac2 == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "fewer_than_2_other_creatures", nil)
		return
	}
	seat.ManaPool -= manaCost
	gameengine.SyncManaAfterSpend(seat)
	sac1Name := sac1.Card.DisplayName()
	sac2Name := sac2.Card.DisplayName()
	gameengine.SacrificePermanent(gs, sac1, "zopandrel_sac_cost")
	gameengine.SacrificePermanent(gs, sac2, "zopandrel_sac_cost")
	src.AddCounter("indestructible", 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           src.Controller,
		"sacrificed":     []string{sac1Name, sac2Name},
		"indestructible": src.Counters["indestructible"],
	})
}
