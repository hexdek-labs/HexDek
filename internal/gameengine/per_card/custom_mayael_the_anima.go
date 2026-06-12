package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMayaelCustom implements Mayael the Anima's activated ability
// that the auto-generated stub leaves as `emitPartial` only.
//
// Oracle text:
//
//	{3}{R}{G}{W}, {T}: Look at the top five cards of your library. You
//	may put a creature card with power 5 or greater from among them
//	onto the battlefield. Put the rest on the bottom of your library in
//	any order.
//
// Picks the highest-power creature among the top 5 with power ≥ 5; if
// none qualifies, the top 5 just go to the bottom in their original
// order. The cost ({3}{R}{G}{W} + tap) is enforced by the engine before
// dispatch — this handler resolves the effect.
func registerMayaelCustom(r *Registry) {
	r.OnActivated("Mayael the Anima", mayaelLookFive)
}

func mayaelLookFive(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mayael_look_five"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || len(seat.Library) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "library_empty", nil)
		return
	}
	// Cost gates: {3}{R}{G}{W} = 6 generic from the engine's pool, plus
	// {T}. Defensive checks for callers that bypass the engine activation
	// dispatcher ONLY — when ActivateAbility dispatched this (the AST
	// Activated node exists), it has already tapped the source and paid
	// the mana; re-gating here both double-paid and, worse, aborted as
	// "already_tapped" on every legitimate activation, leaving the
	// ability permanently dead through the live path (judge round-5).
	if !dispatcherHandledActivationCosts(src, abilityIdx) {
		if src.Tapped {
			emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
			return
		}
		if !payManaFromPool(seat, 6) {
			emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
				"required":  6,
				"mana_pool": seat.ManaPool,
			})
			return
		}
		src.Tapped = true
	}
	n := 5
	if n > len(seat.Library) {
		n = len(seat.Library)
	}
	top := append([]*gameengine.Card(nil), seat.Library[:n]...)
	var pick *gameengine.Card
	pickIdx := -1
	bestPower := 4 // strictly greater than 4 → ≥5
	for i, c := range top {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if c.BasePower > bestPower {
			pick = c
			pickIdx = i
			bestPower = c.BasePower
		}
	}
	if pick != nil {
		// Wave 2 multi-step migration: enterBattlefieldWithETB →
		// createPermanent sweeps `pick` from library canonically;
		// pre-r60 spliced library[n:] first, which silently no-op'd the
		// chokepoint's source removal. The non-picked top cards are now
		// rotated to the bottom as a within-zone reorder (no zone change).
		_ = pickIdx
		enterBattlefieldWithETB(gs, src.Controller, pick, false)
		for _, c := range top {
			if c == nil || c == pick {
				continue
			}
			if len(seat.Library) == 0 || seat.Library[0] != c {
				continue
			}
			seat.Library = seat.Library[1:]
			seat.Library = append(seat.Library, c)
		}
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":  src.Controller,
			"into_play": pick.DisplayName(),
			"power": bestPower,
		})
		return
	}
	// Nothing qualifies — rotate all five to the bottom as a within-zone
	// reorder (no zone-change triggers for library reordering).
	for _, c := range top {
		if c == nil {
			continue
		}
		if len(seat.Library) == 0 || seat.Library[0] != c {
			continue
		}
		seat.Library = seat.Library[1:]
		seat.Library = append(seat.Library, c)
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"into_play": "",
		"note":      "no_qualifying_creature",
	})
}
