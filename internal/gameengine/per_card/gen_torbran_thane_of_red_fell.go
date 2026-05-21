package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTorbranThaneOfRedFell wires Torbran, Thane of Red Fell.
//
// Oracle text:
//
//	If a red source you control would deal damage to an opponent or a
//	permanent an opponent controls, it deals that much damage plus 2
//	instead.
//
// Implementation: a damage replacement is engine-deep (it has to wrap
// every DealDamage call from a red source controlled by Torbran's
// controller). Until that hook lands, set per-seat flags the engine
// can read and emit the partial breadcrumb so the audit catches the gap.
// The flag value carries the controller seat (+1 so default 0 = off)
// AND the damage bonus, so multiple Torbrans stack additively.
//
// R49 batch C add: LTB hook decrements the bonus stack so the flag
// goes inert when Torbran leaves play. Without this the +2 stuck on
// the game state forever once Torbran ever entered, and a second
// Torbran would leave +4 stuck after the first one bounced.
func registerTorbranThaneOfRedFell(r *Registry) {
	r.OnETB("Torbran, Thane of Red Fell", torbranETBSetDamageFlag)
	r.OnTrigger("Torbran, Thane of Red Fell", "permanent_ltb", torbranLTBClearDamageFlag)
}

func torbranLTBClearDamageFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	if gs.Flags == nil {
		return
	}
	if gs.Flags["torbran_red_damage_bonus"] >= 2 {
		gs.Flags["torbran_red_damage_bonus"] -= 2
	}
	if gs.Flags["torbran_red_damage_bonus"] <= 0 {
		delete(gs.Flags, "torbran_red_damage_bonus")
		delete(gs.Flags, "torbran_red_damage_seat")
	}
}

func torbranETBSetDamageFlag(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "torbran_red_damage_plus_two"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	// Encode controller seat (+1 so 0 means inactive) and the damage
	// bonus stack count. A second Torbran adds another +2 — they stack
	// per the rules cleanup ruling on multiple replacement effects.
	gs.Flags["torbran_red_damage_seat"] = perm.Controller + 1
	gs.Flags["torbran_red_damage_bonus"] += 2
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"bonus": gs.Flags["torbran_red_damage_bonus"],
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"red-source-damage replacement needs DealDamage hook to apply +2; flag set for downstream consumers")
}
