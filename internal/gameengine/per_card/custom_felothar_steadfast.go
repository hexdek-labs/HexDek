package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFelotharSteadfastCustom replaces the auto-gen activated stub
// with the actual sacrifice-draw-discard.
//
// Oracle text:
//
//	Each creature you control assigns combat damage equal to its
//	toughness rather than its power.
//	Creatures you control can attack as though they didn't have defender.
//	{3}, {T}, Sacrifice another creature: Draw cards equal to the
//	sacrificed creature's toughness, then discard cards equal to its power.
//
// We pick the sac target as the creature we control with the highest
// toughness-minus-power delta (best draw/discard ratio). The {3} +
// tap cost is engine-enforced; we sub the mana directly from
// ManaPool to be defensive about double-spending. Discard picks the
// lowest-CMC cards in hand to minimize value loss.
//
// The "toughness as damage" + "ignore defender" statics are engine-
// side (combat damage assignment + attack-eligibility layers); we
// surface a partial breadcrumb when the engine handler hasn't yet
// learned them.
func registerFelotharSteadfastCustom(r *Registry) {
	r.OnActivated("Felothar the Steadfast", felotharSacDrawDiscard)
}

func felotharSacDrawDiscard(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "felothar_sac_draw_discard"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	// {3}, {T}, Sacrifice another creature — defensive cost gates for
	// non-engine callers.
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	if seat.ManaPool < 3 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seat.ManaPool,
		})
		return
	}
	// Find sacrifice target — highest (toughness - power) creature we
	// control that isn't Felothar herself.
	var sacrifice *gameengine.Permanent
	bestDelta := -999
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		delta := p.Card.BaseToughness - p.Card.BasePower
		if delta > bestDelta {
			sacrifice = p
			bestDelta = delta
		}
	}
	if sacrifice == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_sacrifice_target", nil)
		return
	}
	tough := sacrifice.Card.BaseToughness
	power := sacrifice.Card.BasePower
	if tough < 0 {
		tough = 0
	}
	if power < 0 {
		power = 0
	}
	seat.ManaPool -= 3
	src.Tapped = true
	// Sacrifice → graveyard.
	gameengine.MoveCard(gs, sacrifice.Card, sacrifice.Controller, "battlefield", "graveyard", "felothar_sacrifice")
	// Draw cards == toughness.
	drawn := 0
	for i := 0; i < tough; i++ {
		if drawOne(gs, src.Controller, src.Card.DisplayName()) != nil {
			drawn++
		}
	}
	// Discard cards == power, lowest-CMC first.
	discarded := 0
	for i := 0; i < power; i++ {
		if len(seat.Hand) == 0 {
			break
		}
		// Pick lowest-CMC card in hand.
		pickIdx := 0
		bestCMC := cardCMC(seat.Hand[0])
		for j, c := range seat.Hand {
			if cardCMC(c) < bestCMC {
				bestCMC = cardCMC(c)
				pickIdx = j
			}
		}
		c := seat.Hand[pickIdx]
		gameengine.DiscardCard(gs, c, src.Controller)
		discarded++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"sacrificed": sacrifice.Card.DisplayName(),
		"drew":      drawn,
		"discarded": discarded,
	})
	emitPartial(gs, "felothar_combat_damage_toughness", src.Card.DisplayName(),
		"static effects: assign-damage-as-toughness + ignore-defender are engine-layer; not yet enforced")
}
