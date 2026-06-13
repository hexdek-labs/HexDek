package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Acorn Harvest — {3}{G} Sorcery.
//
//	Create two 1/1 green Squirrel creature tokens.
//	Flashback—{1}{G}, Pay 3 life.
//
// The token creation parsed to a `custom` slug with no handler, so the
// spell resolved to a no-op. This handler creates the two Squirrels. The
// flashback cost is a separate cast-time mechanic and is not implemented
// here.
func init() {
	registerAcornHarvest(Global())
	AddResetHook(registerAcornHarvest)
}

func registerAcornHarvest(r *Registry) {
	r.OnResolve("Acorn Harvest", acornHarvestResolve)
}

func acornHarvestResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "acorn_harvest"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	const n = 2
	for i := 0; i < n; i++ {
		token := &gameengine.Card{
			Name:          "Squirrel Token",
			Owner:         seat,
			BasePower:     1,
			BaseToughness: 1,
			Types:         []string{"token", "creature", "squirrel"},
			Colors:        []string{"G"},
			TypeLine:      "Token Creature — Squirrel",
		}
		enterBattlefieldWithETB(gs, seat, token, false)
	}
	emit(gs, slug, "Acorn Harvest", map[string]interface{}{
		"seat":   seat,
		"tokens": n,
	})
}
