package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheMasterMultiplied wires The Master, Multiplied.
//
// Oracle text (Scryfall, verified):
//
//	Myriad
//	The "legend rule" doesn't apply to creature tokens you control.
//	Triggered abilities you control can't cause you to sacrifice or
//	exile creature tokens you control.
//
// Implementation (R45 stub port):
//   - Myriad: AST keyword pipeline (keywords_combat.go handles
//     attack-time myriad copies onto each opponent).
//   - Legend-rule exception for creature tokens: engine-side via the
//     new "The Master, Multiplied" presence check in
//     gameengine/sba.go sba704_5j. The grouping pass drops the
//     controller's creature tokens before checking for legend-rule
//     duplicates, so myriad-spawned legendary copies of The Master
//     survive past SBA.
//   - "Triggered abilities you control can't cause you to sacrifice
//     or exile creature tokens you control": engine surface — the
//     sac/exile dispatch would need a per-controller filter that
//     skips own creature tokens when the action source is a
//     triggered ability. emitPartial breadcrumb on ETB; the rider is
//     a follow-up wiring change.
func registerTheMasterMultiplied(r *Registry) {
	r.OnETB("The Master, Multiplied", theMasterMultipliedETB)
}

func theMasterMultipliedETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_master_multiplied_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"legend_rule_skip":  true,
		"sba_change_active": true,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"triggered_ability_sacrifice_exile_suppression_rider_not_wired")
}
