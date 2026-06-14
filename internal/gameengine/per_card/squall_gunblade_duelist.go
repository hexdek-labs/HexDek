package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSquallGunbladeDuelist wires Squall, Gunblade Duelist.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	First strike
//	As Squall enters, choose a number.
//	Whenever one or more creatures attack one of your opponents, if
//	any of those creatures have power or toughness equal to the
//	chosen number, Squall deals damage equal to its power to
//	defending player.
//
// Implementation:
//   - First strike via AST keyword pipeline.
//   - ETB: choose a number — heuristic picks 3 (most common P/T value
//     for early creatures). Stamp into perm.Flags["squall_chosen_number"].
//   - "declare_attackers" trigger: scan attackers; if any have
//     power or toughness equal to the chosen number AND defending
//     player is one of Squall's controller's opponents (it's not
//     literally Squall's controller defending), Squall pings the
//     defending player for Squall.Power().
func registerSquallGunbladeDuelist(r *Registry) {
	r.OnETB("Squall, Gunblade Duelist", squallGunbladeETB)
	r.OnTrigger("Squall, Gunblade Duelist", "declare_attackers", squallGunbladeAttackers)
}

func squallGunbladeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "squall_gunblade_choose_number"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	chosen := 3
	perm.Flags["squall_chosen_number"] = chosen
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"chosen": chosen,
	})
}

func squallGunbladeAttackers(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "squall_gunblade_attackers_check"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat == perm.Controller {
		return
	}
	defender := gs.Seats[defenderSeat]
	if defender == nil || defender.Lost {
		return
	}
	// "Whenever one or more creatures attack one of your opponents" fires
	// once per declare-attackers step per attacked opponent, but the engine
	// fires "declare_attackers" once per declared attacker. Gate to the first
	// hit per (turn, defending player) so Squall pings each attacked opponent
	// once per combat, not once per attacker (Caesar once-per-turn dedup,
	// keyed by defender to preserve the per-opponent semantics).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupe := "squall_fired_d" + strconv.Itoa(defenderSeat)
	if perm.Flags[dedupe] >= gs.Turn {
		return
	}
	perm.Flags[dedupe] = gs.Turn
	chosen := 0
	if perm.Flags != nil {
		chosen = perm.Flags["squall_chosen_number"]
	}
	if chosen <= 0 {
		chosen = 3
	}
	matched := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsAttacking() {
				continue
			}
			da, _ := ctx["defender_seat_for_attacker"].(map[*gameengine.Permanent]int)
			if da != nil {
				if seat, ok := da[p]; ok && seat != defenderSeat {
					continue
				}
			}
			if p.Power() == chosen || p.Toughness() == chosen {
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}
	if !matched {
		return
	}
	dmg := perm.Power()
	gameengine.DealDamage(gs, defenderSeat, dmg, perm.Card.DisplayName())
	_ = gs.CheckEnd()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"defender": defenderSeat,
		"damage":   dmg,
		"chosen":   chosen,
	})
}
