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
// Implementation:
//   - Trample/haste/firebending are AST keyword pipeline.
//   - "Unspent mana becomes red instead": engine-deep ManaEmpty hook;
//     set per-seat flag, partial breadcrumb.
//   - Conditional flying + indestructible: scan controller's mana pool
//     on ETB and on every upkeep_controller (the cheapest "tick" we
//     have). Stamp/unstamp the keyword flags accordingly.
func registerOzaiThePhoenixKing(r *Registry) {
	r.OnETB("Ozai, the Phoenix King", ozaiETBSetFlagsAndConditionalKW)
	r.OnTrigger("Ozai, the Phoenix King", "upkeep_controller", ozaiRecheckConditionalKW)
	// R50 batch F: re-evaluate the conditional flying/indestructible on
	// combat_begin so a mid-turn mana spend or float lands the right
	// state for combat. Without this, only upkeep_controller ticked
	// the flags, meaning a player who spent mana to ≥6 unspent during
	// main phase 1 wouldn't see flying/indestructible until next
	// upkeep. End_step refresh keeps a stale grant from leaking past
	// the turn.
	r.OnTrigger("Ozai, the Phoenix King", "combat_begin", ozaiRecheckAny)
	r.OnTrigger("Ozai, the Phoenix King", "end_step", ozaiRecheckAny)
}

func ozaiRecheckAny(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	ozaiApplyConditional(gs, perm, seat)
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

func ozaiRecheckConditionalKW(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	active, _ := ctx["active_seat"].(int)
	if active != perm.Controller {
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
}
