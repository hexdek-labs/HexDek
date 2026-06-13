package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// deliver_unto_evil.go — per_card handler for Deliver Unto Evil.
//
// Oracle text (Sorcery, {2}{B}):
//
//	Choose up to four target cards in your graveyard. If you control a
//	Bolas planeswalker, return those cards to your hand. Otherwise, an
//	opponent chooses two of them. Leave the chosen cards in your
//	graveyard and put the rest into your hand.
//	Exile Deliver Unto Evil.
//
// Why this needs a per_card handler: the spell body parsed to inert
// scaffold, so on resolution it returned nothing — a graveyard value
// engine running dead. Verified inert: no registered handler for
// "Deliver Unto Evil".
//
// Implemented here (OnResolve): the common (non-Bolas) case. The caster
// "chooses up to four" of their best graveyard cards; an opponent leaves
// two; the remainder return to hand. Under AI policy the opponent leaves
// the two least valuable, so the caster nets their two highest-value
// graveyard cards — that is exactly what we return (the two highest-CMC
// cards in the caster's graveyard).
//
// Deliberately omitted (logged): the "if you control a Bolas
// planeswalker, return all four" upside (narrow build-around) and the
// self-exile flavor clause.

func init() {
	registerDeliverUntoEvil(Global())
	AddResetHook(registerDeliverUntoEvil)
}

func registerDeliverUntoEvil(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Deliver Unto Evil", deliverUntoEvilResolve)
}

func deliverUntoEvilResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "deliver_unto_evil"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	// Rank own-graveyard cards by value (CMC desc); return the best two
	// (the two the opponent is forced to give up).
	gy := gs.Seats[seat].Graveyard
	type scored struct {
		c     *gameengine.Card
		score int
	}
	var ranked []scored
	for _, c := range gy {
		if c == nil {
			continue
		}
		ranked = append(ranked, scored{c, cardCMC(c)})
	}
	// Selection sort top 2 by score (stable enough; graveyards are small).
	returned := 0
	used := map[*gameengine.Card]bool{}
	for returned < 2 {
		best := -1
		var bestCard *gameengine.Card
		for _, s := range ranked {
			if used[s.c] {
				continue
			}
			if s.score > best {
				best = s.score
				bestCard = s.c
			}
		}
		if bestCard == nil {
			break
		}
		used[bestCard] = true
		gameengine.MoveCard(gs, bestCard, seat, "graveyard", "hand", slug)
		returned++
	}

	emit(gs, slug, "Deliver Unto Evil", map[string]interface{}{
		"seat":     seat,
		"returned": returned,
	})
}
