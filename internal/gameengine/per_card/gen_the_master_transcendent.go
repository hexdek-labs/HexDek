package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheMasterTranscendent wires The Master, Transcendent.
//
// Oracle text:
//
//   When The Master enters, target player gets two rad counters.
//   {T}: Put target creature card in a graveyard that was milled this turn onto the battlefield under your control. It's a green Mutant with base power and toughness 3/3. (It loses its other colors and creature types.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTheMasterTranscendent(r *Registry) {
	r.OnETB("The Master, Transcendent", theMasterTranscendentStaticETB)
}

func theMasterTranscendentStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_master_transcendent_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
