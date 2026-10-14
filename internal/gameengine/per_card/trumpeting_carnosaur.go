package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTrumpetingCarnosaur wires Trumpeting Carnosaur (Muninn parser-gap,
// recurring dino-cascade decks).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{4}{R}{R}
//	Creature — Dinosaur
//	Trample
//	When this creature enters, discover 5.
//	{2}{R}, Discard this card: It deals 3 damage to target creature or
//	planeswalker.
//
// Implementation:
//   - Trample handled by AST keyword pipeline.
//   - OnETB: discover 5 via gameengine.PerformDiscover (CR §701.57a).
//     Discover exiles cards from the top of the library until exiling
//     a nonland card with mana value <= N; that card may be cast for
//     free or put into the hand. The engine helper returns the found
//     card; we route the residual "cast-or-hand" choice through the
//     same free-cast partial path other discover/cascade cards use.
//   - The discard-activation removal mode requires per_card OnActivated
//     wiring; emitPartial — the {2}{R}, Discard mode is rarely played
//     since the card prefers to cascade for value.
func registerTrumpetingCarnosaur(r *Registry) {
	r.OnETB("Trumpeting Carnosaur", trumpetingCarnosaurETB)
}

func trumpetingCarnosaurETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "trumpeting_carnosaur_discover5"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	found := gameengine.PerformDiscover(gs, seat, 5)
	name := "nothing"
	if found != nil {
		name = found.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"found": name,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"discard_activated_3_damage_removal_mode_unwired_pending_on_activated")
}
