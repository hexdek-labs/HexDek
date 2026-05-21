package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCommanderMustardCustom wires Commander Mustard's "Other
// Soldiers you control have vigilance, trample, and haste" anthem.
// The gen_*.go handler covers the {2}{R}{W} attack-pinger activated
// ability; its partial breadcrumb covers the static-anthem half.
//
// Implementation: stamp kw:vigilance / kw:trample / kw:haste on every
// other Soldier creature the controller controls, refreshed on
// Mustard's ETB and on permanent_etb / permanent_ltb. Stamps are
// idempotent (each event reapplies, so removal on Mustard leaving
// the battlefield naturally drops the next refresh — but cleanup is
// permanent_ltb-driven so other Mustards or losing-mustard scenarios
// still re-evaluate cleanly).
const mustardSoldierAnthemMarker = "kw:from_commander_mustard"

func registerCommanderMustardCustom(r *Registry) {
	r.OnETB("Commander Mustard", mustardRefreshAnthemOnETB)
	r.OnTrigger("Commander Mustard", "permanent_etb", mustardRefreshAnthemOnEvent)
	r.OnTrigger("Commander Mustard", "permanent_ltb", mustardRefreshAnthemOnEvent)
}

func mustardRefreshAnthemOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	mustardRefreshSoldierAnthem(gs, perm)
}

func mustardRefreshAnthemOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	mustardRefreshSoldierAnthem(gs, perm)
}

func mustardRefreshSoldierAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mustard_soldier_anthem_refresh"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasSubtype(p.Card, "soldier") {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:vigilance"] = 1
		p.Flags["kw:trample"] = 1
		p.Flags["kw:haste"] = 1
		p.Flags[mustardSoldierAnthemMarker] = 1
		stamped++
	}
	if stamped > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"stamped": stamped,
		})
	}
}
