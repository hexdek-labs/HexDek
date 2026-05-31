package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Deadbridge Chant — {4}{B}{G} Enchantment.
//   When this enchantment enters, mill ten cards.
//   At the beginning of your upkeep, choose a card at random in your
//   graveyard. If it's a creature card, put it onto the battlefield.
//   Otherwise, put it into your hand.
//
// Random choice uses gs.Rng for determinism. The ETB mill is a one-shot
// MoveCard sweep from the top of library to graveyard; the upkeep arm
// reads from the controller's graveyard and routes creatures to the
// battlefield (with full ETB cascade) or noncreatures to hand.

func init() {
	registerDeadbridgeChant(Global())
	AddResetHook(registerDeadbridgeChant)
}

func registerDeadbridgeChant(r *Registry) {
	r.OnETB("Deadbridge Chant", deadbridgeChantETB)
	r.OnTrigger("Deadbridge Chant", "upkeep_controller", deadbridgeChantUpkeep)
}

func deadbridgeChantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "deadbridge_chant_etb_mill"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	milled := 0
	for i := 0; i < 10 && len(s.Library) > 0; i++ {
		top := s.Library[0]
		if top == nil {
			break
		}
		moveCardBetweenZones(gs, seat, top, "library", "graveyard", "deadbridge_chant_etb_mill")
		milled++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"milled": milled,
	})
	gs.LogEvent(gameengine.Event{
		Kind:   "mill",
		Seat:   seat,
		Target: seat,
		Source: perm.Card.DisplayName(),
		Amount: milled,
	})
}

func deadbridgeChantUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "deadbridge_chant_upkeep"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost || len(s.Graveyard) == 0 {
		return
	}
	idx := 0
	if gs.Rng != nil && len(s.Graveyard) > 1 {
		idx = gs.Rng.Intn(len(s.Graveyard))
	}
	picked := s.Graveyard[idx]
	if picked == nil {
		return
	}
	if cardHasType(picked, "creature") && gameengine.CardCanEnterBattlefield(picked) {
		moveCardBetweenZones(gs, seat, picked, "graveyard", "battlefield", "deadbridge_chant_reanimate")
		enterBattlefieldWithETB(gs, seat, picked, false)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":               seat,
			"picked":             picked.DisplayName(),
			"put_on_battlefield": true,
		})
		return
	}
	moveCardBetweenZones(gs, seat, picked, "graveyard", "hand", "deadbridge_chant_to_hand")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"picked":      picked.DisplayName(),
		"put_in_hand": true,
	})
}
