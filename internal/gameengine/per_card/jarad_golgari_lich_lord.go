package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJaradGolgariLichLord wires Jarad, Golgari Lich Lord.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{B}{B}{G}{G}
//	Legendary Creature — Zombie Elf
//	2/2
//	Jarad gets +1/+1 for each creature card in your graveyard.
//	{1}{B}{G}, Sacrifice another creature: Each opponent loses life
//	  equal to the sacrificed creature's power.
//	Sacrifice a Swamp and a Forest: Return this card from your graveyard
//	  to your hand.
//
// Implementation:
//   - ETB / static: recompute Jarad's temp power/toughness from current
//     graveyard creature count. The static buff would normally come from
//     the layers system, but the per-card path lets us refresh on every
//     ETB. emitPartial flags that the buff isn't continuously updated as
//     creatures die mid-turn (next ETB / next eval pass refreshes).
//   - Activated drain: pick the best other creature on the battlefield
//     to sacrifice (highest power), drain each opponent for that power.
//   - Recursion-from-grave swamp+forest sac is land-cost activation —
//     emitPartial; rarely worth simulating (you'd rather cast Jarad fresh).
func registerJaradGolgariLichLord(r *Registry) {
	r.OnETB("Jarad, Golgari Lich Lord", jaradGolgariLichLordETB)
	r.OnActivated("Jarad, Golgari Lich Lord", jaradGolgariLichLordActivate)
}

func jaradGolgariLichLordETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "jarad_golgari_lich_lord_etb_yard_buff"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") {
			count++
		}
	}
	// Static buff: each refresh RESTATES the total rather than adding to
	// it, so a second ETB (flicker) must not stack a second copy.
	setStaticPT(gs, perm, count, count)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"creatures_in_yard": count,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"static_buff_not_continuously_layered_only_refreshed_on_etb")
}

func jaradGolgariLichLordActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "jarad_golgari_lich_lord_drain"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx == 1 {
		emitPartial(gs, "jarad_recursion_from_grave", src.Card.DisplayName(),
			"sac_swamp_and_forest_to_recur_from_graveyard_not_implemented")
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	var fodder *gameengine.Permanent
	bestPower := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil || !p.IsCreature() {
			continue
		}
		pw := p.Power()
		if pw > bestPower {
			bestPower = pw
			fodder = p
		}
	}
	if fodder == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_to_sacrifice", map[string]interface{}{
			"seat": src.Controller,
		})
		return
	}
	power := fodder.Power()
	fodderName := fodder.Card.DisplayName()
	gameengine.SacrificePermanent(gs, fodder, "jarad_drain")
	for i, opp := range gs.Seats {
		if opp == nil || opp.Lost || i == src.Controller {
			continue
		}
		gameengine.LoseLife(gs, i, power, src.Card.DisplayName())
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       src.Controller,
		"sacrificed": fodderName,
		"power":      power,
	})
	_ = gs.CheckEnd()
}
