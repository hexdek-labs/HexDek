package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGyrudaDoomOfDepthsCustom implements Gyruda's mill-and-cheat
// ETB. The auto-generated gen_*.go stub is a no-op.
//
// Oracle text:
//
//	Companion — Your starting deck contains only cards with even mana
//	values. (Companion deckbuilding constraint — engine territory.)
//	When Gyruda enters, each player mills four cards. Put a creature
//	card with an even mana value from among the milled cards onto the
//	battlefield under your control.
//
// Implementation notes:
//   - Mills 4 cards from each player via MoveCard so the standard
//     library→graveyard pipeline (replacement effects, mill triggers)
//     fires correctly.
//   - Reanimate target picked from any seat's milled cards (oracle
//     allows "from among the milled cards" — that's all four piles
//     combined). Picks the highest-MV even creature for max value.
//   - Companion deckbuilding constraint is engine territory and is
//     emitPartial-flagged at registration time.
func registerGyrudaDoomOfDepthsCustom(r *Registry) {
	r.OnETB("Gyruda, Doom of Depths", gyrudaETB)
	r.OwnsETBTrigger("Gyruda, Doom of Depths")
}

func gyrudaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "gyruda_mill_and_cheat"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Track which cards each player mills this resolution so we can
	// pick a target from the union.
	type milledRef struct {
		card *gameengine.Card
		seat int
	}
	var milled []milledRef
	for i, s := range gs.Seats {
		if s == nil || s.Lost || s.LeftGame {
			continue
		}
		for j := 0; j < 4 && len(s.Library) > 0; j++ {
			top := s.Library[0]
			if top == nil {
				break
			}
			gameengine.MoveCard(gs, top, i, "library", "graveyard", "gyruda_mill")
			milled = append(milled, milledRef{card: top, seat: i})
		}
	}

	// Pick the highest-MV creature with even MV among milled.
	var pickIdx = -1
	bestCMC := -1
	for i, m := range milled {
		if m.card == nil {
			continue
		}
		if !cardHasType(m.card, "creature") {
			continue
		}
		cmc := cardCMC(m.card)
		if cmc%2 != 0 {
			continue
		}
		if cmc > bestCMC {
			bestCMC = cmc
			pickIdx = i
		}
	}
	if pickIdx < 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seatIdx,
			"milled": len(milled),
			"note":   "no_even_creature_milled",
		})
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"companion deckbuilding constraint not enforced at engine level")
		return
	}
	pick := milled[pickIdx]
	// Move from the milled card's owner's graveyard into our control on
	// the battlefield. Remove from the source graveyard first so the
	// dedup-by-Card check inside createPermanent can fire cleanly.
	srcSeat := gs.Seats[pick.seat]
	for j, c := range srcSeat.Graveyard {
		if c == pick.card {
			srcSeat.Graveyard = append(srcSeat.Graveyard[:j], srcSeat.Graveyard[j+1:]...)
			break
		}
	}
	enterBattlefieldWithETB(gs, seatIdx, pick.card, false)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seatIdx,
		"milled":        len(milled),
		"reanimated":    pick.card.DisplayName(),
		"reanimated_mv": bestCMC,
		"from_seat":     pick.seat,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"companion deckbuilding constraint not enforced at engine level")
}
