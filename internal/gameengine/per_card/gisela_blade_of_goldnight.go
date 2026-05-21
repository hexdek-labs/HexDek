package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGiselaBladeOfGoldnight wires Gisela, Blade of Goldnight.
//
// Oracle text (Scryfall, verified — Avacyn Restored, {4}{R}{W}{W}):
//
//	Flying, first strike, lifelink
//	If a source would deal damage to an opponent or a permanent an
//	opponent controls, that source deals double that damage to that
//	player or permanent instead.
//	If a source would deal damage to you, prevent half that damage,
//	rounded up.
//
// Implementation (R55 — damage replacement primitive):
//   - ETB registers TWO replacement closures:
//       (a) Damage to an opponent or opponent's permanent → ×2.
//           Note: oracle says "a source" (not "a source you control"),
//           so OPPONENT-VS-OPPONENT damage also doubles when targeting
//           an opponent. Multi-Gisela boards therefore stack — each
//           closure doubles independently.
//       (b) Damage to Gisela's controller → halve, rounded up
//           (subtraction modeled: amount -= ceil(amount/2)).
//   - Two HandlerIDs so the two closures can be tested independently.
//   - LTB unregisters both via UnregisterDamageReplacementsForPermanent.
func registerGiselaBladeOfGoldnight(r *Registry) {
	r.OnETB("Gisela, Blade of Goldnight", giselaBladeETBRegisterReplacements)
	r.OnTrigger("Gisela, Blade of Goldnight", "permanent_ltb", giselaBladeLTBUnregister)
}

func giselaBladeETBRegisterReplacements(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "gisela_blade_register_replacements"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller

	// (a) Double damage to opponents / opponent permanents.
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: perm,
		HandlerID:  "gisela_blade_double_to_opps",
		Fn: func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
			if ctx == nil || ctx.Amount <= 0 {
				return
			}
			// "an opponent or a permanent an opponent controls" — the
			// TargetSeat field is set to the affected player or to the
			// target permanent's controller. Equal to Gisela's
			// controller = it's YOU → don't double here (the halving
			// closure handles that branch separately).
			if ctx.TargetSeat == controller {
				return
			}
			ctx.Amount *= 2
		},
	})

	// (b) Halve damage to Gisela's controller, rounded up = lose ceil(amount/2).
	// Implemented as ctx.Amount -= floor(amount/2), which leaves the
	// rounded-up half. Example: amount=5 → halved=ceil(5/2)=3 → loss=3.
	// floor(5/2)=2 subtracted leaves 3. Matches printed text.
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: perm,
		HandlerID:  "gisela_blade_halve_to_self",
		Fn: func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
			if ctx == nil || ctx.Amount <= 0 {
				return
			}
			if ctx.TargetSeat != controller {
				return
			}
			ctx.Amount -= ctx.Amount / 2
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": controller,
	})
}

func giselaBladeLTBUnregister(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterDamageReplacementsForPermanent(perm)
}
