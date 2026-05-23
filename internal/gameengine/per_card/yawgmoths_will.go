package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYawgmothsWill wires Yawgmoth's Will (R60).
//
// Oracle text:
//
//	Until end of turn, you may play lands and cast spells from your
//	graveyard. If a card would be put into your graveyard from
//	anywhere this turn, exile that card instead.
//
// The canonical cEDH "second main phase win condition": after milling
// /discarding /killing a pile of fast mana and finishers, Will lets
// the controller replay it all for one turn while every replayed
// card exiles itself on resolve. Combined with Lion's Eye Diamond,
// Lotus Petal, and a recursive draw spell (Wheel / Brainstorm /
// Frantic Search) it generates lethal storm counts in one turn.
//
// Implementation: OnResolve hook calls into the R60
// gameengine.RegisterPlayFromGraveyard primitive with
// Permanent=false, SourcePerm=nil. The primitive handles all three
// surfaces (per-Card ZoneCastGrants for the current graveyard,
// ZoneCastPolicy for late arrivals, §614 graveyard→exile
// replacement) and the seat flag for the land-play half. The
// turn-scoped pieces are swept at EndOfTurnCleanup via
// gameengine.ExpirePlayFromGraveyardForTurn.
//
// The sorcery itself goes to its owner's graveyard on resolve per
// CR §608.2g; the §614 replacement we just installed redirects that
// to exile (Will exiles itself, matching the printed behavior).
func registerYawgmothsWill(r *Registry) {
	r.OnResolve("Yawgmoth's Will", yawgmothsWillResolve)
}

func yawgmothsWillResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "yawgmoths_will"
	if gs == nil || item == nil {
		return
	}
	seatIdx := item.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	granted := gameengine.RegisterPlayFromGraveyard(gs, gameengine.PlayFromGraveyardOptions{
		SeatIdx:    seatIdx,
		SourceName: item.Card.DisplayName(),
		SourcePerm: nil,
		Permanent:  false,
	})
	emit(gs, slug, item.Card.DisplayName(), map[string]interface{}{
		"seat":            seatIdx,
		"per_card_grants": granted,
		"duration":        "until_end_of_turn",
	})
	// Land-play consumer integration is still pending in the engine
	// (tryPlayLand only scans hand today). Emit a partial-residual
	// breadcrumb so Muninn tracks the remaining surface; the cast
	// half is fully integrated via CastFromZone.
	emitPartial(gs, slug, item.Card.DisplayName(),
		"land_play_from_graveyard consumer not yet wired in tryPlayLand")
}
