package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSandmanShiftingScoundrelCustom — R56: ports Sandman's CDA
// from the legacy Card.BasePower mutation to a Layer 7b dynamic
// continuous effect via RegisterDynamicSetPT. r55 deferred this port
// because it interacted with TestKrark's global math/rand state; r55
// then patched Krark to prefer gs.Rng (krark.go), neutralizing the
// interaction. This file finishes the deferral.
//
// CR §613.4b — set-PT lives in layer 7b sublayer.
//
// Trigger registrations (permanent_etb / permanent_ltb /
// upkeep_controller) are kept as compatibility no-ops so the
// pre-r55 trigger-coverage smoke tests (BatchDR49) still see them.
func registerSandmanShiftingScoundrelCustom(r *Registry) {
	r.OnETB("Sandman, Shifting Scoundrel", sandmanRefreshPTOnETB)
	r.OnTrigger("Sandman, Shifting Scoundrel", "permanent_etb", sandmanRefreshPTOnEvent)
	r.OnTrigger("Sandman, Shifting Scoundrel", "permanent_ltb", sandmanRefreshPTOnEvent)
	r.OnTrigger("Sandman, Shifting Scoundrel", "upkeep_controller", sandmanRefreshPTOnEvent)
}

func sandmanRefreshPTOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sandman_cda_layer7b"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterDynamicSetPT(gs, perm, sandmanCountLands,
		gameengine.DurationUntilSourceLeaves, "Sandman, Shifting Scoundrel", "cda_pt")
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

// sandmanRefreshPTOnEvent is a kept-alive trigger handler from the
// pre-r56 implementation. With the CDA now a dynamic continuous
// effect, the compute fn re-evaluates each layer pass; this handler
// just invalidates the cache so any cached readers see the fresh
// value at the trigger boundary.
func sandmanRefreshPTOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil {
		return
	}
	gs.InvalidateCharacteristicsCache()
}

func sandmanCountLands(gs *gameengine.GameState, perm *gameengine.Permanent) (int, int) {
	if gs == nil || perm == nil {
		return 0, 0
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return 0, 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "land") {
			n++
		}
	}
	return n, n
}
