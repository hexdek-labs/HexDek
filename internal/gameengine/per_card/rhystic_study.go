package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRhysticStudy wires up Rhystic Study.
//
// Oracle text:
//
//	Whenever an opponent casts a spell, unless that player pays {1},
//	you draw a card.
//
// Policy for "unless they pay {1}":
//   - The opponent pays iff their ManaPool >= 1. This is a greedy
//     "always pay if possible" heuristic. Real-game strategy varies
//     (don't pay if it's the 6th Study tick in a turn, etc.) but MVP
//     keeps things deterministic.
//   - When they pay: decrement their pool, no draw.
//   - When they don't (can't): Study's controller draws a card.
//
// Policy choice rationale: in practice cEDH opponents usually DON'T
// pay — Rhystic becomes a 1-per-opponent-cast card advantage engine.
// We invert the default here to "pay if affordable" because not paying
// would let Rhystic trivially spiral the controller to a win, and the
// broader engine doesn't yet have a "decide whether to pay" hook.
// Future agents can wire a proper decision hook via the Hat API.
func registerRhysticStudy(r *Registry) {
	r.OnTrigger("Rhystic Study", "spell_cast_by_opponent", rhysticStudyOnCast)
}

func rhysticStudyOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rhystic_study"
	if gs == nil || perm == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster == perm.Controller {
		return // "Whenever an OPPONENT casts"
	}
	opp := gs.Seats[caster]
	if opp == nil {
		return
	}
	// Greedy: pay {1} if we can.
	if opp.ManaPool >= 1 {
		opp.ManaPool--
		gs.LogEvent(gameengine.Event{
			Kind:   "pay_mana",
			Seat:   caster,
			Source: perm.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"reason": "rhystic_study_tax",
				"rule":   "603.6",
			},
		})
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"caster_seat": caster,
			"paid_tax":    true,
		})
		return
	}
	// Didn't pay → Study's controller draws.
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"caster_seat": caster,
		"paid_tax":    false,
	})
}

// drawOne pulls the top of seat's library into their hand and logs a
// "draw" event. Sets AttemptedEmptyDraw when the library is empty.
func drawOne(gs *gameengine.GameState, seat int, source string) *gameengine.Card {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		s.AttemptedEmptyDraw = true
		gs.LogEvent(gameengine.Event{
			Kind:   "draw_failed_empty",
			Seat:   seat,
			Source: source,
		})
		return nil
	}
	c := s.Library[0]
	gameengine.MoveCard(gs, c, seat, "library", "hand", "draw")
	gs.LogEvent(gameengine.Event{
		Kind:   "draw",
		Seat:   seat,
		Target: seat,
		Source: source,
		Amount: 1,
	})
	return c
}
