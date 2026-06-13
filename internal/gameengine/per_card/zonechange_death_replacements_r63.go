package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// zonechange_death_replacements_r63.go — CR 614.6 "if a creature an
// opponent controls would die, instead exile it (+ rider)" continuous
// replacement effects. These were INERT: the AST emits a bare
// `replacement_static` scaffold node that resolveModificationEffect only
// logs, and the engine's canonical RegisterReplacementsForPermanent
// switch only wires ~15 hard-coded names (Rest in Peace, Leyline of the
// Void, Anafenza, Dauthi Voidwalker, …). Everything else in the
// would-die / graveyard-hate family did nothing.
//
// This file self-registers (init → AddResetHook) per the per_card
// convention and wires each card's replacement through the SAME engine
// framework the canonical handlers use (gameengine.RegisterReplacement
// with a "would_die" ReplacementEffect). Cleanup is automatic: the
// generic gs.UnregisterReplacementsForPermanent(p) runs on every
// battlefield exit (zone_change.go destroy/exile/sacrifice/bounce + the
// SBA paths), removing every replacement whose SourcePerm == the leaving
// permanent.
//
// CR 614.6 correctness: each Applies predicate gates on
// to_zone == "graveyard", so once ANY replacement (e.g. a co-present
// Rest in Peace) has already redirected the death to exile, these no
// longer fire — exactly one redirect (and one rider) resolves, the
// affected player choosing order. Applied-once is the framework's
// AppliedIDs[HandlerID] guard.

func init() {
	registerZoneChangeDeathReplacements(Global())
	AddResetHook(registerZoneChangeDeathReplacements)
}

func registerZoneChangeDeathReplacements(r *Registry) {
	r.OnETB("Kalitas, Traitor of Ghet", kalitasETBRegisterReplacement)
	r.OnETB("Liesa, Forgotten Archangel", liesaETBRegisterReplacement)
	r.OnETB("Ravenous Slime", ravenousSlimeETBRegisterReplacement)
}

// oppNontokenCreatureWouldDie returns an Applies predicate: the dying
// object is an opponent's creature still heading to the graveyard. When
// nontoken is true the predicate also excludes tokens.
func oppNontokenCreatureWouldDie(p *gameengine.Permanent, nontoken bool) func(*gameengine.GameState, *gameengine.ReplEvent) bool {
	return func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
		if ev == nil || ev.TargetPerm == nil {
			return false
		}
		// Only a death still routed to the graveyard — once another
		// replacement has redirected to exile this no longer applies
		// (CR 614.6: a replacement applies to an event at most once,
		// and the event is no longer "would be put into a graveyard").
		if ev.String("to_zone") != "graveyard" {
			return false
		}
		tp := ev.TargetPerm
		if tp.Controller == p.Controller {
			return false
		}
		if !tp.IsCreature() {
			return false
		}
		if nontoken && tp.IsToken() {
			return false
		}
		return true
	}
}

func logRedirect(gs *gameengine.GameState, p *gameengine.Permanent, effect string) {
	gs.LogEvent(gameengine.Event{
		Kind:   "replacement_applied",
		Seat:   p.Controller,
		Source: p.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "614.6",
			"effect": effect,
		},
	})
}

// ---------------------------------------------------------------------
// Kalitas, Traitor of Ghet — "If a nontoken creature an opponent
// controls would die, instead exile that card and create a 2/2 black
// Zombie creature token."
// ---------------------------------------------------------------------

func kalitasETBRegisterReplacement(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      fmt.Sprintf("Kalitas, Traitor of Ghet:die:%d", perm.Timestamp),
		RedirectsZone:  true,
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies:        oppNontokenCreatureWouldDie(perm, true),
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.Payload["to_zone"] = "exile"
			if tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Zombie",
				[]string{"creature", "zombie"}, 2, 2); tok != nil && tok.Card != nil {
				tok.Card.Colors = []string{"B"}
			}
			logRedirect(gs, perm, "opp_creature_die_exile_make_zombie")
		},
	})
	emit(gs, "kalitas_traitor_of_ghet_register", perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}

// ---------------------------------------------------------------------
// Liesa, Forgotten Archangel — "If a creature an opponent controls
// would die, exile it instead." (Pure redirect; the "return your dying
// creatures" clause is a separate triggered ability, not this
// replacement.)
// ---------------------------------------------------------------------

func liesaETBRegisterReplacement(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      fmt.Sprintf("Liesa, Forgotten Archangel:die:%d", perm.Timestamp),
		RedirectsZone:  true,
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies:        oppNontokenCreatureWouldDie(perm, false),
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.Payload["to_zone"] = "exile"
			logRedirect(gs, perm, "opp_creature_die_exile")
		},
	})
	emit(gs, "liesa_forgotten_archangel_register", perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}

// ---------------------------------------------------------------------
// Ravenous Slime — "If a creature an opponent controls would die,
// instead exile it and put a number of +1/+1 counters equal to that
// creature's power on this creature."
// ---------------------------------------------------------------------

func ravenousSlimeETBRegisterReplacement(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      fmt.Sprintf("Ravenous Slime:die:%d", perm.Timestamp),
		RedirectsZone:  true,
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies:        oppNontokenCreatureWouldDie(perm, false),
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.Payload["to_zone"] = "exile"
			// Read the dying creature's power while it is still on the
			// battlefield (the replacement resolves before the move).
			if pow := ev.TargetPerm.Power(); pow > 0 {
				gameengine.AddCountersToPermanent(gs, perm, "+1/+1", pow, "", false)
			}
			logRedirect(gs, perm, "opp_creature_die_exile_grow")
		},
	})
	emit(gs, "ravenous_slime_register", perm.Card.DisplayName(),
		map[string]interface{}{"seat": perm.Controller})
}
