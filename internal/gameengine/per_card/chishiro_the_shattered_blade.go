package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerChishiroTheShatteredBlade wires Chishiro, the Shattered Blade.
//
// Oracle text:
//
//	Whenever an Aura or Equipment you control enters, create a 2/2 red
//	Spirit creature token with menace.
//	At the beginning of your end step, put a +1/+1 counter on each
//	modified creature you control. (Equipment, Auras you control, and
//	counters are modifications.)
func registerChishiroTheShatteredBlade(r *Registry) {
	r.OnTrigger("Chishiro, the Shattered Blade", "permanent_etb", chishiroAuraEquipETB)
	r.OnTrigger("Chishiro, the Shattered Blade", "end_step", chishiroEndStep)
}

func chishiroAuraEquipETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "chishiro_aura_equipment_etb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered.Card == nil {
		return
	}
	if !cardHasType(entered.Card, "aura") && !cardHasType(entered.Card, "equipment") {
		return
	}
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Spirit Token",
		[]string{"creature", "spirit", "pip:R"}, 2, 2)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		gameengine.SetKeywordFlags(gs, tok, "menace")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"trigger": entered.Card.DisplayName(),
	})
}

func chishiroEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "chishiro_end_step_modified_counters"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		modified := false
		if len(p.Counters) > 0 {
			modified = true
		}
		// Naive: any attached aura or equipment? — engine usually flags
		// via .Attached / .EquippedTo.
		if !modified {
			for _, q := range seat.Battlefield {
				if q == nil || q == p || q.Card == nil {
					continue
				}
				if q.AttachedTo == p && (cardHasType(q.Card, "aura") || cardHasType(q.Card, "equipment")) {
					modified = true
					break
				}
			}
		}
		if modified {
			p.AddCounter("+1/+1", 1)
			count++
		}
	}
	if count > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"modified": count,
	})
}
