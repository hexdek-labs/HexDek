package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheSecondDoctor wires The Second Doctor.
//
// Oracle text (Scryfall, verified):
//
//	Players have no maximum hand size.
//	How Civil of You — At the beginning of your end step, each player
//	may draw a card. Each opponent who does can't attack you or
//	permanents you control during their next turn.
//
// Implementation (R42b stub port):
//   - No maximum hand size: AST keyword pipeline.
//   - End-step draw: OnTrigger("end_step") gated to active_seat ==
//     perm.Controller. AI policy is greedy-upside ("may draw" is
//     monotone for the drawer — refilling hand is always net-positive
//     and the attack-restriction rider is the drawer's incentive to
//     accept the rider in exchange for the card).
//   - Per-opponent attack restriction: each opponent that actually
//     drew gets seat.Flags["second_doctor_no_attack_seat_<doctor>"]
//     stamped to gs.Turn+1 (the next turn-boundary number — when
//     that opponent's own turn comes around, the flag is read by
//     combat-layer enforcement, which is not yet wired). The flag
//     is the canonical breadcrumb for the future combat-side
//     consumer; emitPartial documents the gap.
func registerTheSecondDoctor(r *Registry) {
	r.OnETB("The Second Doctor", theSecondDoctorETB)
	r.OnTrigger("The Second Doctor", "end_step", theSecondDoctorHowCivil)
}

func theSecondDoctorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_second_doctor_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"no_maximum_hand_size_static_handled_by_AST_keyword_pipeline")
}

func theSecondDoctorHowCivil(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "second_doctor_how_civil_of_you"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	doctorSeat := perm.Controller
	flagKey := doctorAttackBlockKey(doctorSeat)

	drawers := []int{}
	totalDrew := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		c := drawOne(gs, i, perm.Card.DisplayName())
		if c == nil {
			continue
		}
		totalDrew++
		if i == doctorSeat {
			continue
		}
		if s.Flags == nil {
			s.Flags = map[string]int{}
		}
		s.Flags[flagKey] = gs.Turn + 1
		drawers = append(drawers, i)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       doctorSeat,
		"drawers":    drawers,
		"total_drew": totalDrew,
		"flag_set":   flagKey,
		"flag_value": gs.Turn + 1,
	})
	if len(drawers) > 0 {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"attack_restriction_combat_layer_enforcement_not_yet_wired")
	}
}

// doctorAttackBlockKey returns the seat-flag key stamped on each
// opponent who drew from a Second Doctor end-step trigger. Format
// mirrors the propaganda_seat_N pattern in combat_restrictions.go.
func doctorAttackBlockKey(doctorSeat int) string {
	return "second_doctor_no_attack_seat_" + itoa(doctorSeat)
}
