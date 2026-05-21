package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWilsonRefinedGrizzly wires Wilson, Refined Grizzly.
//
// Oracle text:
//
//	This spell can't be countered.
//	Vigilance, reach, trample
//	Ward {2}
//	Choose a Background (You can have a Background as a second commander.)
//
// R49 stub-batch-E port (defensive utility):
//   - "This spell can't be countered" — PORTED via OnCast hook
//     mirroring Thrun. Stamps StackItem.CostMeta["cannot_be_countered"]=true
//     so counter_resolve.go's spellCannotBeCountered refuses to counter it.
//   - Vigilance / reach / trample + ward: kept as runtime flag stamps at
//     ETB. ward already lives on Flags["ward"]; AST keyword pipeline
//     handles the rest in parallel — flags are belt-and-suspenders for
//     test seats that build Cards without AST.
//   - Background partner: deck-construction concern, breadcrumb only.
func registerWilsonRefinedGrizzly(r *Registry) {
	r.OnCast("Wilson, Refined Grizzly", wilsonRefinedGrizzlyCastUncounterable)
	r.OnETB("Wilson, Refined Grizzly", wilsonRefinedGrizzlyETB)
}

func wilsonRefinedGrizzlyCastUncounterable(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "wilson_refined_grizzly_cast_uncounterable"
	if gs == nil || item == nil {
		return
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["cannot_be_countered"] = true
	name := ""
	if item.Card != nil {
		name = item.Card.DisplayName()
	}
	emit(gs, slug, name, map[string]interface{}{
		"seat":          item.Controller,
		"stack_flagged": true,
	})
}

func wilsonRefinedGrizzlyETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "wilson_refined_grizzly_etb"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["ward"] = 2
	perm.Flags["kw:vigilance"] = 1
	perm.Flags["kw:reach"] = 1
	perm.Flags["kw:trample"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"ward":     2,
		"keywords": []string{"vigilance", "reach", "trample"},
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"choose_background_partner_handled_at_deck_construction_time")
}
