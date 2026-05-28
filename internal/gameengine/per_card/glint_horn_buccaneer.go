package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGlintHornBuccaneer wires Glint-Horn Buccaneer.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Glint-Horn%20Buccaneer):
//
//	{2}{R}
//	Creature — Minotaur Pirate
//	2/2
//	Trample
//	Whenever you discard a card, Glint-Horn Buccaneer deals 1 damage
//	to each opponent and you may draw a card.
//
// The discard-→-pings + loot engine is the centerpiece. Pairs with
// rummaging (Wild Mongrel, Anje Falkenrath) and madness shells to
// turn every discard into a free ping + cycle. In a 4-player pod
// each discard deals 3 total damage across the table; chained
// with looting it can close a game.
//
// Implementation:
//   - OnTrigger("card_discarded") gated on discarder_seat == self
//     controller. The Hat-policy "you may draw" defaults to yes —
//     drawing is essentially always upside (loot makes the next
//     discard cheaper; declining surrenders both halves of the
//     engine).
//   - Deal 1 to each living opponent via gameengine.DealDamage.
//   - Draw one card (move library top to hand); skip if library empty
//     and set AttemptedEmptyDraw so the SBA harness handles
//     loss-on-draw.
func registerGlintHornBuccaneer(r *Registry) {
	r.OnTrigger("Glint-Horn Buccaneer", "card_discarded", glintHornBuccaneerDiscard)
}

func glintHornBuccaneerDiscard(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "glint_horn_buccaneer_discard_ping"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat != perm.Controller {
		return
	}

	// Deal 1 to each living opponent.
	hit := 0
	for _, opp := range gs.Opponents(perm.Controller) {
		if opp < 0 || opp >= len(gs.Seats) {
			continue
		}
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.DealDamage(gs, opp, 1, perm.Card.DisplayName())
		gs.LogEvent(gameengine.Event{
			Kind:   "damage",
			Seat:   perm.Controller,
			Target: opp,
			Source: perm.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"reason": "glint_horn_buccaneer",
			},
		})
		hit++
	}

	// Draw one (Hat policy: always opt yes — discard engine demands fuel).
	seat := gs.Seats[perm.Controller]
	drew := false
	if seat != nil && !seat.Lost {
		if len(seat.Library) > 0 {
			c := seat.Library[0]
			gameengine.MoveCard(gs, c, perm.Controller, "library", "hand", slug)
			drew = true
			gs.LogEvent(gameengine.Event{
				Kind:   "draw",
				Seat:   perm.Controller,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"reason": slug,
				},
			})
		} else {
			seat.AttemptedEmptyDraw = true
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"opponents_hit":  hit,
		"drew":           drew,
	})
}
