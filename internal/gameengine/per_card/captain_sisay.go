package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCaptainSisay wires Captain Sisay.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Captain%20Sisay):
//
//	{T}: Search your library for a legendary card, reveal that card,
//	put it into your hand, then shuffle.
//
// {2}{G}{W} Legendary Creature — Human Soldier 2/2. The canonical
// legendary tutor — chains into commanders, legendary artifacts
// (Sword of the Animist, Skullclamp variants), legendary lands
// (Kor Haven, Eiganjo Castle), and legendary planeswalkers. Combo
// fuel in Selesnya / Bant legendary tribal shells.
//
// Implementation reuses tutorToHand from tutors.go with a
// "legendary" type-tag filter. Tap is enforced via the standard
// activated-ability tap check.
func registerCaptainSisay(r *Registry) {
	r.OnActivated("Captain Sisay", captainSisayActivate)
}

func captainSisayActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "captain_sisay_tutor"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Captain Sisay", "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	src.Tapped = true

	found := tutorToHand(gs, seat, func(c *gameengine.Card) bool {
		return c != nil && cardHasType(c, "legendary")
	}, "Captain Sisay")

	emit(gs, slug, "Captain Sisay", map[string]interface{}{
		"seat":  seat,
		"found": found,
	})
}
