package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGargosViciousWatcher wires Gargos, Vicious Watcher.
//
// Oracle text:
//
//	Vigilance
//	Hydra spells you cast cost {4} less to cast.
//	Whenever a creature you control becomes the target of a spell,
//	Gargos fights up to one target creature you don't control.
//
// Implementation (R53 batch N port):
//   - Vigilance: AST keyword pipeline.
//   - Hydra cost reduction: deferred (engine-side cost-modifier work).
//   - Target-fight trigger on "creature_targeted": gate on the
//     target being a creature controlled by Gargos's controller
//     and the spell being cast by an opponent. Pick the
//     lowest-toughness opposing creature as the fight target and
//     apply mutual damage via MarkedDamage so SBA resolves deaths.
func registerGargosViciousWatcher(r *Registry) {
	r.OnTrigger("Gargos, Vicious Watcher", "creature_targeted", gargosFight)
}

func gargosFight(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gargos_targeted_fight"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		return
	}
	if target.Controller != perm.Controller || !target.IsCreature() {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat == perm.Controller {
		return
	}
	var pick *gameengine.Permanent
	bestT := 1 << 30
	for i, s := range gs.Seats {
		if s == nil || i == perm.Controller {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if p.Toughness() < bestT {
				bestT = p.Toughness()
				pick = p
			}
		}
	}
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"reason": "no_opposing_creature",
		})
		return
	}
	if perm.Power() > 0 {
		pick.MarkedDamage += perm.Power()
	}
	if pick.Power() > 0 {
		perm.MarkedDamage += pick.Power()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"gargos_pt":   [2]int{perm.Power(), perm.Toughness()},
		"opponent":    pick.Card.DisplayName(),
		"opponent_pt": [2]int{pick.Power(), pick.Toughness()},
	})
}
