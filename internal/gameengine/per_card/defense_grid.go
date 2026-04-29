package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDefenseGrid wires Defense Grid.
//
// Oracle text:
//
//	Each spell costs {3} more to cast except during its controller's turn.
//
// Implementation: stamp a flag on ETB so ScanCostModifiers can apply
// the {3} surtax. The cost modifier scanner already handles
// "cost_increase_all" perm flags, but Defense Grid's condition is
// narrower: it only taxes spells cast OUTSIDE the caster's own turn.
// We use a dedicated gs.Flags key that CalculateTotalCost consults.
//
// Hook point: ScanCostModifiers in cost_modifiers.go recognizes
// "Defense Grid" by name (added as a new case alongside Thalia etc.)
// and checks gs.Active != seatIdx to apply the +3 tax.
func registerDefenseGrid(r *Registry) {
	r.OnETB("Defense Grid", defenseGridETB)
}

func defenseGridETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "defense_grid_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"timestamp": perm.Timestamp,
		"effect":    "spells_cost_3_more_outside_controllers_turn",
	})
}
