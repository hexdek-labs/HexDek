package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Mikaeus, the Lunarch — {X}{W} 0/0 Legendary Creature — Human Cleric.
//
//   Mikaeus enters with X +1/+1 counters on it.
//   {T}: Put a +1/+1 counter on Mikaeus.
//   {T}, Remove a +1/+1 counter from Mikaeus: Put a +1/+1 counter on
//   each other creature you control.
//
// Implementation:
//   - OnETB: read X from perm.Flags["x_paid"] (Ingenious Prodigy pattern
//     at ingenious_prodigy.go:47) and AddCounter("+1/+1", X). Defensive
//     zero when flag missing (Mikaeus enters as 0/0 with no counters).
//   - OnActivated(0) {T}: tap + AddCounter("+1/+1", 1). The engine
//     cost-pay path handles the tap pre-handler-fire.
//   - OnActivated(1) {T}, remove counter: confirm Mikaeus has ≥1
//     +1/+1 counter (cost-validate); -1 counter on Mikaeus; +1/+1
//     counter on each OTHER creature controlled by Mikaeus' controller.

func init() {
	registerMikaeusTheLunarch(Global())
	AddResetHook(registerMikaeusTheLunarch)
}

func registerMikaeusTheLunarch(r *Registry) {
	r.OnETB("Mikaeus, the Lunarch", mikaeusLunarchETB)
	r.OnActivated("Mikaeus, the Lunarch", mikaeusLunarchActivate)
}

func mikaeusLunarchETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mikaeus_lunarch_etb_x_counters"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	x := 0
	if perm.Flags != nil {
		x = perm.Flags["x_paid"]
	}
	if x < 0 {
		x = 0
	}
	if x > 0 {
		perm.AddCounter("+1/+1", x)
	} else {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"x_paid_not_threaded_into_etb_hook_enters_at_zero_counters")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"x_value":     x,
		"counters":    perm.Counters["+1/+1"],
	})
}

func mikaeusLunarchActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		mikaeusLunarchSelfPump(gs, src)
	case 1:
		mikaeusLunarchSpreadCounters(gs, src)
	}
}

func mikaeusLunarchSelfPump(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "mikaeus_lunarch_self_pump"
	src.AddCounter("+1/+1", 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"counters": src.Counters["+1/+1"],
	})
}

func mikaeusLunarchSpreadCounters(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "mikaeus_lunarch_spread_counters"
	if src.Counters == nil || src.Counters["+1/+1"] < 1 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_counter_to_remove", nil)
		return
	}
	src.AddCounter("+1/+1", -1)
	s := gs.Seats[src.Controller]
	if s == nil {
		return
	}
	pumped := 0
	for _, p := range s.Battlefield {
		if p == nil || p == src || !p.IsCreature() {
			continue
		}
		p.AddCounter("+1/+1", 1)
		pumped++
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":              src.Controller,
		"pumped_count":      pumped,
		"mikaeus_counters":  src.Counters["+1/+1"],
	})
}
