package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSokratesAthenianTeacher wires Sokrates, Athenian Teacher.
//
// Oracle text:
//
//   Defender
//   Sokrates has hexproof as long as it's untapped.
//   Sokratic Dialogue — {T}: Until end of turn, target creature gains "If this creature would deal combat damage to a player, prevent that damage. This creature's controller and that player each draw half that many cards, rounded down."
//
// Auto-generated activated ability handler.
func registerSokratesAthenianTeacher(r *Registry) {
	r.OnActivated("Sokrates, Athenian Teacher", sokratesAthenianTeacherActivate)
}

func sokratesAthenianTeacherActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sokrates_athenian_teacher_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(), "auto-gen: activated effect not parsed from oracle text")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
