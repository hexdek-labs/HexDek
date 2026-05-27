package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHullbreacher wires Hullbreacher.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Hullbreacher):
//
//	Flash
//	If an opponent would draw a card except the first one they draw in
//	each of their draw steps, instead you create a Treasure token.
//
// {2}{U} Creature — Merfolk Pirate 3/2. cEDH-banned in some lists, but
// still a premier "tax extra draws" card in eternal Commander pools.
// Behavior shape mirrors Notion Thief (see notion_thief.go + the
// engine's RegisterNotionThiefReplacement in replacement.go) — the
// only difference is the side-effect: Notion Thief redirects the
// draw and you draw instead; Hullbreacher cancels the draw and you
// create a Treasure token.
//
// Implementation registers a `would_draw` replacement at ETB. The
// replacement gates the "first draw per turn for the active player"
// exemption identically to Notion Thief — that single draw is the
// draw-step draw and is exempt per the "except the first one they
// draw in each of their draw steps" clause. Every other opponent
// draw is replaced with a Treasure for Hullbreacher's controller.
// UnregisterReplacementsForPermanent on LTB auto-cleans.
func registerHullbreacher(r *Registry) {
	r.OnETB("Hullbreacher", hullbreacherETB)
}

func hullbreacherETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	registerHullbreacherReplacement(gs, perm)
	emit(gs, "hullbreacher_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"timestamp": perm.Timestamp,
		"effect":    "opponent_draw_to_treasure",
	})
}

func registerHullbreacherReplacement(gs *gameengine.GameState, p *gameengine.Permanent) {
	if gs == nil || p == nil {
		return
	}
	controller := p.Controller
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_draw",
		HandlerID:      "hullbreacher:treasure_redirect:" + p.Card.DisplayName(),
		SourcePerm:     p,
		ControllerSeat: controller,
		Timestamp:      p.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev.TargetSeat == controller {
				return false
			}
			if ev.TargetSeat < 0 || ev.TargetSeat >= len(gs.Seats) {
				return false
			}
			if gs.Seats[ev.TargetSeat] == nil || gs.Seats[ev.TargetSeat].Lost {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			// Same first-draw exemption as Notion Thief. Per-permanent
			// per-turn flag tracks whether the active player's draw-step
			// draw has been seen this turn yet.
			if ev.TargetSeat == gs.Active {
				flagKey := "hullbreacher_normal_draw_seat_" + itoaSC(ev.TargetSeat)
				if p.Flags == nil {
					p.Flags = map[string]int{}
				}
				if p.Flags[flagKey] != gs.Turn {
					p.Flags[flagKey] = gs.Turn
					return // first draw passes through
				}
			}

			victim := ev.TargetSeat
			count := ev.Count()
			ev.Cancelled = true
			// Create one Treasure per cancelled draw.
			for i := 0; i < count; i++ {
				gameengine.CreateTreasureToken(gs, controller)
			}
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Hullbreacher",
				Amount: count,
				Details: map[string]interface{}{
					"rule":      "614",
					"effect":    "draw_to_treasure",
					"victim":    victim,
					"treasures": count,
				},
			})
		},
	})
}
