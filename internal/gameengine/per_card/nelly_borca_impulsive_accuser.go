package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNellyBorcaImpulsiveAccuser wires Nelly Borca, Impulsive Accuser.
//
// Oracle text:
//
//	Vigilance
//	Whenever Nelly Borca attacks, suspect target creature. Then goad
//	all suspected creatures.
//	Whenever one or more creatures an opponent controls deal combat
//	damage to one or more of your opponents, you and the controller of
//	those creatures each draw a card.
//
// Suspect/goad continuous flags are stamped on a target creature; the
// shared-draw clause is wired through combat_damage_player.
func registerNellyBorcaImpulsiveAccuser(r *Registry) {
	r.OnTrigger("Nelly Borca, Impulsive Accuser", "attacks", nellySuspect)
	r.OnTrigger("Nelly Borca, Impulsive Accuser", "combat_damage_player", nellySharedDraw)
}

func nellySuspect(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "nelly_suspect_then_goad_all"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 self-gate: "Whenever Nelly Borca attacks" fires only when Nelly
	// itself attacks. creature_attacks is dispatched once per declared attacker,
	// so without this the suspect + goad-all ran once per attacker. See the Aang
	// and Katara fix (5254f9cf).
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	var best *gameengine.Permanent
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if best == nil || p.Power() > best.Power() {
				best = p
			}
		}
	}
	if best == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_target_creature", nil)
		return
	}
	if best.Flags == nil {
		best.Flags = map[string]int{}
	}
	best.Flags["suspected"] = 1
	best.Flags["kw:menace"] = 1
	best.Flags["cant_block"] = 1
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Flags == nil {
				continue
			}
			if p.Flags["suspected"] == 1 {
				p.Flags["goaded"] = 1
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": best.Card.DisplayName(),
	})
}

func nellySharedDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "nelly_shared_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	defenderSeat, _ := ctx["defender_seat"].(int)
	if sourceSeat == perm.Controller {
		return
	}
	if defenderSeat == perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	gateKey := "nelly_drew_turn"
	if perm.Flags[gateKey] == gs.Turn {
		return
	}
	perm.Flags[gateKey] = gs.Turn
	mySeat := gs.Seats[perm.Controller]
	if mySeat != nil && len(mySeat.Library) > 0 {
		c := mySeat.Library[0]
		gameengine.MoveCard(gs, c, perm.Controller, "library", "hand", "draw")
	}
	srcSeat := gs.Seats[sourceSeat]
	if srcSeat != nil && len(srcSeat.Library) > 0 {
		c := srcSeat.Library[0]
		gameengine.MoveCard(gs, c, sourceSeat, "library", "hand", "draw")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"src_seat":  sourceSeat,
		"defender":  defenderSeat,
	})
}
