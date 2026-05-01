package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAthreosGodOfPassage wires Athreos, God of Passage.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Indestructible
//	As long as your devotion to white and black is less than seven,
//	Athreos isn't a creature.
//	Whenever another creature you own dies, return it to your hand
//	unless target opponent pays 3 life.
//
// Implementation:
//   - "creature_dies": gate on card.Owner == perm.Controller and the
//     dying card != Athreos's own card. Tokens cease to exist on dying
//     (CR §704.5d) before zone-change triggers, so the engine's
//     creature_dies event for tokens still fires from a non-existent
//     graveyard state — we filter out tokens by IsToken() on the perm.
//   - Target opponent selection: Athreos's controller picks the opponent
//     LEAST likely to pay (lowest life), since the goal is to force the
//     return. If multiple opponents are tied, prefer the one that can't
//     safely pay (life <= 3 → guaranteed return).
//   - Pay decision (target opponent's POV): pay only if (a) the creature
//     is impactful (commander OR CMC >= 4) AND (b) opp.Life - 3 > 5 (so
//     paying doesn't put the opponent into lethal range). Otherwise the
//     opponent declines and the creature returns to Athreos's owner's
//     hand.
//   - Devotion-based "isn't a creature" clause (state-dependent): not
//     enforced at the per-card level. Athreos's creature-vs-non-creature
//     status is a continuous-effect characteristic and is best handled
//     by the AST/effects layer; we emitPartial to flag the gap.
//   - Indestructible: handled by the AST keyword pipeline.
func registerAthreosGodOfPassage(r *Registry) {
	r.OnTrigger("Athreos, God of Passage", "creature_dies", athreosGodOfPassageDies)
	r.OnETB("Athreos, God of Passage", athreosGodOfPassageETB)
}

func athreosGodOfPassageETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	emitPartial(gs, "athreos_god_of_passage_static", perm.Card.DisplayName(),
		"devotion_isnt_a_creature_clause_not_enforced")
}

func athreosGodOfPassageDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "athreos_god_of_passage_return"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	if dyingCard == nil {
		return
	}
	// "another creature you own" — owner == Athreos's controller, and
	// not Athreos herself.
	if dyingCard == perm.Card {
		return
	}
	if dyingCard.Owner != perm.Controller {
		return
	}
	// Tokens cease to exist on dying (CR §704.5d) — there's nothing to
	// return to hand. The dying perm context lets us check.
	if dyingPerm, _ := ctx["perm"].(*gameengine.Permanent); dyingPerm != nil {
		if dyingPerm.IsToken() {
			emitFail(gs, slug, perm.Card.DisplayName(), "token_ceases_to_exist", map[string]interface{}{
				"creature": dyingCard.DisplayName(),
			})
			return
		}
	}

	// Target opponent — pick the lowest-life living opponent. If any
	// opponent is at life <= 3, prefer them (they can't safely pay).
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
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent", map[string]interface{}{
			"creature": dyingCard.DisplayName(),
		})
		return
	}
	targetSeat := gs.Seats[target]

	// Target opponent decides whether to pay. Heuristic: pay only on
	// impactful creatures AND when paying leaves them at safe life.
	cmc := gameengine.ManaCostOf(dyingCard)
	isCommander := gameengine.IsCommanderCard(gs, dyingCard.Owner, dyingCard)
	impactful := isCommander || cmc >= 4
	paySafe := targetSeat.Life-3 > 5
	pay := impactful && paySafe

	if pay {
		targetSeat.Life -= 3
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   target,
			Target: -1,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"amount": -3,
				"cause":  "athreos_passage_pay",
			},
		})
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":         perm.Controller,
			"creature":     dyingCard.DisplayName(),
			"target_opp":   target,
			"target_paid":  true,
			"target_life":  targetSeat.Life,
			"returned":     false,
		})
		return
	}

	// Opponent declines — return the creature to its owner's hand.
	owner := dyingCard.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		emitFail(gs, slug, perm.Card.DisplayName(), "bad_owner", map[string]interface{}{
			"creature": dyingCard.DisplayName(),
		})
		return
	}
	gameengine.MoveCard(gs, dyingCard, owner, "graveyard", "hand", "athreos_passage_return")

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"creature":    dyingCard.DisplayName(),
		"target_opp":  target,
		"target_paid": false,
		"returned":    true,
	})
}
