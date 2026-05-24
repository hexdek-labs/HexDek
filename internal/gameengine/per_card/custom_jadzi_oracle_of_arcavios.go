package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJadziOracleOfArcaviosCustom wires the magecraft top-of-library
// reveal and the "Discard a card: Return Jadzi to your hand" bounce
// activation. The auto-generated stub
// registerJadziOracleOfArcaviosJourneyToTheOracle in the matching
// gen_*.go remains in place — both handlers fire (its body only emits
// a partial).
//
// Oracle text (Strixhaven, {2}{G}{U}{U} front face):
//
//	Discard a card: Return Jadzi to its owner's hand.
//	Magecraft — Whenever you cast or copy an instant or sorcery spell,
//	reveal the top card of your library. If it's a nonland card, you
//	may cast it by paying {1} rather than paying its mana cost. If
//	it's a land card, put it onto the battlefield.
//
// Implementation:
//   - OnTrigger("instant_or_sorcery_cast") gated on caster == Jadzi's
//     controller. Reveal top of library. If land, MoveCard onto
//     battlefield as a normal land entry. If nonland, we don't have a
//     "cast for {1}" alt-cost path through per-card hooks, so we move
//     the card to hand as a stand-in for the value (the player can
//     still cast it on a later turn) and emitPartial.
//   - OnActivated(0): "Discard a card: Return Jadzi to its owner's
//     hand." Discard the lowest-CMC non-land from hand, then bounce
//     Jadzi.
func registerJadziOracleOfArcaviosCustom(r *Registry) {
	jadziName := "Jadzi, Oracle of Arcavios // Journey to the Oracle"
	r.OnTrigger(jadziName, "instant_or_sorcery_cast", jadziMagecraftReveal)
	r.OnActivated(jadziName, jadziDiscardBounce)
	// Also bind under the front-face short name for engines that
	// dispatch by single-face name.
	r.OnTrigger("Jadzi, Oracle of Arcavios", "instant_or_sorcery_cast", jadziMagecraftReveal)
	r.OnActivated("Jadzi, Oracle of Arcavios", jadziDiscardBounce)
}

func jadziMagecraftReveal(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jadzi_magecraft_reveal_top"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Library) == 0 {
		return
	}
	top := seat.Library[0]
	if top == nil {
		return
	}
	if cardHasType(top, "land") {
		gameengine.MoveCard(gs, top, seatIdx, "library", "battlefield", "jadzi_magecraft_land")
		// Wrap as a permanent and fire ETB.
		newPerm := createPermanent(gs, seatIdx, top, false)
		if newPerm != nil {
			gameengine.RegisterReplacementsForPermanent(gs, newPerm)
			gameengine.FirePermanentETBTriggers(gs, newPerm)
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seatIdx,
			"card":   top.DisplayName(),
			"action": "land_to_battlefield",
		})
		return
	}
	// Nonland — cast-for-{1} alt cost isn't on the per-card path.
	// Stand-in: move to hand so the value isn't lost.
	gameengine.MoveCard(gs, top, seatIdx, "library", "hand", "jadzi_magecraft_to_hand_partial")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seatIdx,
		"card":   top.DisplayName(),
		"action": "nonland_to_hand_partial",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"alt_cast_for_one_mana_not_modeled_card_routed_to_hand")
}

func jadziDiscardBounce(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "jadzi_discard_bounce"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
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
	if len(seat.Hand) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_hand_to_discard", nil)
		return
	}

	// Pick the lowest-CMC non-land from hand to discard.
	pickIdx := -1
	pickCMC := 1<<31 - 1
	for i, c := range seat.Hand {
		if c == nil {
			continue
		}
		if cardHasType(c, "land") {
			continue
		}
		cmc := cardCMC(c)
		if cmc < pickCMC {
			pickCMC = cmc
			pickIdx = i
		}
	}
	if pickIdx < 0 {
		// Only lands in hand — discard the lowest-index land.
		for i, c := range seat.Hand {
			if c != nil {
				pickIdx = i
				break
			}
		}
	}
	if pickIdx < 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_card_to_discard", nil)
		return
	}
	pick := seat.Hand[pickIdx]
	gameengine.DiscardCard(gs, pick, seatIdx)

	// Return Jadzi to owner's hand.
	owner := src.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		owner = src.Controller
	}
	card := src.Card
	removePermanent(gs, src)
	gs.UnregisterReplacementsForPermanent(src)
	gs.UnregisterContinuousEffectsForPermanent(src)
	gameengine.FireZoneChangeTriggers(gs, src, card, "battlefield", "hand")
	gameengine.DetachAll(gs, src)
	gs.Seats[owner].Hand = append(gs.Seats[owner].Hand, card)

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seatIdx,
		"discarded": pick.DisplayName(),
		"bounced":   card.DisplayName(),
	})
}
