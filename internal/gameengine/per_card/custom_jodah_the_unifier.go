package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJodahTheUnifierCustom wires Jodah's "Legendary creatures you
// control get +X/+X, where X is the number of legendary creatures you
// control" anthem. The hand-written jodah_the_unifier.go covers the
// cascade-style legendary spell-cast trigger; its ETB hook breadcrumbs
// the static-anthem gap. We close that here via Cynette-style
// Duration-tagged Modification refresh.
//
// X = count of legendary creatures the controller controls (including
// Jodah herself). Each qualifying legendary gets +X/+X, applied via a
// Modification with the jodah_legendary_anthem tag so re-evaluation on
// permanent_etb / _ltb tears down the old buff and stamps a fresh one.
const jodahLegendaryAnthemTag = "jodah_legendary_anthem"

func registerJodahTheUnifierCustom(r *Registry) {
	r.OnETB("Jodah, the Unifier", jodahRefreshAnthemOnETB)
	r.OnTrigger("Jodah, the Unifier", "permanent_etb", jodahRefreshAnthemOnEvent)
	r.OnTrigger("Jodah, the Unifier", "permanent_ltb", jodahRefreshAnthemOnEvent)
}

func jodahRefreshAnthemOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	jodahRefreshLegendaryAnthem(gs, perm)
}

func jodahRefreshAnthemOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	jodahRefreshLegendaryAnthem(gs, perm)
}

func jodahRefreshLegendaryAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "jodah_legendary_anthem_refresh"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	x := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if jodahIsLegendaryCreature(p) {
			x++
		}
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			stripTaggedModifications(p, jodahLegendaryAnthemTag)
			continue
		}
		stripTaggedModifications(p, jodahLegendaryAnthemTag)
		if !jodahIsLegendaryCreature(p) {
			continue
		}
		if x <= 0 {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     x,
			Toughness: x,
			Duration:  jodahLegendaryAnthemTag,
			Timestamp: gs.NextTimestamp(),
		})
		stamped++
	}
	if stamped > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"x":       x,
			"stamped": stamped,
		})
	}
}

func jodahIsLegendaryCreature(p *gameengine.Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	if !cardHasType(p.Card, "legendary") && !cardHasType(p.Card, "Legendary") {
		return false
	}
	return p.IsCreature()
}
