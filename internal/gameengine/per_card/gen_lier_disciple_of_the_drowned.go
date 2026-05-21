package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLierDiscipleOfTheDrowned wires Lier, Disciple of the Drowned.
//
// Oracle text:
//
//	Spells you control can't be countered.
//	Each instant and sorcery card in your graveyard has flashback. The
//	flashback cost is equal to that card's mana cost.
//
// R49 stub-batch-E port (defensive utility — counter denial):
//   - "Spells can't be countered" — wire OnTrigger("spell_cast") gated
//     on caster_seat == Lier's controller. Locate the matching
//     StackItem (the just-pushed top-of-stack with ev["card"] as the
//     Card pointer) and stamp CostMeta["cannot_be_countered"]=true.
//     counter_resolve.go's spellCannotBeCountered then refuses to
//     resolve a counter-target effect against it. Mirrors Thrun's
//     self-uncounterable OnCast surface, broadened to every spell the
//     controller casts while Lier is on the battlefield.
//   - Flashback grant on every instant/sorcery in graveyard:
//     engine-deep (cost-replacement at flashback-cast time + zone
//     legality). Kept as a partial breadcrumb — AST keyword pipeline
//     owns this surface.
func registerLierDiscipleOfTheDrowned(r *Registry) {
	r.OnTrigger("Lier, Disciple of the Drowned", "spell_cast", lierStampUncounterable)
	r.OnETB("Lier, Disciple of the Drowned", lierETBFlashbackBreadcrumb)
}

func lierStampUncounterable(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lier_disciple_uncounterable_stamp"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	castCard, _ := ctx["card"].(*gameengine.Card)
	if castCard == nil {
		return
	}
	// Find the matching StackItem (just pushed by PushStackItem; spell_cast
	// fires AFTER push, so the item is on the stack now).
	var item *gameengine.StackItem
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil {
			continue
		}
		if si.Card == castCard && si.Controller == casterSeat {
			item = si
			break
		}
	}
	if item == nil {
		return
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["cannot_be_countered"] = true
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        casterSeat,
		"spell":       castCard.DisplayName(),
		"stack_idx":   len(gs.Stack) - 1,
	})
}

func lierETBFlashbackBreadcrumb(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, "lier_disciple_flashback_grant", perm.Card.DisplayName(),
		"flashback grant on instants/sorceries in graveyard handled by AST keyword pipeline; no per_card surface")
}
