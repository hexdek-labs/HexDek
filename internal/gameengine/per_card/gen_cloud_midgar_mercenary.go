package per_card

import (
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
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"tutored":   best != nil,
		"equipment": equipmentName(best),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"trigger-doubling-when-equipped needs trigger-dispatch hook; flag set for downstream consumers")
}

func equipmentName(c *gameengine.Card) string {
	if c == nil {
		return ""
	}
	return c.DisplayName()
}
