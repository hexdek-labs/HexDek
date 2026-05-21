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
	// R52 batch K: register a per-opponent would_draw replacement at
	// each opponent's upkeep that fires on the next draw THAT TURN.
	// The replacement cancels the draw, exiles the top of that
	// opponent's library instead, and grants them a ZoneCastPermission
	// to play the exiled card until end of their turn.
	r.OnTrigger("Urabrask, Heretic Praetor", "upkeep", urabraskRegisterOppDrawReplacement)
}

func urabraskHereticPraetorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "urabrask_heretic_praetor_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func urabraskUpkeepImpulse(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "urabrask_upkeep_exile_impulse"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		// Opponent's upkeep handled by urabraskRegisterOppDrawReplacement.
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

// urabraskRegisterOppDrawReplacement fires on every upkeep event. When
// the active seat is OPPONENT-of-Urabrask, we register a one-shot
// would_draw replacement for that seat that cancels the next draw,
// exiles the top of their library, and grants them a play-this-turn
// ZoneCastPermission (R52 batch K port).
func urabraskRegisterOppDrawReplacement(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "urabrask_register_opp_draw_replacement"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat == perm.Controller {
		return // own upkeep handled by urabraskUpkeepImpulse
	}
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	armTurn := gs.Turn
	fired := false
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_draw",
		HandlerID:      "Urabrask, Heretic Praetor:opp_draw_replace:" + perm.Card.DisplayName(),
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if fired {
				return false
			}
			if gs.Turn != armTurn {
				return false
			}
			if ev == nil || ev.TargetSeat != activeSeat {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			fired = true
			ev.Cancelled = true
			s := gs.Seats[activeSeat]
			if s == nil || len(s.Library) == 0 {
				return
			}
			top := s.Library[0]
			if top == nil {
				return
			}
			gameengine.MoveCard(gs, top, activeSeat, "library", "exile", "urabrask_opp_draw_replace")
			gameengine.RegisterZoneCastGrant(gs, top, &gameengine.ZoneCastPermission{
				Zone:              gameengine.ZoneExile,
				Keyword:           "urabrask_opp_impulse_play",
				ManaCost:          -1,
				RequireController: activeSeat,
				SourceName:        "Urabrask, Heretic Praetor",
				Duration:          "until_end_of_turn",
				GrantTurn:         gs.Turn,
			})
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   perm.Controller,
				Source: "Urabrask, Heretic Praetor",
				Details: map[string]interface{}{
					"slug":        slug,
					"target_seat": activeSeat,
					"exiled":      top.DisplayName(),
				},
			})
		},
	})
}
