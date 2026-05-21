package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAminatouVeilPiercer wires Aminatou, Veil Piercer.
//
// Oracle text (Duskmourn Commander, {1}{W}{U}{B}, 2/4):
//
//	At the beginning of your upkeep, surveil 2.
//	Each enchantment card in your hand has miracle. Its miracle cost is
//	equal to its mana cost reduced by {4}.
//
// Implementation:
//   - "upkeep_controller" trigger: surveil 2 for the active player when
//     it's also Aminatou's controller (the trigger says "your upkeep").
//   - The miracle-grant static is left to the AST engine — granting
//     miracle to enchantments in hand needs cast-time wiring beyond a
//     per_card ETB hook. emitPartial flags the gap.
func registerAminatouVeilPiercer(r *Registry) {
	r.OnETB("Aminatou, Veil Piercer", aminatouVeilPiercerETB)
	r.OnTrigger("Aminatou, Veil Piercer", "upkeep_controller", aminatouVeilPiercerUpkeep)
}

func aminatouVeilPiercerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "aminatou_veil_piercer_etb"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"enchantment_miracle_grant_in_hand_not_wired_to_cast_path")
}

func aminatouVeilPiercerUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aminatou_veil_piercer_upkeep_surveil"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	gameengine.Surveil(gs, perm.Controller, 2)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"surveil": 2,
	})
}
