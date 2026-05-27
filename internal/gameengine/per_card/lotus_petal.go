package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLotusPetal wires Lotus Petal.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Lotus%20Petal):
//
//	{T}, Sacrifice this artifact: Add one mana of any color.
//
// {0} Artifact. The premier {0}-cost fast-mana enabler — every turn-1
// 2-mana play (Hermit Druid, Doomsday combos, Vampiric+pass) leans on
// Petal as one of the three "free" mana sources alongside Lotus Petal,
// Mox Diamond, and Chrome Mox.
//
// Implementation:
//   - OnActivated(0): tap Petal, sacrifice it, add 1 mana of any color
//     to the controller's pool. Sacrifice path routes through
//     SacrificePermanent so §614 / §704 replacements (Rest in Peace,
//     etc.) and dies-trigger observers fire correctly.
func registerLotusPetal(r *Registry) {
	r.OnActivated("Lotus Petal", lotusPetalActivate)
}

func lotusPetalActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "lotus_petal_mana"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Lotus Petal", "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Tap, then sacrifice. Add mana FIRST (mana abilities resolve before
	// the sac cost is consumed in the cost-payment subroutine; engine
	// orders this with mana being available immediately).
	src.Tapped = true
	gameengine.AddMana(gs, s, "any", 1, "Lotus Petal")
	gameengine.SacrificePermanent(gs, src, "lotus_petal_mana_ability")

	emit(gs, slug, "Lotus Petal", map[string]interface{}{
		"seat":  seat,
		"color": "any",
		"mana":  1,
	})
}
