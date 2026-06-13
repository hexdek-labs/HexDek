package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// dark_prophecy.go — per_card handler for Dark Prophecy.
//
// Oracle text (Enchantment, {B}{B}{B}):
//
//	Whenever a creature you control dies, you draw a card and you lose 1
//	life.
//
// Why this needs a per_card handler: the trigger parsed to inert
// scaffold, so this mono-black card-draw engine drew nothing. Verified
// inert: no registered handler for "Dark Prophecy".
//
// Implemented here (OnTrigger creature_dies): when a creature you control
// dies, draw a card and lose 1 life (LoseLife so §704.5a bookkeeping
// fires).

func init() {
	registerDarkProphecy(Global())
	AddResetHook(registerDarkProphecy)
}

func registerDarkProphecy(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Dark Prophecy", "creature_dies", darkProphecyDies)
}

func darkProphecyDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "dark_prophecy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	deadSeat, ok := ctx["controller_seat"].(int)
	if !ok || deadSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	drawOne(gs, seat, "Dark Prophecy")
	gameengine.LoseLife(gs, seat, 1, "Dark Prophecy")
	emit(gs, slug, "Dark Prophecy", map[string]interface{}{"seat": seat})
	_ = gs.CheckEnd()
}
