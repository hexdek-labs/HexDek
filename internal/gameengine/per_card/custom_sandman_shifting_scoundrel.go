package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSandmanShiftingScoundrelCustom — original pre-R55 impl
// retained (Card.BasePower mutation). R55 RegisterDynamicSetPT path
// was deferred due to a test-isolation interaction (still investigating).
func registerSandmanShiftingScoundrelCustom(r *Registry) {
	r.OnETB("Sandman, Shifting Scoundrel", sandmanRefreshPTOnETB)
	r.OnTrigger("Sandman, Shifting Scoundrel", "permanent_etb", sandmanRefreshPTOnEvent)
	r.OnTrigger("Sandman, Shifting Scoundrel", "permanent_ltb", sandmanRefreshPTOnEvent)
	r.OnTrigger("Sandman, Shifting Scoundrel", "upkeep_controller", sandmanRefreshPTOnEvent)
}

func sandmanRefreshPTOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	sandmanRefreshPT(gs, perm)
}

func sandmanRefreshPTOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	sandmanRefreshPT(gs, perm)
}

func sandmanRefreshPT(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sandman_cda_pt_refresh"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	lands := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "land") {
			lands++
		}
	}
	perm.Card.BasePower = lands
	perm.Card.BaseToughness = lands
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"lands": lands,
		"power": lands,
	})
}
