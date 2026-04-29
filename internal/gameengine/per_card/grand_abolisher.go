package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGrandAbolisher wires up Grand Abolisher.
//
// Oracle text:
//
//	Your opponents can't cast spells during your turn and can't
//	activate abilities of artifacts, creatures, or enchantments
//	during your turn.
//
// The canonical "combo shields" creature. Drop Abolisher on turn 2-3,
// then combo on turn 4-5 without fear of counterspells or board-wipe
// activations during your own turn.
//
// Batch #2 scope:
//   - OnETB: stamp gs.Flags["grand_abolisher_active_seat_N"] = 1 (N =
//     controller seat). The engine's CastSpell + activated-ability
//     paths read this flag and refuse opponent casts / activations
//     during seat N's turn.
//   - We DON'T patch CastSpell here — that's engine-side work. The
//     flag exists for the engine to consume when that work lands. For
//     MVP, log partial.
//
// The restriction is seat-specific but turn-gated: an opponent CAN
// cast spells on their OWN turn (or any turn that isn't Abolisher's
// controller's). Enforcement happens at the call site (cast_restriction
// check, which the engine team can wire to these flags).
func registerGrandAbolisher(r *Registry) {
	r.OnETB("Grand Abolisher", grandAbolisherETB)
}

func grandAbolisherETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "grand_abolisher_restriction"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["grand_abolisher_active_seat_"+intToStr(seat)] = perm.Timestamp
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"cast_restriction_enforcement_is_engine_side_flag_set_but_not_consumed_yet")
}

// GrandAbolisherActive returns true if seat `duringTurn` has a Grand
// Abolisher in play that restricts opponent casts. Intended to be
// called by the engine's CastSpell / ActivateAbility legality pass
// when that wiring lands.
func GrandAbolisherActive(gs *gameengine.GameState, duringTurn int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["grand_abolisher_active_seat_"+intToStr(duringTurn)] > 0
}
