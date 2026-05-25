package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAyulaQueenAmongBears wires Ayula, Queen Among Bears.
//
// Oracle text:
//
//	Whenever another Bear you control enters, choose one —
//	• Put two +1/+1 counters on target Bear.
//	• Target Bear you control fights target creature you don't control.
//
// Heuristic (R60): pick fight mode when the entering Bear can kill an
// opposing creature WITHOUT dying itself (strictly favorable removal —
// entering.Power >= opp.Toughness AND entering.Toughness > opp.Power);
// among qualifying opponents, prefer the highest-power target (biggest
// threat off the board). Otherwise default to counters mode and grow
// the entering Bear with two +1/+1 counters (the original no-risk
// default). Fight resolution mirrors Gargos: mutual MarkedDamage so SBA
// settles deaths next pass.
func registerAyulaQueenAmongBears(r *Registry) {
	r.OnTrigger("Ayula, Queen Among Bears", "permanent_etb", ayulaBearETB)
}

func ayulaBearETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered == perm || entered.Card == nil {
		return
	}
	if !cardHasSubtype(entered.Card, "bear") {
		return
	}

	if victim := ayulaPickFightVictim(gs, perm.Controller, entered); victim != nil {
		ayulaFight(gs, perm, entered, victim)
		return
	}
	entered.AddCounter("+1/+1", 2)
	gs.InvalidateCharacteristicsCache()
	emit(gs, "ayula_bear_etb_counters", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": entered.Card.DisplayName(),
	})
}

// ayulaPickFightVictim returns the best opponent creature for entering
// to fight: kills the victim without dying, prefer highest-power victim
// (largest threat removed). Returns nil when no strictly-favorable
// trade exists — caller falls back to counters mode.
func ayulaPickFightVictim(gs *gameengine.GameState, ownerSeat int, entering *gameengine.Permanent) *gameengine.Permanent {
	if entering == nil {
		return nil
	}
	enPow, enTough := entering.Power(), entering.Toughness()
	if enPow <= 0 || enTough <= 0 {
		return nil
	}
	var best *gameengine.Permanent
	bestPower := -1
	for i, s := range gs.Seats {
		if s == nil || i == ownerSeat {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			oppTough := p.Toughness()
			oppPow := p.Power()
			if enPow < oppTough {
				continue // entering can't kill the opp
			}
			if enTough <= oppPow {
				continue // entering would die (or trade) — not strictly favorable
			}
			if oppPow > bestPower {
				best = p
				bestPower = oppPow
			}
		}
	}
	return best
}

func ayulaFight(gs *gameengine.GameState, ayula, bear, victim *gameengine.Permanent) {
	const slug = "ayula_bear_etb_fight"
	if bear.Power() > 0 {
		victim.MarkedDamage += bear.Power()
	}
	if victim.Power() > 0 {
		bear.MarkedDamage += victim.Power()
	}
	emit(gs, slug, ayula.Card.DisplayName(), map[string]interface{}{
		"seat":      ayula.Controller,
		"bear":      bear.Card.DisplayName(),
		"bear_pt":   [2]int{bear.Power(), bear.Toughness()},
		"victim":    victim.Card.DisplayName(),
		"victim_pt": [2]int{victim.Power(), victim.Toughness()},
	})
}
