package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEonFrolicker wires Eon Frolicker (Muninn parser-gap #56, 13,197
// hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{2}{U}{U}
//	Creature — Elemental Otter
//	Flying
//	When this creature enters, if you cast it, target opponent takes an
//	extra turn after this one. Until your next turn, you and
//	planeswalkers you control gain protection from that player.
//
// Implementation (R49 stub port — batch A):
//   - Flying is AST-side.
//   - ETB gated on perm.Flags["was_cast"] == 1.
//   - Target opponent pick: lowest-life living opponent (least scary
//     extra turn to give away — and they're closest to death from the
//     accumulated draw/land they get).
//   - Extra turn: bump gs.Flags["extra_turns_pending"] AND record the
//     target seat in gs.Flags["extra_turn_target_seat"] (+1 to allow
//     0-seat encoding) so any future seat-aware extra-turn consumer
//     can route it. Without that consumer the bump still triggers the
//     existing extra-turn queue logic and downstream observers — at
//     worst the wrong seat takes it. emitPartial flags the gap.
//   - Protection clause: stamp seat.Flags["eon_frolicker_protection_<target>_until_turn_<N+1>"]
//     on the controller's seat. The static protection-from-player
//     pipeline isn't wired; emitPartial flags it but the flag is
//     readable by future static layers.
func registerEonFrolicker(r *Registry) {
	r.OnETB("Eon Frolicker", eonFrolickerETB)
}

func eonFrolickerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "eon_frolicker_etb_opp_extra_turn"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil || perm.Flags["was_cast"] != 1 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": false,
			"reason":    "not_cast",
		})
		return
	}
	target := -1
	bestLife := 1 << 30
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent", nil)
		return
	}

	// Bump the extra-turn queue + record the intended target seat.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["extra_turns_pending"]++
	gs.Flags["extra_turn_target_seat"] = target + 1 // +1 so seat 0 is distinguishable from "unset"

	// Stamp protection-from-target on the controller's seat.
	ctlSeat := gs.Seats[perm.Controller]
	if ctlSeat != nil {
		if ctlSeat.Flags == nil {
			ctlSeat.Flags = map[string]int{}
		}
		ctlSeat.Flags["eon_frolicker_protection_from_seat"] = target + 1
		ctlSeat.Flags["eon_frolicker_protection_until_turn"] = gs.Turn + 1
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "extra_turn",
		Seat:   target,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"target_seat": target,
			"slug":        slug,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"target":   target,
		"opp_life": bestLife,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"extra_turn_routed_via_target_seat_flag_engine_lacks_per_seat_extra_turn_queue")
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"protection_from_target_player_stamped_on_seat_flags_static_layer_not_wired")
}
