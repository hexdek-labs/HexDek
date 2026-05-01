package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGorma wires Gorma, the Gullet.
//
// Oracle text:
//
//	Lifelink
//	Whenever another creature you control dies, put a +1/+1 counter on
//	Gorma.
//	Nontoken creatures you control enter with an additional +1/+1
//	counter on them for each creature that died under your control this
//	turn.
//
// Implementation:
//   - Lifelink is granted via the AST keyword path; no per-card hook.
//   - "creature_dies" trigger fires when ANY creature dies; we gate on
//     controller_seat == Gorma's controller, and skip Gorma itself
//     ("another creature"). Adds a +1/+1 counter to Gorma and increments
//     a per-seat death counter used by the static.
//   - "permanent_etb" trigger seeds N additional +1/+1 counters on each
//     nontoken creature entering under Gorma's controller, where N is
//     the death count tracked above. Modeled as a post-ETB trigger
//     rather than a §614 replacement so subsequent ETB observers see the
//     buffed creature, matching Tayam's pattern.
//   - "end_step" trigger resets the per-seat death counter once per turn.
const gormaDeathCountKey = "gorma_creatures_died_this_turn"

func registerGorma(r *Registry) {
	r.OnTrigger("Gorma, the Gullet", "creature_dies", gormaDeathTrigger)
	r.OnTrigger("Gorma, the Gullet", "permanent_etb", gormaETBStatic)
	r.OnTrigger("Gorma, the Gullet", "end_step", gormaEndStepReset)
}

func gormaDeathTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gorma_death_counter"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	// "another creature" — skip Gorma's own death (defensive; fireTrigger
	// walks the battlefield, so Gorma being in the graveyard would mean
	// the handler doesn't fire anyway).
	if dyingPerm, _ := ctx["perm"].(*gameengine.Permanent); dyingPerm == perm {
		return
	}

	perm.AddCounter("+1/+1", 1)
	seat := gs.Seats[perm.Controller]
	if seat != nil {
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags[gormaDeathCountKey]++
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"plus_one_total": perm.Counters["+1/+1"],
		"deaths_this_turn": func() int {
			if seat == nil || seat.Flags == nil {
				return 0
			}
			return seat.Flags[gormaDeathCountKey]
		}(),
	})
}

func gormaETBStatic(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gorma_etb_static_counters"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering == perm {
		return
	}
	if !entering.IsCreature() {
		return
	}
	// Static specifies "nontoken creatures."
	if entering.IsToken() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	n := seat.Flags[gormaDeathCountKey]
	if n <= 0 {
		return
	}
	entering.AddCounter("+1/+1", n)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"target":           entering.Card.DisplayName(),
		"counters_added":   n,
		"deaths_this_turn": n,
	})
}

func gormaEndStepReset(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	if seat.Flags[gormaDeathCountKey] == 0 {
		return
	}
	seat.Flags[gormaDeathCountKey] = 0
}
