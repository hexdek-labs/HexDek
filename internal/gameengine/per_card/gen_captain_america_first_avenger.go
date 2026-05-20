package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCaptainAmericaFirstAvenger wires Captain America, First Avenger.
//
// Oracle text (Scryfall, verified 2026-05-09):
//
//	Throw — {3}, Unattach an Equipment from Captain America: He deals
//	damage equal to that Equipment's mana value divided as you choose
//	among one, two, or three targets.
//	Catch — At the beginning of combat on your turn, attach up to one
//	target Equipment you control to Captain America.
//
// Implementation:
//   - OnActivated (Throw): pick the highest-MV Equipment currently
//     attached to Captain America. Unattach it (set AttachedTo = nil),
//     then deal `equip.cmc` damage divided across opponents — concentrated
//     on the lowest-life living opponent for AI lethal pressure when
//     possible, else round-robin'd 1 each across up to three opponents.
//   - "begin_combat_controller" (Catch): scan controller's Equipment.
//     Pick the unattached, highest-MV Equipment and attach it to Cap.
//     If none unattached, pick the highest-MV Equipment whose current
//     host has lower power than Cap (so we don't strip a better target).
//
// The {3} mana cost is engine-enforced before dispatch; we defensively
// drain the pool if it covers the amount.
func registerCaptainAmericaFirstAvenger(r *Registry) {
	r.OnActivated("Captain America, First Avenger", captainAmericaFirstAvengerActivate)
	r.OnTrigger("Captain America, First Avenger", "begin_combat_controller", captainAmericaCatch)
}

func captainAmericaFirstAvengerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "captain_america_throw_equipment"
	if gs == nil || src == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	// Find highest-MV Equipment currently attached to Captain America.
	// We do this BEFORE paying mana so an activation that can't find an
	// attached Equipment doesn't burn the {3} cost.
	var bestEq *gameengine.Permanent
	bestCMC := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !capAmericaIsEquipment(p.Card) {
			continue
		}
		if p.AttachedTo != src {
			continue
		}
		cmc := gameengine.ManaCostOf(p.Card)
		if cmc > bestCMC {
			bestCMC = cmc
			bestEq = p
		}
	}
	if bestEq == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_equipment_attached", nil)
		return
	}
	// Real cost gate: {3}. Prior code used a "defensive top-up"
	// (`if seat.ManaPool >= 3 { seat.ManaPool -= 3 }`) which allowed
	// the activation to proceed with insufficient mana when callers
	// bypassed the engine's ActivateAbility check (test fixtures, AI
	// evaluator probes, replay rebuilds). Match the Erebos / Tasigur /
	// Yenna pattern: refuse cleanly.
	const manaCost = 3
	if !gameengine.PayGenericCost(gs, seat, manaCost, "activated", "captain_america_throw", src.Card.DisplayName()) {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seat.ManaPool,
			"mana_cost": manaCost,
		})
		return
	}
	// Unattach.
	bestEq.AttachedTo = nil
	dmg := bestCMC
	if dmg <= 0 {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      seatIdx,
			"equipment": bestEq.Card.DisplayName(),
			"damage":    0,
			"note":      "equipment_zero_mv",
		})
		return
	}
	// Distribute damage. Concentrate on the lowest-life living opponent
	// for lethal pressure; if multiple opponents are at risk, fall back to
	// splitting 1 each up to three.
	opps := []int{}
	for _, o := range gs.Opponents(seatIdx) {
		s := gs.Seats[o]
		if s == nil || s.Lost {
			continue
		}
		opps = append(opps, o)
	}
	if len(opps) == 0 {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      seatIdx,
			"equipment": bestEq.Card.DisplayName(),
			"damage":    dmg,
			"note":      "no_opponents",
		})
		return
	}
	// Pick lowest-life opponent.
	target := opps[0]
	bestLife := gs.Seats[target].Life
	for _, o := range opps {
		if gs.Seats[o].Life < bestLife {
			bestLife = gs.Seats[o].Life
			target = o
		}
	}
	// If lethal in one shot, dump it all there. Else split 1 each across
	// up to 3 opponents and pile the rest on lowest-life.
	allocations := map[int]int{}
	if dmg >= bestLife {
		allocations[target] = dmg
	} else {
		// Spread: 1 each up to min(dmg, 3, len(opps)) targets, weighted
		// onto the lowest-life opponent.
		split := dmg
		nTargets := 3
		if nTargets > len(opps) {
			nTargets = len(opps)
		}
		if nTargets > split {
			nTargets = split
		}
		// Sort opps by life ascending for deterministic spread.
		sortedOpps := append([]int(nil), opps...)
		for i := 0; i < len(sortedOpps); i++ {
			for j := i + 1; j < len(sortedOpps); j++ {
				if gs.Seats[sortedOpps[j]].Life < gs.Seats[sortedOpps[i]].Life {
					sortedOpps[i], sortedOpps[j] = sortedOpps[j], sortedOpps[i]
				}
			}
		}
		for i := 0; i < nTargets; i++ {
			allocations[sortedOpps[i]] = 1
			split--
		}
		// Dump remainder on lowest-life opp.
		if split > 0 {
			allocations[sortedOpps[0]] += split
		}
	}
	for opp, amount := range allocations {
		gameengine.DealDamage(gs, opp, amount, src.Card.DisplayName())
	}
	_ = gs.CheckEnd()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":        seatIdx,
		"equipment":   bestEq.Card.DisplayName(),
		"equipment_mv": bestCMC,
		"damage":      dmg,
		"allocations": allocations,
	})
}

func captainAmericaCatch(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "captain_america_catch_attach"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var bestEq *gameengine.Permanent
	bestScore := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !capAmericaIsEquipment(p.Card) {
			continue
		}
		if p.AttachedTo == perm {
			continue // already on Cap
		}
		// Don't strip an Equipment off a creature with higher power than Cap.
		if p.AttachedTo != nil && p.AttachedTo.IsCreature() &&
			p.AttachedTo.Power() > perm.Power() {
			continue
		}
		score := gameengine.ManaCostOf(p.Card) * 2
		if p.AttachedTo == nil {
			score += 100
		}
		if score > bestScore {
			bestScore = score
			bestEq = p
		}
	}
	if bestEq == nil {
		return
	}
	prior := bestEq.AttachedTo
	bestEq.AttachedTo = perm
	priorName := ""
	if prior != nil && prior.Card != nil {
		priorName = prior.Card.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"equipment":     bestEq.Card.DisplayName(),
		"detached_from": priorName,
	})
}

func capAmericaIsEquipment(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "equipment") {
			return true
		}
	}
	return false
}
