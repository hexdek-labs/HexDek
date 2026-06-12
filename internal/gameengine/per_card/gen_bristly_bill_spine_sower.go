package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBristlyBillSpineSower wires Bristly Bill, Spine Sower.
//
// Oracle text (OTJ, {1}{G}, 2/2):
//
//	Landfall — Whenever a land you control enters, put a +1/+1 counter
//	on target creature.
//	{3}{G}{G}: Double the number of +1/+1 counters on each creature
//	you control.
//
// Implementation:
//   - "permanent_etb" trigger fires the landfall counter when a land
//     controlled by Bristly Bill's controller enters. Target picked
//     greedily: prefer Bristly Bill himself if he has counters
//     (snowballs), else any controlled creature with at least one
//     +1/+1 counter, else the highest-power creature.
//   - Activated ability {3}{G}{G} doubles every controlled creature's
//     +1/+1 counters. The mana cost is not enforced by this handler
//     (the engine's activation path collects cost externally; flagged
//     in cost-unenforced tracking).
func registerBristlyBillSpineSower(r *Registry) {
	r.OnTrigger("Bristly Bill, Spine Sower", "permanent_etb", bristlyBillLandfall)
	r.OnActivated("Bristly Bill, Spine Sower", bristlyBillSpineSowerDouble)
}

func bristlyBillLandfall(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bristly_bill_landfall_counter"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || !entering.IsLand() {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	target := pickBristlyBillTarget(gs, perm)
	if target == nil {
		return
	}
	target.AddCounter("+1/+1", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"land":   entering.Card.DisplayName(),
		"target": target.Card.DisplayName(),
	})
}

func pickBristlyBillTarget(gs *gameengine.GameState, perm *gameengine.Permanent) *gameengine.Permanent {
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return nil
	}
	if perm.Counters != nil && perm.Counters["+1/+1"] > 0 {
		return perm
	}
	var fallback *gameengine.Permanent
	bestPower := -1 << 30
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Counters != nil && p.Counters["+1/+1"] > 0 {
			return p
		}
		if p.Power() > bestPower {
			bestPower = p.Power()
			fallback = p
		}
	}
	if fallback != nil {
		return fallback
	}
	if perm.IsCreature() {
		return perm
	}
	return nil
}

func bristlyBillSpineSowerDouble(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "bristly_bill_double_counters"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	// {3}{G}{G} mana cost — defensive gate for non-engine callers.
	if !payDefensiveManaCost(src, seat, abilityIdx, 5) {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  5,
			"mana_pool": seat.ManaPool,
		})
		return
	}
	doubled := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Counters == nil {
			continue
		}
		n := p.Counters["+1/+1"]
		if n <= 0 {
			continue
		}
		p.AddCounter("+1/+1", n)
		doubled++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             src.Controller,
		"creatures_doubled": doubled,
	})
}
