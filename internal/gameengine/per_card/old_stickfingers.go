package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOldStickfingers wires Old Stickfingers.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Old%20Stickfingers):
//
//	{X}{B}{G}
//	Legendary Creature — Horror
//	4/4
//	When you cast this spell, mill cards until you mill X creature
//	cards, then return all creature cards milled this way to the
//	battlefield under your control.
//
// The poster child for self-mill-reanimator commanders. A {7}{B}{G}
// cast (X=7) mills until 7 creatures hit the graveyard — often
// translating to 12-15 cards milled in a creature-heavy deck — then
// drops all 7 onto the battlefield for free. Combined with persist
// creatures (Eternal Witness, Glen Elendra), self-mill payoffs
// (Splinterfright, Mortalpack-style), and reanimator backup, it's
// one of Commander's most consistent turn-4 board-flips.
//
// Implementation:
//   - OnETB hook (simpler than OnCast and produces the same final
//     state: Stickfingers + N creatures on the battlefield). The cast-
//     trigger / ETB ordering distinction only matters for layer
//     observability (an ETB-doubler on Stickfingers itself would
//     double the milled creatures' ETBs but not Stickfingers' own
//     ETB; we don't model that distinction here).
//   - Read x_paid from perm.Flags. If 0, no mill happens — that's
//     rules-correct (X=0 = mill until you mill 0 creatures = mill
//     nothing).
//   - Mill cards from owner's library top until X creature cards are
//     in the graveyard. Cap the mill at library depth to handle
//     small libraries.
//   - For each creature card milled, enterBattlefieldWithETB so it
//     joins the controller's battlefield with full ETB cascade.
//     Non-creature milled cards stay in the graveyard.
func registerOldStickfingers(r *Registry) {
	r.OnETB("Old Stickfingers", oldStickfingersETB)
}

func oldStickfingersETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "old_stickfingers_self_mill_reanimate"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	x := 0
	if perm.Flags != nil {
		x = perm.Flags["x_paid"]
	}
	if x <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"x":         0,
			"triggered": false,
			"reason":    "x_is_zero_or_unthreaded",
		})
		if perm.Flags == nil || perm.Flags["x_paid"] == 0 {
			emitPartial(gs, slug, perm.Card.DisplayName(),
				"x_paid_not_threaded_into_etb_hook")
		}
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Mill until X creature cards are in graveyard (or library empty).
	creaturesMilled := []*gameengine.Card{}
	totalMilled := 0
	for len(creaturesMilled) < x && len(seat.Library) > 0 {
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		gameengine.MoveCard(gs, top, perm.Controller, "library", "graveyard", slug)
		totalMilled++
		gs.LogEvent(gameengine.Event{
			Kind:   "mill",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"card":   top.DisplayName(),
				"reason": slug,
			},
		})
		if cardHasType(top, "creature") {
			creaturesMilled = append(creaturesMilled, top)
		}
	}

	// Reanimate each creature milled this way to controller's battlefield.
	reanimated := []string{}
	for _, c := range creaturesMilled {
		ent := enterBattlefieldWithETB(gs, perm.Controller, c, false)
		if ent != nil {
			reanimated = append(reanimated, c.DisplayName())
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"x":                x,
		"total_milled":     totalMilled,
		"creatures_milled": len(creaturesMilled),
		"reanimated":       reanimated,
	})
}
