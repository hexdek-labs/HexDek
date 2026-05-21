package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCaradoraHeartOfAlacria wires Caradora, Heart of Alacria.
//
// Oracle text:
//
//	When Caradora enters, you may search your library for a Mount or
//	Vehicle card, reveal it, put it into your hand, then shuffle.
//	If one or more +1/+1 counters would be put on a creature or
//	Vehicle you control, that many plus one +1/+1 counters are put on
//	it instead.
//
// Implementation (R50 batch F):
//   - ETB tutor: highest-CMC Mount or Vehicle from library → hand,
//     shuffle.
//   - +1/+1 replacement: register a "would_put_counter" replacement
//     (Hardened Scales pattern) gated to creatures or Vehicles the
//     controller controls and to "+1/+1" counter_type. Adds +1 to
//     ev.Count().
//   - SourcePerm = Caradora, so UnregisterReplacementsForPermanent
//     cleans up on LTB.
func registerCaradoraHeartOfAlacria(r *Registry) {
	r.OnETB("Caradora, Heart of Alacria", caradoraETBTutorAndReplaceFlag)
}

func caradoraETBTutorAndReplaceFlag(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "caradora_etb_tutor_and_replace"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Library {
		if c == nil {
			continue
		}
		if !cardHasSubtype(c, "mount") && !cardHasSubtype(c, "vehicle") &&
			!cardHasType(c, "vehicle") {
			continue
		}
		if cmc := cardCMC(c); cmc > bestCMC {
			best = c
			bestCMC = cmc
		}
	}
	if best != nil {
		gameengine.MoveCard(gs, best, perm.Controller, "library", "hand", "caradora_tutor")
		if len(seat.Library) > 1 && gs.Rng != nil {
			gs.Rng.Shuffle(len(seat.Library), func(i, j int) {
				seat.Library[i], seat.Library[j] = seat.Library[j], seat.Library[i]
			})
		}
	}

	// Register the +1/+1 counter replacement effect.
	controller := perm.Controller
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_put_counter",
		HandlerID:      "Caradora, Heart of Alacria:plus_one_counter:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(_ *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.TargetPerm == nil {
				return false
			}
			if ev.TargetPerm.Controller != controller {
				return false
			}
			if ev.String("counter_type") != "+1/+1" {
				return false
			}
			// "creature or Vehicle you control"
			if ev.TargetPerm.Card == nil {
				return false
			}
			tc := ev.TargetPerm.Card
			if !ev.TargetPerm.IsCreature() && !cardHasSubtype(tc, "vehicle") && !cardHasType(tc, "vehicle") {
				return false
			}
			return ev.Count() > 0
		},
		ApplyFn: func(_ *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Caradora, Heart of Alacria",
				Amount: ev.Count(),
				Details: map[string]interface{}{
					"rule":   "614",
					"effect": "plus_one_counter_for_creature_or_vehicle",
				},
			})
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"tutored": best != nil,
		"target":  equipmentName(best),
	})
}
