package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerChainerDementiaMaster wires Chainer, Dementia Master.
//
// Oracle text:
//
//   All Nightmares get +1/+1.
//   {B}{B}{B}, Pay 3 life: Put target creature card from a graveyard onto the battlefield under your control. That creature is black and is a Nightmare in addition to its other creature types.
//   When Chainer leaves the battlefield, exile all Nightmares.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerChainerDementiaMaster(r *Registry) {
	r.OnETB("Chainer, Dementia Master", chainerDementiaMasterStaticETB)
}

func chainerDementiaMasterStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "chainer_dementia_master_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
