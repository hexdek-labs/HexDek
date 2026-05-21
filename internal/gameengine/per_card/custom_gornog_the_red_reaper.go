package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGornogTheRedReaperCustom wires Gornog's tribal payloads.
// The gen_*.go handler is a pure breadcrumb partial. Gornog's printed
// text:
//
//	Haste
//	Cowards can't block Warriors.
//	Whenever one or more Warriors you control attack a player,
//	  target creature that player controls becomes a Coward.
//	Attacking Warriors you control get +X/+0, where X is the number
//	  of Cowards your opponents control.
//
// Implementation:
//   - Attack trigger: pick a creature controlled by the defending
//     player (lowest power so the type-change weakens their best
//     blockers' role — the practical play is to coward-tag a
//     potential blocker so it can't block our Warrior). Stamp
//     "creature_type:coward" on the chosen target.
//   - Anthem: refresh on creature_attacks and combat_begin events.
//     Count opponents' Cowards (creatures tagged coward), then
//     stamp a +X/+0 buff on every ATTACKING Warrior we control.
//     The buff is tagged with the gornog Modification Duration so
//     it tears down cleanly after combat.
//   - Block restriction ("Cowards can't block Warriors"): stamp a
//     seat-level flag the combat-blocker validator can consult.
//     Engine-side block validators aren't surfaced from per_card;
//     the flag is a breadcrumb that the audit can wire up later.
const gornogWarriorAnthemTag = "gornog_warrior_attack_anthem"

func registerGornogTheRedReaperCustom(r *Registry) {
	r.OnETB("Gornog, the Red Reaper", gornogETBSetBlockRestriction)
	r.OnTrigger("Gornog, the Red Reaper", "creature_attacks", gornogOnAttack)
	r.OnTrigger("Gornog, the Red Reaper", "combat_begin", gornogRefreshAnthem)
	r.OnTrigger("Gornog, the Red Reaper", "end_of_combat", gornogClearAnthem)
}

func gornogETBSetBlockRestriction(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "gornog_etb_block_restriction"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["gornog_cowards_cant_block_warriors"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func gornogOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gornog_warrior_attack_coward_tag"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk.Card == nil || atk.Controller != perm.Controller {
		return
	}
	if !cardHasSubtype(atk.Card, "warrior") {
		return
	}
	// Once per turn per defending player — heuristic via turn flag.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnKey := "gornog_attack_fired_turn"
	if perm.Flags[turnKey] == gs.Turn+1 {
		// Already fired this turn — still refresh anthem in case more
		// attackers joined.
		gornogRefreshAnthem(gs, perm, ctx)
		return
	}
	perm.Flags[turnKey] = gs.Turn + 1

	// Default defender = the seat being attacked. ctx may carry the
	// target_seat; fall back to lowest-life opponent.
	defSeat, _ := ctx["defending_seat"].(int)
	if defSeat <= 0 {
		defSeat, _ = ctx["target_seat"].(int)
	}
	if defSeat <= 0 || defSeat == perm.Controller || defSeat >= len(gs.Seats) {
		defSeat = -1
		bestLife := 1 << 30
		for i, s := range gs.Seats {
			if s == nil || s.Lost || i == perm.Controller {
				continue
			}
			if s.Life < bestLife {
				bestLife = s.Life
				defSeat = i
			}
		}
	}
	if defSeat < 0 {
		return
	}
	// Pick the defender's lowest-power non-coward creature — coward-
	// tagging their weakest creature suppresses a potential blocker.
	var tgt *gameengine.Permanent
	tgtPow := 1 << 30
	defS := gs.Seats[defSeat]
	if defS == nil {
		return
	}
	for _, p := range defS.Battlefield {
		if p == nil || !p.IsCreature() || p.Card == nil {
			continue
		}
		if cardHasSubtype(p.Card, "coward") {
			continue
		}
		if pw := gs.PowerOf(p); pw < tgtPow {
			tgtPow = pw
			tgt = p
		}
	}
	if tgt != nil && tgt.Card != nil {
		tgt.Card.Types = append(tgt.Card.Types, "coward")
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"target": tgt.Card.DisplayName(),
		})
	}
	gornogRefreshAnthem(gs, perm, ctx)
}

func gornogRefreshAnthem(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gornog_warrior_anthem_refresh"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	cowards := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == perm.Controller {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if cardHasSubtype(p.Card, "coward") {
				cowards++
			}
		}
	}
	if cowards <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasSubtype(p.Card, "warrior") {
			continue
		}
		if !p.IsAttacking() {
			continue
		}
		stripTaggedModifications(p, gornogWarriorAnthemTag)
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     cowards,
			Toughness: 0,
			Duration:  gornogWarriorAnthemTag,
			Timestamp: gs.NextTimestamp(),
		})
		stamped++
	}
	if stamped > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"stamped": stamped,
			"cowards": cowards,
		})
	}
}

func gornogClearAnthem(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	cleared := 0
	for _, p := range seat.Battlefield {
		if p == nil {
			continue
		}
		before := len(p.Modifications)
		stripTaggedModifications(p, gornogWarriorAnthemTag)
		if len(p.Modifications) < before {
			cleared++
		}
	}
	if cleared > 0 {
		gs.InvalidateCharacteristicsCache()
	}
}
