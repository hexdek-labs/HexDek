package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Niambi, Esteemed Speaker — {1}{W} 2/3 Legendary Creature — Human
// Cleric.
//
//   Flash
//   When Niambi enters, you may return another target creature you
//   control to its owner's hand. If you do, you gain life equal to that
//   creature's mana value.
//   {1}{W}{U}, {T}, Discard a legendary card: Draw two cards.
//
// Implementation:
//   - OnETB: pick the highest-CMC OTHER creature controlled (bounce
//     value scales with MV); if any exists, BouncePermanent → hand,
//     GainLife(cmc).
//   - Activated draw-two with discard-legendary cost: cost-pay is engine
//     territory (discard-a-legendary as part of activation cost). When
//     this handler fires the engine has already paid the cost, so the
//     handler just draws 2.

func init() {
	registerNiambiEsteemedSpeaker(Global())
	AddResetHook(registerNiambiEsteemedSpeaker)
}

func registerNiambiEsteemedSpeaker(r *Registry) {
	r.OnETB("Niambi, Esteemed Speaker", niambiETB)
	r.OnActivated("Niambi, Esteemed Speaker", niambiActivate)
}

func niambiETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "niambi_etb_bounce_and_gain"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	var pick *gameengine.Permanent
	bestCMC := -1
	for _, p := range s.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc > bestCMC {
			bestCMC = cmc
			pick = p
		}
	}
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"reason": "no_eligible_creature_to_bounce",
		})
		return
	}
	cmcVal := cardCMC(pick.Card)
	pickName := pick.Card.DisplayName()
	gameengine.BouncePermanent(gs, pick, perm, "hand")
	if cmcVal > 0 {
		gameengine.GainLife(gs, seat, cmcVal, perm.Card.DisplayName())
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"bounced":      pickName,
		"life_gained":  cmcVal,
	})
}

func niambiActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "niambi_draw_two"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := src.Controller
	drawn := 0
	for i := 0; i < 2; i++ {
		if drawOne(gs, seat, src.Card.DisplayName()) != nil {
			drawn++
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"drawn": drawn,
	})
}
