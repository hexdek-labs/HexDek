package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// return_of_the_wildspeaker.go — per_card handler for Return of the Wildspeaker.
//
// Oracle text (Instant, {4}{G}):
//
//	Choose one —
//	• Draw cards equal to the greatest power among non-Human creatures you control.
//	• Non-Human creatures you control get +3/+3 until end of turn.
//
// Why a per_card handler: the modal body parsed to an inert `parsed_tail`
// scaffold, so this green card-draw staple (65 decks) did NOTHING. Verified
// inert: no registered handler.
//
// Mode policy (AI): the draw mode is the dominant use (a big refuel) and is
// generically stronger than a temporary pump, so the handler draws cards equal
// to the greatest power among the controller's non-Human creatures (via the
// shared DrawN primitive). The +3/+3 combat-trick mode is logged not-modeled.
func init() {
	registerReturnOfTheWildspeaker(Global())
	AddResetHook(registerReturnOfTheWildspeaker)
}

func registerReturnOfTheWildspeaker(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Return of the Wildspeaker", returnOfTheWildspeakerResolve)
}

func returnOfTheWildspeakerResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "return_of_the_wildspeaker"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	maxPow := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if creatureIsHuman(p.Card) {
			continue
		}
		if pw := p.Power(); pw > maxPow {
			maxPow = pw
		}
	}
	emitPartial(gs, slug, "Return of the Wildspeaker", "plus3plus3_mode_not_modeled_ai_picks_draw")
	if maxPow <= 0 {
		emitFail(gs, slug, "Return of the Wildspeaker", "no_nonhuman_creature_power", nil)
		return
	}
	drawn := gameengine.DrawN(gs, seat, maxPow, nil)
	emit(gs, slug, "Return of the Wildspeaker", map[string]interface{}{
		"seat": seat, "mode": "draw", "max_power": maxPow, "drawn": drawn,
	})
}

func creatureIsHuman(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if cardHasType(c, "human") {
		return true
	}
	return strings.Contains(strings.ToLower(c.TypeLine), "human")
}
