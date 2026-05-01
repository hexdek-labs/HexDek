package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKillianInkDuelist wires Killian, Ink Duelist.
//
// Oracle text:
//
//	Lifelink
//	Menace (This creature can't be blocked except by two or more creatures.)
//	Spells you cast that target a creature cost {2} less to cast.
//
// Implementation:
//   - Lifelink + Menace are AST keywords; the engine handles them.
//   - The {2}-less cost reduction lives in cost_modifiers.go's
//     ScanCostModifiers switch (case "Killian, Ink Duelist") so it slots
//     into the standard CR §601.2f cost-calculation pipeline alongside
//     Helm of Awakening, Animar, and the medallions. See
//     spellOracleTargetsCreature in cost_modifiers.go for the targeting
//     heuristic.
//   - This ETB hook only logs that Killian's static is now active so
//     analytics can correlate the cost reduction with on-battlefield
//     presence.
func registerKillianInkDuelist(r *Registry) {
	r.OnETB("Killian, Ink Duelist", killianInkDuelistETB)
}

func killianInkDuelistETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "killian_ink_duelist_static"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"reduction":  2,
		"applies_to": "spells_targeting_creature",
		"wired_in":   "cost_modifiers.go",
	})
}
