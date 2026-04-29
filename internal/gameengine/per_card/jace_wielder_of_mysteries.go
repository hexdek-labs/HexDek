package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJaceWielderOfMysteries wires up Jace, Wielder of Mysteries.
//
// Oracle text:
//
//	+1: Draw a card.
//	-8: Exile the top ten cards of your library. You may cast any
//	    number of nonland cards from among them without paying their
//	    mana costs.
//	If you would draw a card and your library is empty, you win the
//	game instead.
//
// The static replacement ("win on empty-library draw") is the
// identical shape to Laboratory Maniac — CR §614. Registration lives
// in internal/gameengine/replacement.go (RegisterJaceWielderOfMysteries)
// and the auto-dispatcher RegisterReplacementsForPermanent hooks it at
// ETB. This per_card handler ships an ETB observer identical to the
// Laboratory Maniac pattern for audit visibility.
//
// The +1/-8 loyalty abilities are NOT implemented here — planeswalker
// activation timing is engine-side work (it routes through activate_
// ability + loyalty cost enforcement). Tests can still verify the
// alt-win works via the ETB path.
func registerJaceWielderOfMysteries(r *Registry) {
	r.OnETB("Jace, Wielder of Mysteries", jaceWielderETB)
}

func jaceWielderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "jace_wielder_altwin"
	if gs == nil || perm == nil {
		return
	}
	gameengine.RegisterJaceWielderOfMysteries(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"rule":     "614",
		"replaces": "would_draw_from_empty_library",
		"outcome":  "controller_wins",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"plusone_and_minuseight_loyalty_abilities_not_implemented_in_batch2")
}
