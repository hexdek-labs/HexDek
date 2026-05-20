package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKataraTheFearless wires Katara, the Fearless.
//
// Oracle text:
//
//	If a triggered ability of an Ally you control triggers, that
//	ability triggers an additional time.
//
// Implementation (R46 stub port):
//   - Same shape as Clara Oswald's Doctor-trigger doubler: register a
//     `would_fire_etb_trigger` replacement filtered to allied creature
//     ETB triggers controlled by Katara's controller. Non-ETB triggers
//     stay partial (engine only exposes the ETB-trigger event today).
//
// SourcePerm = Katara, so LTB cleanup is automatic.
func registerKataraTheFearless(r *Registry) {
	r.OnETB("Katara, the Fearless", kataraTheFearlessRegisterAllyDoubler)
}

func kataraTheFearlessRegisterAllyDoubler(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "katara_the_fearless_ally_trigger_doubler"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller

	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_fire_etb_trigger",
		HandlerID:      "Katara, the Fearless:ally_etb_dbl:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(_ *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.Source == nil || ev.Source.Card == nil {
				return false
			}
			if ev.Source.Controller != controller {
				return false
			}
			if ev.Count() <= 0 {
				return false
			}
			return cardHasSubtype(ev.Source.Card, "ally")
		},
		ApplyFn: func(_ *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Katara, the Fearless",
				Amount: ev.Count(),
				Details: map[string]interface{}{
					"slug":   slug,
					"rule":   "614",
					"effect": "ally_etb_trigger_extra",
				},
			})
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     controller,
		"replaces": "would_fire_etb_trigger",
		"filter":   "ally_subtype",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"non_etb_triggered_abilities_not_doubled_engine_only_exposes_would_fire_etb_trigger")
}
