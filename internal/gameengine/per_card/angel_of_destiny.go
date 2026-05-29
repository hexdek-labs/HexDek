package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAngelOfDestiny wires Angel of Destiny (Muninn parser-gap #50,
// 15,465 hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{3}{W}{W}
//	Creature — Angel Cleric
//	Flying, double strike
//	Whenever a creature you control deals combat damage to a player,
//	you and that player each gain that much life.
//	At the beginning of your end step, if you have at least 15 life
//	more than your starting life total, each player this creature
//	attacked this turn loses the game.
//
// Implementation:
//   - Flying and double strike are AST keywords (engine-side).
//   - Symmetric life-gain trigger: OnTrigger("combat_damage_to_player").
//     Gate on damager being a creature controlled by perm.Controller.
//     GainLife on both Angel's controller and the damaged player.
//   - "Attacked this turn" tracking: we stamp perm.Flags["attacked_seat_<n>"]
//     = turn-stamp at the attacks trigger so we can recover the set at
//     end step.
//   - End-step win check: gates on active_seat == controller. If
//     controller's life ≥ starting + 15, every player Angel attacked
//     this turn loses the game (LossReason on each, then trigger SBAs).
//     Starting life defaults to 40 (Commander) but we read seat.Turn or
//     gs.StartingLife if available, else fall back to 40.
//   - Multi-player loss: emitWin only if the only surviving non-Angel
//     seats are eliminated. We use seat.Lost + LossReason to mark each
//     attacked player; engine SBAs handle the rest.
func registerAngelOfDestiny(r *Registry) {
	r.OnTrigger("Angel of Destiny", "combat_damage_to_player", angelOfDestinyDamage)
	// "attacks" aliases to "creature_attacks" — register only one to
	// avoid double-fire.
	r.OnTrigger("Angel of Destiny", "creature_attacks", angelOfDestinyAttacks)
	r.OnTrigger("Angel of Destiny", "end_step", angelOfDestinyEndStep)
}

func angelOfDestinyDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "angel_of_destiny_lifegain"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	// ctx fields: damager_seat (int), target_seat (int), amount (int).
	dmgSeat, _ := ctx["damager_seat"].(int)
	tgtSeat, _ := ctx["target_seat"].(int)
	amount, _ := ctx["amount"].(int)
	if amount <= 0 || dmgSeat != perm.Controller {
		return
	}
	// Filter to creature damagers controlled by us. ctx may carry
	// damager_perm; if so, require IsCreature. If absent, trust the
	// seat check (the engine only fires this for creature combat damage).
	if dp, ok := ctx["damager_perm"].(*gameengine.Permanent); ok && dp != nil {
		if !dp.IsCreature() {
			return
		}
	}
	if tgtSeat < 0 || tgtSeat >= len(gs.Seats) || gs.Seats[tgtSeat] == nil {
		return
	}
	gameengine.GainLife(gs, perm.Controller, amount, perm.Card.DisplayName())
	gameengine.GainLife(gs, tgtSeat, amount, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": tgtSeat,
		"life":   amount,
	})
}

func angelOfDestinyAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	dseat, ok := ctx["defender_seat"].(int)
	if !ok || dseat < 0 || dseat >= len(gs.Seats) {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Stamp turn+1 so 0-turn doesn't collide with zero-value; key is
	// per-defender so we can reconstruct the set at end step.
	perm.Flags["angel_attacked_seat_"+itoa(dseat)] = gs.Turn + 1
}

func angelOfDestinyEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "angel_of_destiny_end_step_win"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	// Commander default; Seat.StartingLife carries the per-seat opening
	// total when non-default (Commander = 40, normal = 20).
	starting := seat.StartingLife
	if starting <= 0 {
		starting = 40
	}
	if seat.Life < starting+15 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"life":      seat.Life,
			"threshold": starting + 15,
			"triggered": false,
		})
		// Clear stamps so a future turn's attacks don't accidentally
		// carry forward. The trigger only checks "this turn", so we
		// zero out attacked-this-turn keys whose stamp != gs.Turn+1.
		angelOfDestinyClearStaleStamps(perm, gs.Turn+1)
		return
	}

	losers := []int{}
	for i := range gs.Seats {
		k := "angel_attacked_seat_" + itoa(i)
		if perm.Flags == nil {
			break
		}
		if perm.Flags[k] != gs.Turn+1 {
			continue
		}
		s := gs.Seats[i]
		if s == nil || s.Lost {
			continue
		}
		// CR §104.3e: route through canonical helper so §614
		// would_lose_game (Platinum Angel / Angel's Grace) can cancel.
		if gameengine.MarkSeatLostByEffect(gs, i, "Angel of Destiny") {
			losers = append(losers, i)
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"life":      seat.Life,
		"threshold": starting + 15,
		"losers":    losers,
	})
	angelOfDestinyClearStaleStamps(perm, gs.Turn+1)

	// If every non-Angel seat is now Lost, this is effectively a win for
	// the controller — emit a canonical win event and CheckEnd.
	allDead := true
	for i, s := range gs.Seats {
		if i == perm.Controller {
			continue
		}
		if s != nil && !s.Lost {
			allDead = false
			break
		}
	}
	if allDead && len(losers) > 0 {
		emitWin(gs, perm.Controller, slug, perm.Card.DisplayName(),
			"angel_of_destiny_alt_win")
	}
}

func angelOfDestinyClearStaleStamps(perm *gameengine.Permanent, currentStamp int) {
	if perm == nil || perm.Flags == nil {
		return
	}
	for k, v := range perm.Flags {
		if len(k) >= 20 && k[:20] == "angel_attacked_seat_" && v != currentStamp {
			delete(perm.Flags, k)
		}
	}
}

