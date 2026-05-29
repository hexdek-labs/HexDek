package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWeddingRing wires Wedding Ring (Muninn parser-gap #22, 58,767 hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{2}{W}{W}
//	Artifact
//	When this artifact enters, if it was cast, target opponent creates a
//	token that's a copy of it.
//	Whenever an opponent who controls an artifact named Wedding Ring
//	draws a card during their turn, you draw a card.
//	Whenever an opponent who controls an artifact named Wedding Ring
//	gains life during their turn, you gain that much life.
//
// Implementation:
//   - OnETB: if perm.Flags["was_cast"] is set, hand the lowest-life
//     opponent a token copy. Picking the "weakest" opponent maximizes
//     the chance the gifted Ring survives long enough to mirror their
//     draws/lifegain back to us.
//   - card_drawn / life_gained: every Wedding Ring on the battlefield
//     evaluates the trigger. The trigger condition is "opponent who
//     controls another Wedding Ring" — so when an opponent draws during
//     their turn, any Wedding Ring controller whose opponent also has
//     one fires.
//   - "during their turn" gates on gs.Active == drawer/gainer.
func registerWeddingRing(r *Registry) {
	r.OnETB("Wedding Ring", weddingRingETB)
	r.OnTrigger("Wedding Ring", "card_drawn", weddingRingCardDrawn)
	r.OnTrigger("Wedding Ring", "life_gained", weddingRingLifeGained)
}

func weddingRingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "wedding_ring_etb_gift"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil || perm.Flags["was_cast"] == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"gifted": false,
			"reason": "not_cast",
		})
		return
	}
	target := -1
	bestLife := 1 << 30
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target < 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"gifted": false,
			"reason": "no_opponent",
		})
		return
	}
	ts := gs.Seats[target]
	// InstanceID Phase 5: token-as-copy chokepoint.
	card := gameengine.MintTokenAsCopyOf(gs, perm.Card, target, "")
	if card == nil {
		return
	}
	tokenPerm := &gameengine.Permanent{
		Card:       card,
		Controller: target,
		Owner:      target,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	ts.Battlefield = append(ts.Battlefield, tokenPerm)
	gameengine.RegisterReplacementsForPermanent(gs, tokenPerm)
	gameengine.FirePermanentETBTriggers(gs, tokenPerm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target,
		"gifted": true,
	})
}

// weddingRingHasOpponentRing returns true if oppSeat controls a
// Wedding Ring permanent on the battlefield.
func weddingRingHasOpponentRing(gs *gameengine.GameState, oppSeat int) bool {
	if oppSeat < 0 || oppSeat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[oppSeat]
	if s == nil {
		return false
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == "Wedding Ring" {
			return true
		}
	}
	return false
}

func weddingRingCardDrawn(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "wedding_ring_mirror_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	drawer, ok := ctx["drawer_seat"].(int)
	if !ok {
		return
	}
	if drawer == perm.Controller {
		return
	}
	if drawer != gs.Active {
		return
	}
	if !weddingRingHasOpponentRing(gs, drawer) {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"drawer": drawer,
	})
}

func weddingRingLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "wedding_ring_mirror_life"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainer, _ := ctx["seat"].(int)
	if gainer == perm.Controller {
		return
	}
	if gainer != gs.Active {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	if !weddingRingHasOpponentRing(gs, gainer) {
		return
	}
	gameengine.GainLife(gs, perm.Controller, amount, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"gainer": gainer,
		"amount": amount,
	})
}
