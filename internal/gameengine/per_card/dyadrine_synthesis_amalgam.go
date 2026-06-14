package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDyadrineSynthesisAmalgam wires Dyadrine, Synthesis Amalgam.
//
// Oracle text:
//
//	Trample
//	Dyadrine enters with a number of +1/+1 counters on it equal to the
//	amount of mana spent to cast it.
//	Whenever you attack, you may remove a +1/+1 counter from each of
//	two creatures you control. If you do, draw a card and create a 2/2
//	colorless Robot artifact creature token.
//
// ETB X-counters and removal-driven attack trigger.
func registerDyadrineSynthesisAmalgam(r *Registry) {
	r.OnETB("Dyadrine, Synthesis Amalgam", dyadrineETB)
	r.OnTrigger("Dyadrine, Synthesis Amalgam", "declare_attackers", dyadrineAttack)
}

func dyadrineETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "dyadrine_etb_counters"
	if gs == nil || perm == nil {
		return
	}
	x := 0
	if perm.Card != nil {
		x = perm.Card.CMC
		if x <= 0 {
			x = gameengine.ManaCostOf(perm.Card)
		}
	}
	if x > 0 {
		perm.AddCounter("+1/+1", x)
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counters": x,
	})
}

func dyadrineAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "dyadrine_attack_robot"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	attackerSeat, _ := ctx["seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}
	// "Whenever you attack" fires once per declare-attackers step, but the
	// engine fires "declare_attackers" once per declared attacker. Gate to
	// the first hit per turn so the counter-removal/token resolves once per
	// combat, not once per attacker (Caesar/Raffine once-per-turn dedup).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["dyadrine_attack_fired"] >= gs.Turn {
		return
	}
	perm.Flags["dyadrine_attack_fired"] = gs.Turn
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Find two creatures with +1/+1 counters; prefer non-Dyadrine.
	var picks []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Counters == nil || p.Counters["+1/+1"] <= 0 {
			continue
		}
		picks = append(picks, p)
		if len(picks) == 2 {
			break
		}
	}
	if len(picks) < 2 {
		return
	}
	for _, p := range picks {
		p.AddCounter("+1/+1", -1)
		if p.Counters["+1/+1"] <= 0 {
			delete(p.Counters, "+1/+1")
		}
	}
	gs.InvalidateCharacteristicsCache()
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	gameengine.CreateCreatureToken(gs, perm.Controller, "Robot Token",
		[]string{"artifact", "creature", "robot"}, 2, 2)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"removed": 2,
	})
}
