package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// replacement_lifegain_ifwould_r63.go — CR §614 GENERAL "if you would …,
// instead …" replacement effects in the life / draw long-tail that were
// parsed but INERT (no handler registered, so the §614 chain never saw
// them). Each card below registers a replacement on ETB exactly the way
// the canonical Bilbo / Boon Reflection / Rhox Faithmender handlers do —
// via the would_gain_life / would_lose_life / would_draw Fire hooks that
// resolveGainLife / resolveLoseLife / resolveDraw already consult.
//
// Coverage note (matches the existing canonical handlers): the would_*
// chain is consulted on the AST resolve paths (resolveGainLife, etc.).
// Life gained directly through the bare GainLife() helper (e.g. some
// lifelink fast-paths) bypasses the chain for ALL §614 life replacements,
// Bilbo included; widening that is a separate life-system change and is
// out of scope here.
//
// Families covered:
//   A. "you gain that much life plus 1 instead" — additive +1, controller.
//   B. "you gain twice that much life instead"  — ×2, controller
//      (unconditional and life-threshold-gated variants).
//   C. "if an opponent would gain life, that player loses that much life
//      instead" — cancel the gain, deal the loss to the opponent.
//   D. "if you would draw a card, draw two cards instead" — ×2 draw.

func init() {
	registerLifeGainIfWouldR63(Global())
	AddResetHook(registerLifeGainIfWouldR63)
}

func registerLifeGainIfWouldR63(r *Registry) {
	// A. life-gain "plus 1 instead" (additive).
	for _, name := range []string{
		"Angel of Vitality",
		"Heron of Hope",
		"Honor Troll",
		"Knight of Dawn's Light",
		"Cleric Class",
		"Leyline of Hope",
		"Pest Rescuer",
	} {
		n := name
		r.OnETB(n, func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			registerSelfLifeGainDelta(gs, perm, n, 1)
		})
	}

	// B. life-gain "twice that much instead" (×2).
	r.OnETB("The Wind Crystal", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		registerSelfLifeGainMultiplier(gs, perm, "The Wind Crystal", 2, nil)
	})
	// Phial of Galadriel — only "while you have 5 or less life".
	r.OnETB("Phial of Galadriel", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		ctrl := perm.Controller
		registerSelfLifeGainMultiplier(gs, perm, "Phial of Galadriel", 2,
			func(gs *gameengine.GameState) bool {
				return ctrl >= 0 && ctrl < len(gs.Seats) &&
					gs.Seats[ctrl] != nil && gs.Seats[ctrl].Life <= 5
			})
	})

	// C. opponent would gain life → that player loses that much instead.
	for _, name := range []string{"Tainted Remedy", "Plague Drone"} {
		n := name
		r.OnETB(n, func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			registerOpponentGainBecomesLoss(gs, perm, n)
		})
	}

	// D. "if you would draw a card, draw two cards instead."
	r.OnETB("Thought Reflection", func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		registerSelfDrawMultiplier(gs, perm, "Thought Reflection", 2)
	})
}

// registerSelfLifeGainDelta registers a would_gain_life replacement that
// adds `delta` to any positive life the controller would gain.
func registerSelfLifeGainDelta(gs *gameengine.GameState, perm *gameengine.Permanent, name string, delta int) {
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}
	slug := "lifegain_plus_" + strconv.Itoa(delta)
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_gain_life",
		HandlerID:      name + ":lifegain_delta:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			return ev != nil && ev.TargetSeat == controller && ev.Count() > 0
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			before := ev.Count()
			ev.SetCount(before + delta)
			logReplApplied(gs, controller, name, slug, before, before+delta)
		},
	})
	emit(gs, slug, name, map[string]interface{}{"seat": controller, "replaces": "would_gain_life"})
}

// registerSelfLifeGainMultiplier registers a would_gain_life replacement
// that multiplies the controller's positive life gain by `mult`. An
// optional cond gates the replacement (Phial of Galadriel: life <= 5).
func registerSelfLifeGainMultiplier(gs *gameengine.GameState, perm *gameengine.Permanent, name string, mult int, cond func(*gameengine.GameState) bool) {
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}
	slug := "lifegain_times_" + strconv.Itoa(mult)
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_gain_life",
		HandlerID:      name + ":lifegain_mult:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.TargetSeat != controller || ev.Count() <= 0 {
				return false
			}
			return cond == nil || cond(gs)
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			before := ev.Count()
			ev.SetCount(before * mult)
			logReplApplied(gs, controller, name, slug, before, before*mult)
		},
	})
	emit(gs, slug, name, map[string]interface{}{"seat": controller, "replaces": "would_gain_life"})
}

// registerOpponentGainBecomesLoss registers a would_gain_life replacement
// that cancels an opponent's life gain and makes them lose that much life
// instead (Tainted Remedy, Plague Drone). "Opponent" = any living seat
// other than the source's controller (FFA), mirroring Notion Thief.
func registerOpponentGainBecomesLoss(gs *gameengine.GameState, perm *gameengine.Permanent, name string) {
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_gain_life",
		HandlerID:      name + ":gain_to_loss:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.Count() <= 0 {
				return false
			}
			s := ev.TargetSeat
			if s == controller || s < 0 || s >= len(gs.Seats) {
				return false
			}
			return gs.Seats[s] != nil && !gs.Seats[s].Lost
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			amt := ev.Count()
			victim := ev.TargetSeat
			ev.Cancelled = true // the gain does not happen (CR §614.5)
			gameengine.LoseLife(gs, victim, amt, name)
			logReplApplied(gs, victim, name, "opponent_gain_to_loss", amt, -amt)
		},
	})
	emit(gs, "opponent_gain_to_loss", name, map[string]interface{}{"seat": controller, "replaces": "would_gain_life"})
}

// registerSelfDrawMultiplier registers a would_draw replacement that
// multiplies the controller's per-draw count by `mult` ("draw two cards
// instead"). resolveDraw fires would_draw once per card (count 1).
func registerSelfDrawMultiplier(gs *gameengine.GameState, perm *gameengine.Permanent, name string, mult int) {
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}
	slug := "draw_times_" + strconv.Itoa(mult)
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_draw",
		HandlerID:      name + ":draw_mult:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			return ev != nil && ev.TargetSeat == controller && ev.Count() > 0
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			before := ev.Count()
			ev.SetCount(before * mult)
			logReplApplied(gs, controller, name, slug, before, before*mult)
		},
	})
	emit(gs, slug, name, map[string]interface{}{"seat": controller, "replaces": "would_draw"})
}

// logReplApplied emits a uniform replacement_applied event (CR §614).
func logReplApplied(gs *gameengine.GameState, seat int, name, slug string, before, after int) {
	gs.LogEvent(gameengine.Event{
		Kind:   "replacement_applied",
		Seat:   seat,
		Source: name,
		Amount: after,
		Details: map[string]interface{}{
			"slug":   slug,
			"rule":   "614",
			"before": before,
			"after":  after,
		},
	})
}
