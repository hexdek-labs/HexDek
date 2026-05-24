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
	r.OnTrigger("Cloud, Midgar Mercenary", "permanent_ltb", cloudMidgarMercenaryLTB)
}

func cloudMidgarMercenaryLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterReplacementsForPermanent(perm)
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
	// R60 followup: register the would_fire_etb_trigger replacement so
	// Cloud's own ETB triggers (and any triggers fired off the attached
	// Equipment, when Source==attached perm) fire an additional time
	// while Cloud is equipped. The "equipped" gate is checked at
	// replacement-fire time via the Applies predicate so attaching /
	// unequipping flips behavior without re-registration.
	cloudPerm := perm
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_fire_etb_trigger",
		HandlerID:      "Cloud, Midgar Mercenary:equipped_etb_dbl:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(g *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.Source == nil || ev.Count() <= 0 {
				return false
			}
			if !cloudIsEquipped(g, cloudPerm) {
				return false
			}
			// Cloud's own triggered abilities OR the attached equipment's.
			return ev.Source == cloudPerm || ev.Source.AttachedTo == cloudPerm
		},
		ApplyFn: func(g *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.SetCount(ev.Count() + 1)
			g.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   cloudPerm.Controller,
				Source: "Cloud, Midgar Mercenary",
				Amount: ev.Count(),
				Details: map[string]interface{}{
					"slug":   "cloud_midgar_etb_trigger_doubler",
					"rule":   "614",
					"effect": "extra_etb_trigger_when_equipped",
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
}

// cloudIsEquipped reports whether any permanent on Cloud's controller's
// battlefield has its AttachedTo pointer aimed at cloud. Mirrors the
// printed "as long as Cloud is equipped" check.
func cloudIsEquipped(gs *gameengine.GameState, cloud *gameengine.Permanent) bool {
	if gs == nil || cloud == nil {
		return false
	}
	if cloud.Controller < 0 || cloud.Controller >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[cloud.Controller]
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil {
			continue
		}
		if p.AttachedTo == cloud {
			return true
		}
	}
	return false
}

func equipmentName(c *gameengine.Card) string {
	if c == nil {
		return ""
	}
	return c.DisplayName()
}
