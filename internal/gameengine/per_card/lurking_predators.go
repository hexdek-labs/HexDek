package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Lurking Predators — {4}{G}{G} Enchantment. Whenever an opponent casts
// a spell, reveal the top card of your library. If it's a creature card,
// put it onto the battlefield. Otherwise, you may put that card on the
// bottom of your library.
//
// Engine fires "spell_cast_by_opponent" unconditionally from stack.go's
// fireCastTriggersFromZone; handler filters ctx["caster_seat"] against
// perm.Controller to scope to opponent casts only. Greedy policy on the
// nonpermanent "may bottom" — always elect yes (the typical Lurking
// Predators line is "I don't want this brick on top blocking the next
// reveal"). Stays revealed-on-top if greedy-no was chosen, but the
// handler always greedy-yes.

func init() {
	registerLurkingPredators(Global())
	AddResetHook(registerLurkingPredators)
}

func registerLurkingPredators(r *Registry) {
	r.OnTrigger("Lurking Predators", "spell_cast_by_opponent", lurkingPredatorsTrigger)
}

func lurkingPredatorsTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lurking_predators"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat == perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost || len(s.Library) == 0 {
		return
	}
	top := s.Library[0]
	if top == nil {
		return
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "reveal",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"card":         top.DisplayName(),
			"from_library": seat,
		},
	})
	if cardHasType(top, "creature") && gameengine.CardCanEnterBattlefield(top) {
		enterBattlefieldWithETB(gs, seat, top, false)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":            seat,
			"caster_seat":     casterSeat,
			"revealed_card":   top.DisplayName(),
			"put_on_battlefield": true,
		})
		return
	}
	// Greedy yes — put on bottom.
	s.Library = s.Library[1:]
	s.Library = append(s.Library, top)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             seat,
		"caster_seat":      casterSeat,
		"revealed_card":    top.DisplayName(),
		"put_on_bottom":    true,
	})
}
