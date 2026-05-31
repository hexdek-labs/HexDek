package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Marisi, Breaker of the Coil — {1}{R}{G}{W} 3/3 Legendary Creature —
// Cat Warrior.
//
//   Your opponents can't cast spells during combat.
//   Whenever a creature you control deals combat damage to a player,
//   goad each creature that player controls. (Until your next turn,
//   those creatures attack each combat if able and attack a player
//   other than you if able.)
//
// Implementation:
//   - OnETB: stamp gs.Flags["marisi_no_combat_cast_seat_N"] = perm.
//     Timestamp for the cast-priority pipeline to consume (latent-flag
//     pattern mirrors necropotence / damia_sage_of_stone). LTB clears.
//   - OnTrigger combat_damage_player: filter source_seat ==
//     Marisi.Controller; iterate defender's battlefield and set
//     p.Flags["goaded"] = 1 on each creature (Nelly Borca pattern at
//     nelly_borca_impulsive_accuser.go:64). The "until your next turn"
//     cleanup window is engine territory — emitPartial flags it.

func init() {
	registerMarisiBreakerOfTheCoil(Global())
	AddResetHook(registerMarisiBreakerOfTheCoil)
}

func registerMarisiBreakerOfTheCoil(r *Registry) {
	r.OnETB("Marisi, Breaker of the Coil", marisiETB)
	r.OnTrigger("Marisi, Breaker of the Coil", "permanent_ltb", marisiLTB)
	r.OnTrigger("Marisi, Breaker of the Coil", "combat_damage_player", marisiCombatDamage)
}

func marisiETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "marisi_no_combat_cast_flag"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["marisi_no_combat_cast_seat_"+intToStr(perm.Controller)] = perm.Timestamp
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"no_combat_cast":    "opponents",
	})
}

func marisiLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "marisi_ltb_clear_flag"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	if gs.Flags != nil {
		delete(gs.Flags, "marisi_no_combat_cast_seat_"+intToStr(perm.Controller))
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func marisiCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "marisi_goad_defender_creatures"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat == perm.Controller {
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	defender := gs.Seats[defenderSeat]
	if defender == nil || defender.Lost {
		return
	}
	goaded := 0
	for _, p := range defender.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["goaded"] = 1
		goaded++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"goaded_count":  goaded,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"goad_until_your_next_turn_cleanup_window_not_enforced")
}
