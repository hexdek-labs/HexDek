package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGaeasWill wires Gaea's Will (R60).
//
// Oracle text:
//
//	Suspend 4—{G}
//	Until end of turn, you may play lands and cast spells from your
//	graveyard. If a card would be put into your graveyard from
//	anywhere this turn, exile that card instead.
//
// Modern Horizons 2's green echo of Yawgmoth's Will. Identical body
// to Will once it resolves; the suspend rider is the AST-driven
// alt-cost path (keywords_suspend.go) and doesn't need special
// handling here. The card lands on the stack as a sorcery whose
// effect mirrors Yawgmoth's Will.
//
// Implementation: identical to Yawgmoth's Will — OnResolve delegates
// to gameengine.RegisterPlayFromGraveyard with Permanent=false.
func registerGaeasWill(r *Registry) {
	r.OnResolve("Gaea's Will", gaeasWillResolve)
}

func gaeasWillResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "gaeas_will"
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
	emitPartial(gs, slug, item.Card.DisplayName(),
		"land_play_from_graveyard consumer not yet wired in tryPlayLand")
}
