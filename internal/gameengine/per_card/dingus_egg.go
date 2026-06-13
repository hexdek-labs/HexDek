package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// dingus_egg.go — per_card handler for Dingus Egg.
//
// Oracle text (Artifact, {4}):
//
//	Whenever a land is put into a graveyard from the battlefield, this
//	artifact deals 2 damage to that land's controller.
//
// Why this needs a per_card handler: the trigger parsed to inert
// scaffold, so Dingus Egg — the classic land-destruction punisher —
// dealt no damage. Verified inert: no registered handler for "Dingus
// Egg".
//
// Implemented here (OnTrigger land_to_graveyard): on a land moving from
// the battlefield to a graveyard, deal 2 to that land's controller
// (modelled via LoseLife so §704.5a bookkeeping fires). Symmetric — it
// fires for every land that dies, regardless of whose it was.

func init() {
	registerDingusEgg(Global())
	AddResetHook(registerDingusEgg)
}

func registerDingusEgg(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Dingus Egg", "land_to_graveyard", dingusEggLandDies)
}

func dingusEggLandDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "dingus_egg"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Only "from the battlefield".
	if from, ok := ctx["from_zone"].(string); ok && from != "battlefield" {
		return
	}
	victim, ok := ctx["owner_seat"].(int)
	if !ok || victim < 0 || victim >= len(gs.Seats) {
		return
	}
	gameengine.LoseLife(gs, victim, 2, "Dingus Egg")
	emit(gs, slug, "Dingus Egg", map[string]interface{}{
		"seat":   perm.Controller,
		"victim": victim,
	})
	_ = gs.CheckEnd()
}
