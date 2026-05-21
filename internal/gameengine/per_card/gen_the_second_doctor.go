package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheSecondDoctor wires The Second Doctor.
//
// Oracle text (Scryfall, verified, {2}{U}{R}, 3/4 legendary):
//
//	Players have no maximum hand size.
//	How Civil of You — At the beginning of your end step, each player
//	may draw a card. Each opponent who does can't attack you or
//	permanents you control during their next turn.
//
// Implementation (R42 stub port):
//   - "Players have no maximum hand size" is a global static rule; it
//     belongs to the discard-step / max-hand-size enforcement layer,
//     not per_card. We record a no_max_hand_size flag on each seat at
//     ETB so any downstream max-hand-size scanner can observe the
//     grant, and emitPartial breadcrumb for the actual cleanup-step
//     wiring.
//   - end_step trigger gated on active_seat == controller. Greedy
//     policy: every living player draws (a free card is never
//     declined). Each opponent who actually drew gets a per-seat
//     flag "cant_attack_seat_<N>_until_turn_<T>" set on their Flags
//     so the next-turn attack-legality check can read it.
//   - The "can't attack you or permanents you control" rider is a
//     declare-attackers restriction; we stamp the seat flag here but
//     the actual restriction enforcement is engine territory —
//     emitPartial breadcrumb so any future combat-restriction
//     scanner can pick it up.
func registerTheSecondDoctor(r *Registry) {
	r.OnETB("The Second Doctor", theSecondDoctorETB)
	r.OnTrigger("The Second Doctor", "end_step", theSecondDoctorHowCivilOfYou)
	// R51 batch I: LTB clears the no_max_hand_size flag on each seat
	// (only when no OTHER Second Doctor remains in play) so the
	// cleanup-step max-hand-size scanner reverts. Also clears the
	// per-opponent cant_attack_doctor_controller restriction tied to
	// this Doctor's controller, since the restriction is rooted in
	// the printed "you" — when the Doctor leaves, the controller's
	// claim on the restriction goes with it.
	r.OnTrigger("The Second Doctor", "permanent_ltb", theSecondDoctorLTB)
}

func theSecondDoctorLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	// Bail if another Second Doctor still in play (any seat — the
	// printed static applies globally).
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p == perm || p.Card == nil {
				continue
			}
			if normalizeName(p.Card.DisplayName()) == normalizeName("The Second Doctor") {
				return
			}
		}
	}
	for _, s := range gs.Seats {
		if s == nil || s.Flags == nil {
			continue
		}
		delete(s.Flags, "no_max_hand_size")
		// Clear the attack-restriction stamps tied to this Doctor's
		// controller. The encoded payload is controller_seat + 1.
		if s.Flags["cant_attack_doctor_controller"] == perm.Controller+1 {
			delete(s.Flags, "cant_attack_doctor_controller")
			delete(s.Flags, "cant_attack_doctor_controller_until_turn")
		}
	}
}

func theSecondDoctorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_second_doctor_no_max_hand_size"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	stamped := 0
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.Flags == nil {
			s.Flags = map[string]int{}
		}
		s.Flags["no_max_hand_size"] = 1
		_ = i
		stamped++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"seats_flagged":   stamped,
		"flag":            "no_max_hand_size",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"no_max_hand_size_global_static_observed_via_seat_flag_cleanup_step_enforcement_not_wired_at_per_card_layer")
}

func theSecondDoctorHowCivilOfYou(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_second_doctor_how_civil_of_you"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}

	type drawResult struct {
		seat int
		drew bool
	}
	var results []drawResult
	// Each living player draws. We always draw for the controller
	// first (their own end step), then opponents in seat order.
	order := []int{perm.Controller}
	for i := range gs.Seats {
		if i == perm.Controller {
			continue
		}
		order = append(order, i)
	}
	for _, idx := range order {
		s := gs.Seats[idx]
		if s == nil || s.Lost {
			continue
		}
		c := drawOne(gs, idx, perm.Card.DisplayName())
		drew := c != nil
		results = append(results, drawResult{seat: idx, drew: drew})
		if drew && idx != perm.Controller {
			if s.Flags == nil {
				s.Flags = map[string]int{}
			}
			// Stamp the no-attack-vs-controller flag for next turn.
			s.Flags["cant_attack_doctor_controller"] = perm.Controller + 1
			s.Flags["cant_attack_doctor_controller_until_turn"] = gs.Turn + 2
		}
	}
	drewSummary := map[int]bool{}
	for _, r := range results {
		drewSummary[r.seat] = r.drew
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"drew": drewSummary,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"cant_attack_doctor_controller_restriction_recorded_via_seat_flag_declare_attackers_enforcement_not_wired")
}
