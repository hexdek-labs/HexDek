package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUrabraskHereticPraetor wires Urabrask, Heretic Praetor.
//
// Oracle text (Scryfall, verified):
//
//	Haste
//	At the beginning of your upkeep, exile the top card of your
//	library. You may play it this turn.
//	At the beginning of each opponent's upkeep, the next time they
//	would draw a card this turn, instead they exile the top card of
//	their library. They may play it this turn.
//
// Implementation (R45 stub port):
//   - Haste: AST keyword pipeline.
//   - Own-upkeep impulse: OnTrigger("upkeep") gated on active_seat ==
//     controller. Exile the top card of the controller's library and
//     register a ZoneCastPermission{Zone: exile, Duration:
//     until_end_of_turn} so the cast pipeline can play it this turn.
//     Mirrors the Jeska's-Will and Prosper impulse patterns.
//   - Opponent-upkeep draw replacement: emitPartial. The
//     replacement effect surface (the next time they would draw, they
//     exile instead) is engine territory — the per_card layer can't
//     intercept the engine's drawOne mid-call. Documented gap.
func registerUrabraskHereticPraetor(r *Registry) {
	r.OnETB("Urabrask, Heretic Praetor", urabraskHereticPraetorETB)
	r.OnTrigger("Urabrask, Heretic Praetor", "upkeep", urabraskUpkeepImpulse)
}

func urabraskHereticPraetorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "urabrask_heretic_praetor_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"opponent_upkeep_draw_to_exile_replacement_not_wired_at_per_card_layer")
}

func urabraskUpkeepImpulse(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "urabrask_upkeep_exile_impulse"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		// Opponent's upkeep — the printed effect is a draw-replacement
		// on that opponent, which we can't drive from the per_card
		// layer. Breadcrumb only.
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"opponent_upkeep_draw_replacement_not_modeled")
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil || s.Lost || len(s.Library) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "empty_library_or_no_seat", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	top := s.Library[0]
	if top == nil {
		return
	}
	gameengine.MoveCard(gs, top, perm.Controller, "library", "exile", "urabrask_upkeep_impulse")

	gameengine.RegisterZoneCastGrant(gs, top, &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneExile,
		Keyword:           "urabrask_impulse_play",
		ManaCost:          -1, // pay normal mana cost
		RequireController: perm.Controller,
		SourceName:        "Urabrask, Heretic Praetor",
		Duration:          "until_end_of_turn",
		GrantTurn:         gs.Turn,
	})
	// End-of-turn cleanup: drop the grant if the card is still in
	// exile (the cast pipeline's per-turn cleanup also expires this
	// duration; this delayed trigger is a belt-and-suspenders for
	// callers that bypass the cleanup).
	captured := top
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.RemoveZoneCastGrant(gs, captured)
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"exiled": top.DisplayName(),
	})
}
