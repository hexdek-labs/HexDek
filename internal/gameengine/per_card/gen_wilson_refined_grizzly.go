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
// R49 stub-batch-E + R50 batch F merged port:
//   - OnCast stamps CostMeta["cannot_be_countered"]=true so the
//     counter-spell resolver refuses to counter Wilson on the stack
//     (same shape as Thrun, Breaker of Silence).
//   - ETB stamps the static keyword flags:
//       - ward:2 (read by target dispatcher)
//       - kw:vigilance, kw:reach, kw:trample (combat reads)
//   - Choose-a-Background partner is deck-construction-time only.
func registerWilsonRefinedGrizzly(r *Registry) {
	r.OnCast("Wilson, Refined Grizzly", wilsonRefinedGrizzlyCast)
	r.OnETB("Wilson, Refined Grizzly", wilsonRefinedGrizzlyETB)
}

// wilsonRefinedGrizzlyCastUncounterable is a kept-alive alias for the
// pre-merge function name. Removed in a follow-up sweep.
func wilsonRefinedGrizzlyCastUncounterable(gs *gameengine.GameState, item *gameengine.StackItem) {
	wilsonRefinedGrizzlyCast(gs, item)
}

func wilsonRefinedGrizzlyCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "wilson_refined_grizzly_cant_be_countered"
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
}
