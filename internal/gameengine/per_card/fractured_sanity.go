package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// fractured_sanity.go — per_card handler for Fractured Sanity.
//
// Oracle text (Sorcery, {U}{U}{U}):
//
//	Each opponent mills fourteen cards.
//	Cycling {1}{U}
//	When you cycle this card, each opponent mills four cards.
//
// Why this needs a per_card handler: the spell body parsed to inert
// scaffold, so on resolution Fractured Sanity milled nobody — a marquee
// mill-deck finisher running dead. Verified inert: no registered handler
// for "Fractured Sanity".
//
// Implemented here (OnResolve): each opponent mills 14, via the canonical
// per_card mill idiom (MoveCard library→graveyard per card, firing
// zone-change triggers). The cycling-trigger "mill 4" mode is a separate
// cast/cycle path, not the resolve body, and is left to the cycling
// pipeline.

func init() {
	registerFracturedSanity(Global())
	AddResetHook(registerFracturedSanity)
}

func registerFracturedSanity(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Fractured Sanity", fracturedSanityResolve)
}

func fracturedSanityResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "fractured_sanity"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	total := 0
	for _, opp := range gs.Opponents(seat) {
		total += millSeatN(gs, opp, 14, seat, slug)
	}
	emit(gs, slug, "Fractured Sanity", map[string]interface{}{
		"seat":         seat,
		"total_milled": total,
	})
	_ = gs.CheckEnd()
}

// millSeatN mills up to n cards from the top of seat's library to its
// graveyard, attributing the mill to controllerSeat for logging. Returns
// the number actually milled (stops early on an empty library). Routed
// through MoveCard so zone-change triggers fire.
func millSeatN(gs *gameengine.GameState, seat, n, controllerSeat int, slug string) int {
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return 0
	}
	milled := 0
	for i := 0; i < n; i++ {
		if len(gs.Seats[seat].Library) == 0 {
			break
		}
		top := gs.Seats[seat].Library[0]
		gameengine.MoveCard(gs, top, seat, "library", "graveyard", slug)
		milled++
		gs.LogEvent(gameengine.Event{
			Kind:    "mill",
			Seat:    controllerSeat,
			Target:  seat,
			Source:  slug,
			Amount:  1,
			Details: map[string]interface{}{"card": top.DisplayName()},
		})
	}
	return milled
}
