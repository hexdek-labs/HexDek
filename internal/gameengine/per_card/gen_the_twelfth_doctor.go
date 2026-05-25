package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheTwelfthDoctor wires The Twelfth Doctor.
//
// Oracle text:
//
//	The first spell you cast from anywhere other than your hand each
//	turn has demonstrate. (When you cast that spell, you may copy it.
//	If you do, choose an opponent to also copy it. A copy of a
//	permanent spell becomes a token.)
//	Whenever you copy a spell, put a +1/+1 counter on The Twelfth
//	Doctor.
//
// Implementation (R46 stub port):
//   - OnTrigger("spell_copied") — dispatched by resolve.go's spell-copy
//     resolution path. When the copying seat matches Twelfth Doctor's
//     controller, add a +1/+1 counter. Per-card-driven copies (Kalamax,
//     Alania, Riku) don't reach the resolve.go path and don't fire;
//     that's flagged via emitPartial on ETB.
//   - The demonstrate-grant clause (first non-hand cast per turn gets
//     demonstrate) is a cast-pipeline rider not exposed at the
//     per_card layer — remains partial.
func registerTheTwelfthDoctor(r *Registry) {
	r.OnETB("The Twelfth Doctor", theTwelfthDoctorETB)
	r.OnTrigger("The Twelfth Doctor", "spell_copied", theTwelfthDoctorCounter)
	r.OnTrigger("The Twelfth Doctor", "spell_cast", theTwelfthDoctorConsumeGrant)
}

func theTwelfthDoctorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_twelfth_doctor_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat != nil {
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["twelfth_doctor_demonstrate_pending"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":               perm.Controller,
		"demonstrate_armed": 1,
	})
}

// theTwelfthDoctorConsumeGrant consumes the demonstrate-pending flag on
// the first spell the controller casts from a zone other than hand each
// turn. The flag is re-armed by the per-turn untap helper (or simply by
// re-ETB), so this handler only needs to clear it.
func theTwelfthDoctorConsumeGrant(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	zone, _ := ctx["cast_zone"].(string)
	if zone == "" || zone == "hand" {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	if seat.Flags["twelfth_doctor_demonstrate_pending"] <= 0 {
		return
	}
	seat.Flags["twelfth_doctor_demonstrate_pending"] = 0
	emit(gs, "the_twelfth_doctor_demonstrate_consumed", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"cast_zone": zone,
	})
}

func theTwelfthDoctorCounter(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_twelfth_doctor_copy_counter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	perm.AddCounter("+1/+1", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counter":  "+1/+1",
		"new_pow":  perm.Power(),
	})
}
