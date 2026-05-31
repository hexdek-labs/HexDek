package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Chulane, Teller of Tales — {1}{G}{W}{U} 2/4 Legendary Creature — Human
// Druid.
//
//   Vigilance
//   Whenever you cast a creature spell, draw a card, then you may put a
//   land card from your hand onto the battlefield.
//   {T}, {3}: Return target creature you control to its owner's hand.
//
// Cast trigger: scoped to caster_seat == controller. Draw is mandatory;
// land drop from hand is "may" — greedy yes (the typical Chulane line is
// "every cast = land + card", which is the whole engine). Vigilance is
// handled by the keyword pipeline; the activated bounce is left to AST
// dispatch (standard targeted owner-return).

func init() {
	registerChulaneTellerOfTales(Global())
	AddResetHook(registerChulaneTellerOfTales)
}

func registerChulaneTellerOfTales(r *Registry) {
	r.OnTrigger("Chulane, Teller of Tales", "creature_spell_cast", chulaneCreatureCast)
}

func chulaneCreatureCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "chulane_creature_cast"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	drawn := drawOne(gs, seat, perm.Card.DisplayName())
	landPlayed := ""
	for _, c := range s.Hand {
		if c != nil && cardHasType(c, "land") {
			moveCardBetweenZones(gs, seat, c, "hand", "battlefield", "chulane_land_drop")
			if gameengine.CardCanEnterBattlefield(c) {
				enterBattlefieldWithETB(gs, seat, c, false)
			}
			landPlayed = c.DisplayName()
			break
		}
	}
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"caster_seat":  casterSeat,
		"drawn":        drawnName,
		"land_played":  landPlayed,
	})
}
