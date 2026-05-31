package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGolosTirelessPilgrim wires Golos, Tireless Pilgrim.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	When Golos enters, you may search your library for a land card, put
//	that card onto the battlefield tapped, then shuffle.
//	{2}{W}{U}{B}{R}{G}: Exile the top three cards of your library. You
//	may play them this turn without paying their mana costs.
//
// Implementation:
//   - ETB: search library for the most-color-fixing land available
//     (prefer non-basic duals/shocks/triomes by color count, then basics
//     of the most-needed color), put onto battlefield tapped, shuffle.
//   - Activated 5-color exile-and-cast (R60 batch 8): exile the top
//     three cards of controller's library, then register a
//     free-cast permission per card via NewFreeCastFromExilePermission
//     (ManaCost=0 — "without paying their mana costs"), with
//     Duration=until_end_of_turn + SourceTimestamp tied to Golos so
//     ExpireZoneCastGrants / ExpireSourceGrants reap them at cleanup
//     or on Golos LTB. The {2}{W}{U}{B}{R}{G} mana cost is enforced
//     upstream by the activation cost pipeline.
func registerGolosTirelessPilgrim(r *Registry) {
	r.OnETB("Golos, Tireless Pilgrim", golosTirelessPilgrimETB)
	r.OnActivated("Golos, Tireless Pilgrim", golosTirelessPilgrimActivate)
}

func golosTirelessPilgrimETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "golos_tireless_pilgrim_land_tutor"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Pick best land: highest color-source count wins; basic last.
	bestIdx := -1
	bestScore := -1
	for i, c := range s.Library {
		if c == nil || !cardHasType(c, "land") {
			continue
		}
		score := len(c.Colors) * 10
		if !cardHasType(c, "basic") {
			score += 5
		}
		if score == 0 {
			score = 1
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, perm.Card.DisplayName(), "no_land_in_library", map[string]interface{}{"seat": seat})
		return
	}
	land := s.Library[bestIdx]
	s.Library = append(s.Library[:bestIdx], s.Library[bestIdx+1:]...)
	shuffleLibraryPerCard(gs, seat)
	enterBattlefieldWithETB(gs, seat, land, true)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"tutored":  land.DisplayName(),
		"entered":  "tapped",
	})
}

func golosTirelessPilgrimActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "golos_tireless_pilgrim_activated"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_seat", nil)
		return
	}
	exiled := []string{}
	for i := 0; i < 3 && len(seat.Library) > 0; i++ {
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		gameengine.MoveCard(gs, top, seatIdx, "library", "exile", "golos_exile")
		grant := gameengine.NewFreeCastFromExilePermission(seatIdx, src.Card.DisplayName())
		grant.ManaCost = 0
		grant.Duration = "until_end_of_turn"
		grant.GrantTurn = gs.Turn
		grant.SourceTimestamp = src.Timestamp
		gameengine.RegisterZoneCastGrant(gs, top, grant)
		exiled = append(exiled, top.DisplayName())
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seatIdx,
		"exiled_count": len(exiled),
		"exiled":       exiled,
	})
}
