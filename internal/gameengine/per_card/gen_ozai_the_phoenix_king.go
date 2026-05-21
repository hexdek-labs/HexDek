package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOzaiThePhoenixKing wires Ozai, the Phoenix King.
//
// Oracle text:
//
//	Trample, firebending 4, haste
//	If you would lose unspent mana, that mana becomes red instead.
//	Ozai has flying and indestructible as long as you have six or more
//	unspent mana.
//
// R49 stub-batch-E + R50 batch F merged port:
//
//	The original implementation only re-evaluated the ≥6-mana condition
//	on the controller's upkeep_controller event, which left the
//	keyword-grant stale through opponent turns and inside a single
//	turn after spending mana. This port broadens the refresh:
//	  - ETB: snapshot mana pool, stamp/unstamp Flags accordingly.
//	  - upkeep / combat_begin / end_step: every phase boundary,
//	    re-evaluate so a mid-turn mana spend cleans up before the
//	    next SBA check and so combat math sees the right state.
//
//	The Flags fast-path is what IsIndestructible() (state.go) and the
//	combat flying check read, so the per-condition gate is faithful as
//	long as the refresh ticks cover the windows where mana might be
//	spent.
//
//	"Unspent mana becomes red instead" remains breadcrumbed — needs an
//	engine-side ManaEmpty replacement hook to land properly.
func registerOzaiThePhoenixKing(r *Registry) {
	r.OnETB("Ozai, the Phoenix King", ozaiETBSetFlagsAndConditionalKW)
	r.OnTrigger("Ozai, the Phoenix King", "upkeep", ozaiPhaseRecheck)
	r.OnTrigger("Ozai, the Phoenix King", "combat_begin", ozaiPhaseRecheck)
	r.OnTrigger("Ozai, the Phoenix King", "end_step", ozaiPhaseRecheck)
}

func ozaiETBSetFlagsAndConditionalKW(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ozai_phoenix_king_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["ozai_unspent_mana_to_red"] = 1
	ozaiApplyConditional(gs, perm, seat)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"mana_pool": seat.ManaPool,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"unspent-mana-to-red replacement needs ManaEmpty hook; flag set for downstream consumers")
}

func ozaiPhaseRecheck(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	ozaiApplyConditional(gs, perm, seat)
}

func ozaiApplyConditional(gs *gameengine.GameState, perm *gameengine.Permanent, seat *gameengine.Seat) {
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if seat.ManaPool >= 6 {
		perm.Flags["kw:flying"] = 1
		perm.Flags["kw:indestructible"] = 1
	} else {
		delete(perm.Flags, "kw:flying")
		delete(perm.Flags, "kw:indestructible")
	}
	gs.InvalidateCharacteristicsCache()
}
