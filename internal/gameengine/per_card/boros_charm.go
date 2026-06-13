package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// boros_charm.go — per_card handler for Boros Charm.
//
// Oracle text (Instant, {R}{W}):
//
//	Choose one —
//	• Boros Charm deals 4 damage to target player or planeswalker.
//	• Creatures you control gain indestructible until end of turn.
//	• Target permanent you control gains double strike until end of turn.
//
// Why a per_card handler: the modal body parsed to an inert `parsed_tail`
// scaffold (the engine's modal resolver never structured the bullets), so the
// whole spell did NOTHING. Boros Charm is a heavily-played burn finisher
// (76 decks in the local pool). Verified inert: no registered handler.
//
// Mode policy: the AI resolves the highest-impact generically-useful mode —
// 4 damage to the face of the lowest-life living opponent (Boros Charm's
// dominant cEDH use as a reach/finisher). The indestructible-team and
// double-strike modes are situational (respond-to-wrath / combat trick) and
// are intentionally not chosen here; logged as not-modeled so this stays a
// strict improvement over the prior do-nothing behavior rather than guessing
// a worse line.
func init() {
	registerBorosCharm(Global())
	AddResetHook(registerBorosCharm)
}

func registerBorosCharm(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Boros Charm", borosCharmResolve)
}

func borosCharmResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "boros_charm"
	if gs == nil || item == nil {
		return
	}
	target := borosLowestLifeOpponent(gs, item.Controller)
	if target < 0 {
		emitFail(gs, slug, "Boros Charm", "no_opponent_target", nil)
		return
	}
	emitPartial(gs, slug, "Boros Charm", "indestructible_and_double_strike_modes_not_modeled_ai_picks_burn")
	gameengine.DealDamage(gs, target, 4, "Boros Charm")
	emit(gs, slug, "Boros Charm", map[string]interface{}{
		"seat":   item.Controller,
		"target": target,
		"damage": 4,
		"mode":   "burn_face",
	})
	_ = gs.CheckEnd()
}

// borosLowestLifeOpponent returns the seat index of the lowest-life living opponent
// of `controller`, or -1 if none.
func borosLowestLifeOpponent(gs *gameengine.GameState, controller int) int {
	best := -1
	for _, opp := range gs.Opponents(controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if best < 0 || s.Life < gs.Seats[best].Life {
			best = opp
		}
	}
	return best
}
