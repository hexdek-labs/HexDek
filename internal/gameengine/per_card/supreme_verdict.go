package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSupremeVerdict wires Supreme Verdict.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Supreme%20Verdict):
//
//	This spell can't be countered.
//	Destroy all creatures.
//
// {1}{W}{W}{U} Sorcery. The Wrath of God you can't counter. Premium
// Bant / Esper / 4c control finisher when the table has 4+ untapped
// blue mana — Wrath, Damnation, Day of Judgment all eat Force of Will
// at the wrong moment; Supreme Verdict glides through. The {U}
// splash is the only thing keeping it out of mono-white shells.
//
// Implementation:
//   - OnCast: stamp CostMeta["cannot_be_countered"] = true (mirrors
//     Dovin's Veto / Lier shape) so spellCannotBeCountered returns
//     true when any opp tries to counter the resolving Verdict.
//   - OnResolve: destroyAllCreatures helper from board_wipes.go —
//     same body as Wrath of God / Damnation, just gated by the
//     uncounterable flag stamped at cast time.
func registerSupremeVerdict(r *Registry) {
	r.OnCast("Supreme Verdict", supremeVerdictCast)
	r.OnResolve("Supreme Verdict", supremeVerdictResolve)
}

func supremeVerdictCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	if item == nil {
		return
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["cannot_be_countered"] = true
}

func supremeVerdictResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "supreme_verdict"
	if gs == nil || item == nil {
		return
	}
	destroyed := destroyAllCreatures(gs)
	emit(gs, slug, "Supreme Verdict", map[string]interface{}{
		"seat":      item.Controller,
		"destroyed": destroyed,
	})
}
