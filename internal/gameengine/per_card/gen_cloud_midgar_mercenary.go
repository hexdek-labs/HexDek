package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCloudMidgarMercenary wires Cloud, Midgar Mercenary.
//
// Oracle text:
//
//	When Cloud enters, search your library for an Equipment card,
//	reveal it, put it into your hand, then shuffle.
//	As long as Cloud is equipped, if a triggered ability of Cloud or
//	an Equipment attached to it triggers, that ability triggers an
//	additional time.
//
// Implementation:
//   - ETB tutors the highest-CMC Equipment from the library to hand.
//   - The "trigger doubler when equipped" static is engine-deep
//     (parallels Panharmonicon at the trigger-dispatch layer); we set
//     a per-permanent flag so an aware engine can branch on it and
//     emit a partial breadcrumb.
func registerCloudMidgarMercenary(r *Registry) {
	r.OnETB("Cloud, Midgar Mercenary", cloudMidgarMercenaryETB)
}

func cloudMidgarMercenaryETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cloud_midgar_mercenary_etb_equipment_tutor"
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
		if c == nil || !cardHasType(c, "equipment") {
			continue
		}
		if cmc := cardCMC(c); cmc > bestCMC {
			best = c
			bestCMC = cmc
		}
	}
	if best != nil {
		gameengine.MoveCard(gs, best, perm.Controller, "library", "hand", "cloud_equipment_tutor")
		if len(seat.Library) > 1 && gs.Rng != nil {
			gs.Rng.Shuffle(len(seat.Library), func(i, j int) {
				seat.Library[i], seat.Library[j] = seat.Library[j], seat.Library[i]
			})
		}
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["cloud_trigger_doubler_when_equipped"] = 1

	// R51 batch H port: register a would_fire_etb_trigger replacement
	// (Panharmonicon shape) that doubles ETB triggers from Cloud or
	// any Equipment currently attached to him — but only when Cloud
	// is actually equipped (AttachedTo != nil on at least one of his
	// equipped artifacts) at the time the event fires. Non-ETB
	// triggers (attack, dies, etc.) remain partial because the engine
	// only exposes the ETB-trigger event today.
	controller := perm.Controller
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_fire_etb_trigger",
		HandlerID:      "Cloud, Midgar Mercenary:equipped_trigger_dbl:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.Source == nil || ev.Source.Card == nil {
				return false
			}
			if ev.Source.Controller != controller {
				return false
			}
			if ev.Count() <= 0 {
				return false
			}
			// Only fire while Cloud is equipped — scan the controller's
			// battlefield for an Equipment whose AttachedTo points at
			// Cloud.
			ourSeat := gs.Seats[controller]
			if ourSeat == nil {
				return false
			}
			equipped := false
			for _, p := range ourSeat.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if p.AttachedTo == perm && cardHasType(p.Card, "equipment") {
					equipped = true
					break
				}
			}
			if !equipped {
				return false
			}
			// Filter to Cloud himself or an attached Equipment.
			if ev.Source == perm {
				return true
			}
			if ev.Source.AttachedTo == perm && cardHasType(ev.Source.Card, "equipment") {
				return true
			}
			return false
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Cloud, Midgar Mercenary",
				Amount: ev.Count(),
				Details: map[string]interface{}{
					"slug":   slug,
					"rule":   "614",
					"effect": "cloud_equipped_etb_trigger_extra",
				},
			})
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"tutored":   best != nil,
		"equipment": equipmentName(best),
		"replaces":  "would_fire_etb_trigger",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"non_etb_triggered_abilities_not_doubled_engine_only_exposes_would_fire_etb_trigger")
}

func equipmentName(c *gameengine.Card) string {
	if c == nil {
		return ""
	}
	return c.DisplayName()
}
