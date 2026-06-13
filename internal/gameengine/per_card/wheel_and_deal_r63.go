package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// wheel_and_deal_r63.go — per_card handler for Wheel and Deal.
//
// Oracle text (Scryfall):
//
//	Any number of target opponents each discard their hands, then draw
//	seven cards.
//	Draw a card.
//
// {3}{U} Instant. An ASYMMETRIC wheel: the caster does NOT refill their
// own hand to seven — the chosen opponents wheel to 7 while the caster
// merely cantrips (+1). Played as a "group hug" tempo/politics tool or to
// refill an ally. The targeted, asymmetric "opponents wheel, you draw
// one" shape has no structured AST node, so the generic dispatch logs it
// inert: no one wheels and the caster doesn't even cantrip (DOA).
//
// Implementation (OnResolve; the per_card resolve hook is the
// authoritative spell body):
//   - "Any number of target opponents": with no negotiation context a
//     per_card handler takes the maximal printed line — ALL living
//     opponents are targeted (the common refuel-the-table / dig-for-
//     answers line). Each discards their whole hand, then draws seven.
//   - "Draw a card": the CASTER draws exactly one (NOT seven) — the
//     detail the inert parse dropped.
func init() {
	registerWheelAndDealR63(Global())
	AddResetHook(registerWheelAndDealR63)
}

func registerWheelAndDealR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Wheel and Deal", wheelAndDealResolve)
}

func wheelAndDealResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "wheel_and_deal"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Each targeted opponent discards their hand, then draws seven.
	wheeled := 0
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		for len(gs.Seats[opp].Hand) > 0 {
			gameengine.DiscardCard(gs, gs.Seats[opp].Hand[0], opp)
		}
		for j := 0; j < 7 && len(gs.Seats[opp].Library) > 0; j++ {
			top := gs.Seats[opp].Library[0]
			gameengine.MoveCard(gs, top, opp, "library", "hand", "draw")
		}
		wheeled++
	}

	// "Draw a card." — the caster cantrips for exactly one.
	if s := gs.Seats[seat]; s != nil && !s.Lost && len(s.Library) > 0 {
		top := s.Library[0]
		gameengine.MoveCard(gs, top, seat, "library", "hand", "draw")
	}

	emit(gs, slug, "Wheel and Deal", map[string]interface{}{
		"seat":              seat,
		"opponents_wheeled": wheeled,
	})
}
