package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAdelizTheCinderWind wires Adeliz, the Cinder Wind.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{1}{U}{R}
//	Legendary Creature — Human Wizard
//	Flying, haste
//	Whenever you cast an instant or sorcery spell, Wizards you control
//	get +1/+1 until end of turn.
//
// Implementation:
//   - instant_or_sorcery_cast trigger gated on caster_seat == controller:
//     pump every Wizard the controller controls (+1/+1 until end of
//     turn) through pumpUntilEOT (pt_pump.go), which records the pump as
//     a Permanent.Modification. The older temp_power / temp_toughness
//     flag write is kept as a diagnostic stamp but is not what makes the
//     pump real — nothing in the engine reads it.
func registerAdelizTheCinderWind(r *Registry) {
	r.OnTrigger("Adeliz, the Cinder Wind", "instant_or_sorcery_cast", adelizCastPump)
}

func adelizCastPump(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "adeliz_cinder_wind_wizard_pump"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	pumped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "wizard") {
			continue
		}
		pumpUntilEOT(gs, p, 1, 1)
		pumped++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"wizards_pumped": pumped,
	})
}
