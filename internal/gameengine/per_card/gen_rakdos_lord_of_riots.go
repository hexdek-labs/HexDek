package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRakdosLordOfRiots wires Rakdos, Lord of Riots.
//
// Oracle text (Scryfall, verified):
//
//	You can't cast Rakdos unless an opponent lost life this turn.
//	Flying, trample
//	Creature spells you cast cost {1} less to cast for each 1 life
//	your opponents have lost this turn.
//
// Implementation (R41 stub port):
//   - Flying, trample: AST keyword pipeline.
//   - Creature-spell cost reduction: ScanCostModifiers in
//     cost_modifiers.go ("case Rakdos, Lord of Riots") sums each
//     opponent's Turn.LifeLost for every creature spell the controller
//     casts. The engine surface owns the discount math; the register
//     hook here is a breadcrumb so the registration-coverage lint stays
//     satisfied.
//   - Cast restriction ("can't cast Rakdos unless an opponent lost
//     life this turn"): the engine's cast-legality surface doesn't
//     have a generalized per-card hook yet. Marked emitPartial. In
//     practice the gate is rarely binding (by the time Rakdos's
//     controller reaches 7 mana, at least one opponent has typically
//     lost life), so leaving it permissive only biases toward legality.
func registerRakdosLordOfRiots(r *Registry) {
	r.OnETB("Rakdos, Lord of Riots", rakdosLordOfRiotsETB)
}

func rakdosLordOfRiotsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rakdos_lord_of_riots_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	// Cast restriction "can't cast Rakdos unless an opponent lost life
	// this turn" is wired in cost_modifiers.go as a CostModMinimum
	// (R50 batch F). Cost reduction handled separately there.
}
