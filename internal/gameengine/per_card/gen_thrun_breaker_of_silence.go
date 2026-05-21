package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerThrunBreakerOfSilence wires Thrun, Breaker of Silence.
//
// Oracle text:
//
//	This spell can't be countered.
//	Trample
//	Thrun can't be the target of nongreen spells your opponents
//	control or abilities from nongreen sources your opponents control.
//	During your turn, Thrun has indestructible.
//
// R49 stub-batch-E port (defensive utility — conditional self-protection):
//
//	The R37 implementation stamped the cast-uncounterable flag on the
//	StackItem (still preserved) but the active-turn indestructible
//	half lived as an emitPartial breadcrumb. This port replaces the
//	breadcrumb with an upkeep-driven Flags toggle that keeps
//	Flags["kw:indestructible"] in sync with gs.Active:
//	  - ETB: stamp the flag if entering on Thrun's own controller's turn.
//	  - upkeep trigger: re-evaluate at the start of every turn — set
//	    the flag when active_seat == controller, clear it otherwise.
//
//	The Flags fast-path is what IsIndestructible() reads in state.go,
//	so Thrun-self SBA destruction is correctly gated on whose turn it
//	is. The trade-off versus a layer-6 continuous effect is per-turn
//	granularity (vs. instant predicate re-eval), but since the
//	condition only flips on turn boundaries, the upkeep tick is
//	sufficient.
//
//	Pattern mirrors Ozai's conditional flying+indestructible refresh
//	(gen_ozai_the_phoenix_king.go), broadened to listen on every
//	upkeep rather than only the controller's upkeep.
//
//	Trample / cast-uncounterable kept as-is. Target-protection vs
//	nongreen opponents stays partial — engine §702.16 protection-from
//	machinery is what should own that surface; we'd just shadow it.
func registerThrunBreakerOfSilence(r *Registry) {
	r.OnCast("Thrun, Breaker of Silence", thrunCastUncounterable)
	r.OnETB("Thrun, Breaker of Silence", thrunETBStampIndestructible)
	r.OnTrigger("Thrun, Breaker of Silence", "upkeep", thrunUpkeepRefreshIndestructible)
}

func thrunCastUncounterable(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "thrun_cant_be_countered"
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

func thrunETBStampIndestructible(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "thrun_active_turn_indestructible_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	thrunSyncIndestructible(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"active": gs.Active,
	})
	emitPartial(gs, "thrun_target_protection_nongreen_opponents", perm.Card.DisplayName(),
		"target-protection vs nongreen opponent spells/abilities — §702.16 protection-from machinery owns this surface")
}

func thrunUpkeepRefreshIndestructible(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	thrunSyncIndestructible(gs, perm)
}

func thrunSyncIndestructible(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if gs.Active == perm.Controller {
		perm.Flags["kw:indestructible"] = 1
	} else {
		delete(perm.Flags, "kw:indestructible")
	}
	gs.InvalidateCharacteristicsCache()
}
