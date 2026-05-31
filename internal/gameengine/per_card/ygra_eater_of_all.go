package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYgraEaterOfAll wires Ygra, Eater of All.
//
// Oracle text (Scryfall, verified 2026-05-30, {1}{B}{G}, 4/4 Legendary
// Creature — Cat Beast):
//
//	Ward—Sacrifice a Food.
//	Other creatures are Food artifacts in addition to their other types
//	and have "{2}, {T}, Sacrifice this permanent: You gain 3 life."
//	Whenever a Food is put into a graveyard from the battlefield, put two
//	+1/+1 counters on Ygra.
//
// Implementation (R60 stub sweep batch 2):
//   - "permanent_ltb" trigger: if the leaving permanent's destination is
//     graveyard and the dying card is a Food OR is a non-Ygra creature
//     (per "other creatures are Food in addition to their other types"
//     — the §613 type-add is active while Ygra is on the battlefield,
//     so any other creature dying counts as a Food dying), put 2 +1/+1
//     counters on Ygra. Self-LTB is excluded because the static would
//     have already ceased when Ygra started leaving (§613 timestamp
//     ordering — Ygra's own static doesn't apply to itself per the
//     "other" qualifier anyway).
//   - The granted activated mana-sink "{2}, {T}, Sac: gain 3 life" on
//     each other creature is a cost-pipeline AST grant the per_card
//     layer can't express; emitPartial flags the gap on ETB.
func registerYgraEaterOfAll(r *Registry) {
	r.OnETB("Ygra, Eater of All", ygraETBPartial)
	r.OnTrigger("Ygra, Eater of All", "permanent_ltb", ygraPermLeftBattlefield)
}

func ygraETBPartial(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ygra_etb_granted_ability_partial"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"granted_two_tap_sac_for_3_life_activated_cost_pipeline_unwired")
}

func ygraPermLeftBattlefield(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ygra_food_ltb_counters"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	dest, _ := ctx["to_zone"].(string)
	if dest != "graveyard" {
		return
	}
	leaver, _ := ctx["card"].(*gameengine.Card)
	if leaver == nil {
		return
	}
	// Skip self-LTB: "other creatures" qualifier and Ygra-self double-
	// counting would compound on the dying permanent before SBA cleanup.
	if leaver == perm.Card {
		return
	}
	isFood := false
	for _, t := range leaver.Types {
		if t == "food" || t == "Food" {
			isFood = true
			break
		}
	}
	// "Other creatures are Food" §613 type-add: any other creature dying
	// is a Food dying for trigger purposes. Honor the "other" qualifier.
	if !isFood && !cardHasType(leaver, "creature") {
		return
	}
	perm.AddCounter("+1/+1", 2)
	gs.InvalidateCharacteristicsCache()
	via := "creature_as_food_static"
	if isFood {
		via = "food_native"
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counters": 2,
		"via":      via,
	})
}
