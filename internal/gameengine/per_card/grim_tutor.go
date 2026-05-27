package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGrimTutor wires Grim Tutor.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Grim%20Tutor):
//
//	Search your library for a card, put that card into your hand,
//	then shuffle. You lose 3 life.
//
// {1}{B}{B} Sorcery. Demonic Tutor with a 3-life premium. Reuses the
// tutorToHand helper from tutors.go.
func registerGrimTutor(r *Registry) {
	r.OnResolve("Grim Tutor", grimTutorResolve)
}

func grimTutorResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "grim_tutor"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	found := tutorToHand(gs, seat, nil, "Grim Tutor")
	gameengine.LoseLife(gs, seat, 3, "Grim Tutor")
	emit(gs, slug, "Grim Tutor", map[string]interface{}{
		"seat":      seat,
		"found":     found,
		"life_paid": 3,
	})
	_ = gs.CheckEnd()
}
