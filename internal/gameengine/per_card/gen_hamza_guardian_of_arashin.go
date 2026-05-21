package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHamzaGuardianOfArashin wires Hamza, Guardian of Arashin.
//
// Oracle text (Scryfall, verified):
//
//	This spell costs {1} less to cast for each creature you control
//	with a +1/+1 counter on it.
//	Creature spells you cast cost {1} less to cast for each creature
//	you control with a +1/+1 counter on it.
//
// Implementation (R43 stub port):
//   - Creature-spell cost reduction: handled engine-side by the new
//     "Hamza, Guardian of Arashin" case in
//     gameengine/cost_modifiers.go ScanCostModifiers. Counts all
//     creatures the controller controls that have ≥1 +1/+1 counter
//     and reduces by that many {1}s for any creature spell the
//     controller casts (mirrors the Animar pattern).
//   - Self-cost reduction (first clause "this spell costs..."):
//     marked emitPartial. When Hamza is on the battlefield as a
//     permanent and gets re-cast (commander recast, flicker copy),
//     the second clause already covers him because he's a creature
//     spell. The first-cast-from-command-zone path would need a
//     dedicated command-zone scan analogous to Ur-Dragon's
//     eminence — extending that pattern is a follow-up.
//   - The register hook here is the breadcrumb so the registration-
//     coverage lint stays satisfied; the engine surface owns the
//     discount math.
func registerHamzaGuardianOfArashin(r *Registry) {
	r.OnETB("Hamza, Guardian of Arashin", hamzaGuardianOfArashinETB)
}

func hamzaGuardianOfArashinETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "hamza_guardian_of_arashin_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	// Self-cost reduction (first clause) wired in cost_modifiers.go as
	// a top-level Hamza self-cast scan (R50 batch F).
}
