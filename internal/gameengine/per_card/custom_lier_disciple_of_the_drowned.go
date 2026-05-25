package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLierDiscipleOfTheDrownedCustom marks instants and sorceries
// the controller casts as uncounterable and registers a continuous
// "instants/sorceries in your graveyard have flashback at their mana
// cost" grant on ETB. The auto-generated stub
// registerLierDiscipleOfTheDrowned in gen_lier_disciple_of_the_drowned.go
// remains as a breadcrumb for the uncounterable surface only.
//
// Oracle text (Innistrad: Midnight Hunt, {3}{U}{U}):
//
//	Flash
//	Spells can't be countered.
//	Each instant and sorcery card in your graveyard has flashback. The
//	flashback cost is equal to that card's mana cost.
//
// (TLA reprint / current printing on the Avatar set drops the
// graveyard-exile replacement and the explicit "you may cast from
// graveyard" line — both are now subsumed by the flashback grant.)
//
// Implementation:
//   - OnTrigger("instant_or_sorcery_cast"): stamps
//     CostMeta["cannot_be_countered"] on the stack item when the caster
//     is Lier's controller. Counterspell handlers honor this flag.
//   - OnETB: registers a continuous GraveyardFlashbackGrant with
//     OnlyActiveTurn=false (always-on) using PrintedMassFlashbackCost as
//     the cost predicate (every instant/sorcery in the controller's
//     graveyard gets flashback at its printed mana cost).
//   - OnTrigger("permanent_ltb"): expires the grant when Lier leaves.
//   - Flash is granted by the AST keyword pipeline.
func registerLierDiscipleOfTheDrownedCustom(r *Registry) {
	r.OnTrigger("Lier, Disciple of the Drowned", "instant_or_sorcery_cast", lierUncounterableMark)
	r.OnETB("Lier, Disciple of the Drowned", lierGraveyardFlashbackGrantETB)
	r.OnTrigger("Lier, Disciple of the Drowned", "permanent_ltb", lierGraveyardFlashbackGrantLTB)
}

func lierGraveyardFlashbackGrantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	grant := &gameengine.GraveyardFlashbackGrant{
		Controller:      perm.Controller,
		SourceTimestamp: perm.Timestamp,
		SourceName:      perm.Card.DisplayName(),
		OnlyActiveTurn:  false,
		CostFor:         gameengine.PrintedMassFlashbackCost,
	}
	gameengine.RegisterGraveyardFlashbackGrant(gs, grant)
	emit(gs, "lier_disciple_of_the_drowned_flashback_grant", perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"only_active_turn": false,
	})
}

func lierGraveyardFlashbackGrantLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.ExpireGraveyardFlashbackGrantsBySource(gs, perm.Timestamp)
}

func lierUncounterableMark(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lier_disciple_of_the_drowned_uncounterable_mark"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	item, _ := ctx["stack_item"].(*gameengine.StackItem)
	if item == nil {
		// Try alternate ctx keys some emitters use.
		item, _ = ctx["item"].(*gameengine.StackItem)
	}
	if item != nil {
		if item.CostMeta == nil {
			item.CostMeta = map[string]interface{}{}
		}
		item.CostMeta["cannot_be_countered"] = true
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"marked": true,
		})
	} else {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"stack_item_not_in_trigger_ctx_uncounterable_flag_skipped")
	}
}
