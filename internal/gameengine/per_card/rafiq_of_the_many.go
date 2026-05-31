package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Rafiq of the Many — {1}{G}{W}{U} 3/3 Legendary Creature — Human Knight.
//
//   Exalted (Whenever a creature you control attacks alone, that creature
//   gets +1/+1 until end of turn.)
//   Whenever a creature you control attacks alone, it gains double strike
//   until end of turn.
//
// Implementation: OnTrigger declare_attackers, count attackers on
// controller's battlefield; if exactly 1, grant the lone attacker
// double_strike UEOT (Squall, SeeD Mercenary pattern at
// squall_seed_mercenary.go:30). The +1/+1 from exalted itself is engine
// keyword-pipeline (combat.go:599 handles per-instance exalted triggers).

func init() {
	registerRafiqOfTheMany(Global())
	AddResetHook(registerRafiqOfTheMany)
}

func registerRafiqOfTheMany(r *Registry) {
	r.OnTrigger("Rafiq of the Many", "declare_attackers", rafiqAttacksAlone)
}

func rafiqAttacksAlone(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rafiq_attacks_alone_double_strike"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var lone *gameengine.Permanent
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.IsAttacking() {
			count++
			lone = p
		}
	}
	if count != 1 || lone == nil {
		return
	}
	if lone.Flags == nil {
		lone.Flags = map[string]int{}
	}
	lone.Flags["kw:double_strike"] = 1
	lone.Flags["double_strike_eot"] = 1
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": lone.Card.DisplayName(),
	})
}
