package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerObNixilisCaptive wires Ob Nixilis, Captive Kingpin.
//
// Oracle text:
//
//	Flying, trample.
//	Whenever one or more opponents each lose exactly 1 life, put a
//	+1/+1 counter on Ob Nixilis, Captive Kingpin. Exile the top card
//	of your library. Until your next end step, you may play that card.
//
// Triggers on the canonical "life_lost" event with ctx["amount"] == 1
// and ctx["seat"] != perm.Controller. Each qualifying loss bumps a +1/+1
// counter and impulse-exiles the top card. The "until your next end
// step" duration is approximated by ZoneCastGrant living until removed
// (matches meria.go's impulse pattern).
func registerObNixilisCaptive(r *Registry) {
	r.OnTrigger("Ob Nixilis, Captive Kingpin", "life_lost", obNixilisCaptiveTrigger)
}

func obNixilisCaptiveTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ob_nixilis_captive_kingpin_grow_impulse"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	lossSeat, ok := ctx["seat"].(int)
	if !ok {
		return
	}
	if lossSeat == perm.Controller {
		return
	}
	if lossSeat < 0 || lossSeat >= len(gs.Seats) {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount != 1 {
		return
	}

	perm.AddCounter("+1/+1", 1)

	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	if len(s.Library) == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          seat,
			"loss_seat":     lossSeat,
			"counter_added": true,
			"impulse":       false,
			"reason":        "library_empty",
		})
		return
	}
	card := s.Library[0]
	gameengine.MoveCard(gs, card, seat, "library", "exile", "ob_nixilis_captive_impulse")
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	gs.ZoneCastGrants[card] = &gameengine.ZoneCastPermission{
		Zone:              "exile",
		Keyword:           "ob_nixilis_captive_impulse",
		ManaCost:          -1,
		RequireController: seat,
		SourceName:        perm.Card.DisplayName(),
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "exile_from_library",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"card":   card.DisplayName(),
			"reason": "ob_nixilis_captive_impulse",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"loss_seat":     lossSeat,
		"counter_added": true,
		"impulse":       true,
		"exiled_card":   card.DisplayName(),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"play_window_until_next_end_step_not_time_bounded")
}
