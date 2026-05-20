package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerClaraOswald wires Clara Oswald.
//
// Oracle text:
//
//	Impossible Girl — If Clara Oswald is your commander, choose a color
//	before the game begins. Clara Oswald is the chosen color.
//	If a triggered ability of a Doctor you control triggers, that
//	ability triggers an additional time.
//	Doctor's companion (You can have two commanders if the other is
//	the Doctor.)
//
// Implementation (R46 stub port):
//   - The Doctor-trigger-doubling clause is the live mechanical text.
//     Register a `would_fire_etb_trigger` replacement (the same hook
//     Panharmonicon / Yarok use) filtered by:
//       * ev.Source.Controller == Clara's controller
//       * ev.Source.Card has the "Doctor" subtype
//     On apply, increment Count() by 1. This covers the ETB-trigger
//     surface only; non-ETB triggered abilities (attack, dies, etc.)
//     remain partial because the engine doesn't yet expose a generic
//     would_fire_trigger event.
//   - Impossible Girl color choice and Doctor's companion are setup-
//     time / commander-zone mechanics, no battlefield handler needed.
//
// SourcePerm = Clara, so UnregisterReplacementsForPermanent cleans up
// on LTB.
func registerClaraOswald(r *Registry) {
	r.OnETB("Clara Oswald", claraOswaldRegisterDoctorDoubler)
}

func claraOswaldRegisterDoctorDoubler(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "clara_oswald_doctor_trigger_doubler"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller

	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_fire_etb_trigger",
		HandlerID:      "Clara Oswald:doctor_etb_dbl:" + strconv.Itoa(perm.Timestamp),
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
			return cardHasSubtype(ev.Source.Card, "doctor")
		},
		ApplyFn: func(_ *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Clara Oswald",
				Amount: ev.Count(),
				Details: map[string]interface{}{
					"slug":   slug,
					"rule":   "614",
					"effect": "doctor_etb_trigger_extra",
				},
			})
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     controller,
		"replaces": "would_fire_etb_trigger",
		"filter":   "doctor_subtype",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"non_etb_triggered_abilities_not_doubled_engine_only_exposes_would_fire_etb_trigger")
}
