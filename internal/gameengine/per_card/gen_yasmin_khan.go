package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYasminKhan wires Yasmin Khan.
//
// Oracle text:
//
//	{T}: Exile the top card of your library. Until your next end step,
//	you may play it.
//	Doctor's companion (You can have two commanders if the other is
//	the Doctor.)
//
// Implementation:
//   - Activated ability ({T}): exile the top of the controller's
//     library, register a ZoneCastPermission so the cast pipeline
//     accepts it from exile (paying its normal mana cost, NOT free —
//     Yasmin says "you may play it", not "without paying its cost"),
//     and clean up the grant via a delayed trigger at the next end
//     step.
//   - Doctor's companion is a deckbuilding restriction; nothing to do
//     at runtime.
//   - Mirrors the Urza, Lord High Artificer pattern in
//     urza_lord_high_artificer.go for the exile-then-cast-grant
//     plumbing.
func registerYasminKhan(r *Registry) {
	r.OnActivated("Yasmin Khan", yasminKhanActivate)
}

func yasminKhanActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "yasmin_khan_exile_top_may_play"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	if len(seat.Library) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "library_empty", nil)
		return
	}
	src.Tapped = true

	c := seat.Library[0]
	if c == nil {
		seat.Library = seat.Library[1:]
		emitFail(gs, slug, src.Card.DisplayName(), "top_card_nil", nil)
		return
	}
	gameengine.MoveCard(gs, c, src.Controller, "library", "exile", "yasmin_khan_exile")

	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	// Oracle: "Until your next end step, you may play it." Yasmin is
	// activated at sorcery speed ({T} on her own turn), so the "next end
	// step" is the current turn's end step — i.e. EOT semantics.
	// Use the canonical "until_end_of_turn" Duration value so the
	// engine's ExpireZoneCastGrants + ZoneCastGrantExpiry invariant
	// recognise the lifetime; the previous "until_next_end_step" value
	// was not in either switch and the cleanup ran only via the
	// delayed-trigger fallback below.
	gs.ZoneCastGrants[c] = &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneExile,
		Keyword:           "yasmin_khan_may_play",
		ManaCost:          -1, // sentinel: pay the card's normal cost
		ExileOnResolve:    false,
		RequireController: src.Controller,
		SourceName:        "Yasmin Khan",
		Duration:          "until_end_of_turn",
		GrantTurn:         gs.Turn,
	}

	cardRef := c
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: src.Controller,
		SourceCardName: "Yasmin Khan (cast-grant cleanup)",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if gs.ZoneCastGrants == nil {
				return
			}
			delete(gs.ZoneCastGrants, cardRef)
		},
	})

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            src.Controller,
		"exiled_card":     c.DisplayName(),
		"may_play_window": "until_next_end_step",
	})
}
