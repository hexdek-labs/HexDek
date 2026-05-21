package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSolphimMayhemDominus wires Solphim, Mayhem Dominus.
//
// Oracle text (Scryfall, verified — Phyrexia: All Will Be One):
//
//	If a source you control would deal noncombat damage to an
//	opponent or a permanent an opponent controls, it deals double
//	that damage to that player or permanent instead.
//	{1}{R/P}{R/P}, Discard two cards: Put an indestructible counter
//	on Solphim. ({R/P} can be paid with either {R} or 2 life.)
//
// Implementation:
//   - Damage-doubling static: per-seat flag at ETB, engine-layer
//     DealDamage hook reads "noncombat_damage_doubler_count".
//   - Activated cost — r51 fix: the prior handler called MoveCard
//     (battlefield→exile) on two other creatures, which is wrong on
//     two fronts:
//       (a) printed cost is "Discard two cards", not "Exile two
//           creatures".
//       (b) MoveCard from battlefield is a no-op for Permanent
//           cleanup (sibling of the Mondrak game-59 CardIdentity leak).
//     Fixed by routing two cards from hand → graveyard via MoveCard
//     (a discard-cost-shaped path; clear of the battlefield→exile
//     anti-pattern). Counter count corrected from 2 to 1, mana gate
//     corrected from 4 to 3 ({1}{R/P}{R/P} = 3 generic-equivalent).
func registerSolphimMayhemDominus(r *Registry) {
	r.OnETB("Solphim, Mayhem Dominus", solphimSetDamageDoublerFlag)
	r.OnActivated("Solphim, Mayhem Dominus", solphimIndestructibleActivate)
}

func solphimSetDamageDoublerFlag(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "solphim_noncombat_damage_doubler_flag"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["noncombat_damage_doubler_count"]++
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"doublers": seat.Flags["noncombat_damage_doubler_count"],
	})
	emitPartial(gs, "solphim_doubles_noncombat_damage", perm.Card.DisplayName(),
		"noncombat damage doubling requires DealDamage replacement-effect hook; flag set for downstream")
}

func solphimIndestructibleActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "solphim_indestructible_activate"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	const manaCost = 3 // {1}{R/P}{R/P} → 3 generic-equivalent
	if seat.ManaPool < manaCost {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seat.ManaPool,
			"mana_cost": manaCost,
		})
		return
	}
	if len(seat.Hand) < 2 {
		emitFail(gs, slug, src.Card.DisplayName(), "fewer_than_2_cards_in_hand", map[string]interface{}{
			"hand_size": len(seat.Hand),
		})
		return
	}
	seat.ManaPool -= manaCost
	gameengine.SyncManaAfterSpend(seat)
	// Discard two: pick the two highest-CMC cards (cheapest to lose
	// strategically — anything castable this turn stays in hand).
	// Tiebreak by index (older cards picked first).
	pickDiscard := func() *gameengine.Card {
		var best *gameengine.Card
		bestCMC := -1
		for _, c := range seat.Hand {
			if c == nil {
				continue
			}
			cmc := gameengine.ManaCostOf(c)
			if cmc > bestCMC {
				bestCMC = cmc
				best = c
			}
		}
		return best
	}
	d1 := pickDiscard()
	if d1 != nil {
		gameengine.MoveCard(gs, d1, src.Controller, "hand", "graveyard", "solphim_discard_cost")
	}
	d2 := pickDiscard()
	if d2 != nil {
		gameengine.MoveCard(gs, d2, src.Controller, "hand", "graveyard", "solphim_discard_cost")
	}
	seat.Turn.Discarded += 2
	src.AddCounter("indestructible", 1)
	discarded := []string{}
	if d1 != nil {
		discarded = append(discarded, d1.DisplayName())
	}
	if d2 != nil {
		discarded = append(discarded, d2.DisplayName())
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           src.Controller,
		"discarded":      discarded,
		"indestructible": src.Counters["indestructible"],
	})
}
