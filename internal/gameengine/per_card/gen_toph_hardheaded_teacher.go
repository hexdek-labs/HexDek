package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTophHardheadedTeacher wires Toph, Hardheaded Teacher.
//
// Oracle text:
//
//   When Toph enters, you may discard a card. If you do, return target instant or sorcery card from your graveyard to your hand.
//   Whenever you cast a spell, earthbend 1. If that spell is a Lesson, put an additional +1/+1 counter on that land. (Target land you control becomes a 0/0 creature with haste that's still a land. Put a +1/+1 counter on it. When it dies or is exiled, return it to the battlefield tapped.)
//
// Auto-generated ETB handler.
func registerTophHardheadedTeacher(r *Registry) {
	r.OnETB("Toph, Hardheaded Teacher", tophHardheadedTeacherETB)
}

func tophHardheadedTeacherETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "toph_hardheaded_teacher_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
