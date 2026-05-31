package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Sek'Kuar, Deathkeeper — {2}{B}{R}{G} 4/4 Legendary Creature — Orc Warrior.
//
//   Whenever another nontoken creature you control dies, create a 3/1
//   black and red Graveborn creature token with haste.
//
// Implementation: OnTrigger creature_dies, filter:
//   - dying creature is non-token (per "nontoken creature you control")
//   - dying creature's controller == Sek'Kuar's controller
//   - dying creature is not Sek'Kuar himself ("another")
// Spawn a 3/1 BR Graveborn with haste via CreateCreatureToken.

func init() {
	registerSekKuarDeathkeeper(Global())
	AddResetHook(registerSekKuarDeathkeeper)
}

func registerSekKuarDeathkeeper(r *Registry) {
	r.OnTrigger("Sek'Kuar, Deathkeeper", "creature_dies", sekKuarCreatureDies)
}

func sekKuarCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sek_kuar_graveborn_token"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying == nil || dying == perm || dying.Card == nil {
		return
	}
	if !dying.IsCreature() {
		return
	}
	if dying.IsToken() {
		return
	}
	if dying.Controller != perm.Controller {
		return
	}
	token := gameengine.CreateCreatureToken(gs, perm.Controller, "Graveborn Token",
		[]string{"creature", "graveborn", "pip:B", "pip:R", "haste"}, 3, 1)
	if token == nil {
		return
	}
	token.SummoningSick = false
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"trigger":  dying.Card.DisplayName(),
		"token":    "Graveborn Token (3/1 BR haste)",
	})
}
