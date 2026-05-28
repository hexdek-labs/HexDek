package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDiabolicTutor wires Diabolic Tutor.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Diabolic%20Tutor):
//
//	Search your library for a card, put that card into your hand, then
//	shuffle.
//
// {2}{B}{B} Sorcery. The vanilla 4-mana hand-tutor template — the
// baseline every other black tutor is benchmarked against. Strictly
// outclassed by Demonic Tutor at 2 mana, but Diabolic Tutor is on
// budget shelves where Demonic isn't, and it's a B4-tier marker in
// non-cEDH brackets where the Reserved List staples don't exist.
//
// Implementation:
//   - OnResolve. Wraps tutorToHand from tutors.go with no filter —
//     identical to Demonic Tutor's body, distinct cost only.
func registerDiabolicTutor(r *Registry) {
	r.OnResolve("Diabolic Tutor", diabolicTutorResolve)
}

func diabolicTutorResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "diabolic_tutor"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	found := tutorToHand(gs, seat, nil, "Diabolic Tutor")
	emit(gs, slug, "Diabolic Tutor", map[string]interface{}{
		"seat":  seat,
		"found": found,
	})
}
