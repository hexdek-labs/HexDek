package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Reyhan, Last of the Abzan — {B}{G}{W} 0/0 Legendary Creature — Human
// Warrior.
//
//   Reyhan enters with three +1/+1 counters on it.
//   Whenever a creature you control dies or is put into the command
//   zone, if it had one or more +1/+1 counters on it, you may put that
//   many +1/+1 counters on target creature.
//   Partner
//
// Target heuristic: pick the controlled creature with the highest
// existing +1/+1 count (concentrate the stack); fall back to Reyhan
// herself if no other valid target. The "put into the command zone"
// branch is engine commander-replacement territory.

func init() {
	registerReyhanLastOfTheAbzan(Global())
	AddResetHook(registerReyhanLastOfTheAbzan)
}

func registerReyhanLastOfTheAbzan(r *Registry) {
	r.OnETB("Reyhan, Last of the Abzan", reyhanETB)
	r.OnTrigger("Reyhan, Last of the Abzan", "creature_dies", reyhanCreatureDies)
}

func reyhanETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "reyhan_etb_three_counters"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	perm.AddCounter("+1/+1", 3)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counters": perm.Counters["+1/+1"],
	})
}

func reyhanCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "reyhan_transfer_counters"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying == nil || dying == perm || dying.Card == nil {
		return
	}
	if dying.Controller != perm.Controller {
		return
	}
	if !dying.IsCreature() {
		return
	}
	n := 0
	if dying.Counters != nil {
		n = dying.Counters["+1/+1"]
	}
	if n <= 0 {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	var bestOther *gameengine.Permanent
	bestCount := -1
	for _, p := range s.Battlefield {
		if p == nil || p == perm || p == dying || p.Card == nil || !p.IsCreature() {
			continue
		}
		c := 0
		if p.Counters != nil {
			c = p.Counters["+1/+1"]
		}
		if c > bestCount {
			bestCount = c
			bestOther = p
		}
	}
	target := bestOther
	if target == nil {
		target = perm
	}
	target.AddCounter("+1/+1", n)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"trigger_cause": dying.Card.DisplayName(),
		"counters_moved": n,
		"target":         target.Card.DisplayName(),
	})
}
