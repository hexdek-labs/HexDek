package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDictateOfTheTwinGods wires Dictate of the Twin Gods.
//
// Oracle text (Scryfall, verified — Journey into Nyx, {3}{R}{R}):
//
//	Flash
//	If a source would deal damage to a permanent or player, it deals
//	double that damage to that permanent or player instead.
//
// Identical effect-shape to Furnace of Rath; Flash is the
// differentiator and is handled at cast-time by the AST keyword
// pipeline. The damage-doubling closure ships here.
//
// Implementation (R55 — damage replacement primitive):
//   - ETB registers a universal damage-doubling closure.
//   - LTB unregisters via UnregisterDamageReplacementsForPermanent.
//   - Multiple Dictates / Furnaces stack via independent closures.
func registerDictateOfTheTwinGods(r *Registry) {
	r.OnETB("Dictate of the Twin Gods", dictateOfTheTwinGodsETBRegisterReplacement)
	r.OnTrigger("Dictate of the Twin Gods", "permanent_ltb", dictateOfTheTwinGodsLTBUnregister)
}

func dictateOfTheTwinGodsETBRegisterReplacement(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "dictate_of_the_twin_gods_double_all_damage"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: perm,
		HandlerID:  "dictate_twin_gods_double",
		Fn: func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
			if ctx == nil || ctx.Amount <= 0 {
				return
			}
			ctx.Amount *= 2
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "double_all_damage",
	})
}

func dictateOfTheTwinGodsLTBUnregister(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterDamageReplacementsForPermanent(perm)
}
