package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFurnaceOfRath wires Furnace of Rath.
//
// Oracle text (Scryfall, verified — Tempest, {3}{R}{R}):
//
//	If a source would deal damage to a permanent or player, it deals
//	double that damage to that permanent or player instead.
//
// Implementation (R55 — damage replacement primitive):
//   - ETB registers a DamageReplacement closure. Filter is universal:
//     every damage event (every source, every target) doubles.
//   - LTB unregisters via UnregisterDamageReplacementsForPermanent so
//     a bounced / destroyed Furnace stops doubling damage.
//   - Multiple Furnaces stack: each registered replacement doubles
//     independently, so two Furnaces = 4x, three = 8x. Matches the
//     CR §616 multiple-replacement application ordering (controller-
//     of-affected-object picks, but doubling is associative so order
//     doesn't matter).
//   - Symmetric: Furnace doesn't care who controls the source or who
//     the target is. This is the headline "everyone's burn does
//     double damage" effect.
func registerFurnaceOfRath(r *Registry) {
	r.OnETB("Furnace of Rath", furnaceOfRathETBRegisterReplacement)
	r.OnTrigger("Furnace of Rath", "permanent_ltb", furnaceOfRathLTBUnregister)
}

func furnaceOfRathETBRegisterReplacement(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "furnace_of_rath_double_all_damage"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: perm,
		HandlerID:  "furnace_of_rath_double",
		Fn: func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
			if ctx == nil || ctx.Amount <= 0 {
				return
			}
			ctx.Amount *= 2
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"effect":   "double_all_damage",
		"symmetry": "universal",
	})
}

func furnaceOfRathLTBUnregister(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterDamageReplacementsForPermanent(perm)
}
