package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Army of the Damned — {5}{B}{B} Sorcery.
//
//	Create thirteen tapped 2/2 black Zombie creature tokens.
//	Flashback {7}{B}{B}{B}.
//
// The token creation parsed to a `custom` slug
// (army_damned_create_thirteen_zombies_tapped) with no per-card handler,
// so the spell resolved to a logged no-op — thirteen Zombies never
// appeared. This handler creates them. The flashback cost is a separate
// cast-time mechanic (the parser emits army_damned_flashback_7BBB); it is
// NOT implemented here and remains a known gap (see report).
func init() {
	registerArmyOfTheDamned(Global())
	AddResetHook(registerArmyOfTheDamned)
}

func registerArmyOfTheDamned(r *Registry) {
	r.OnResolve("Army of the Damned", armyOfTheDamnedResolve)
}

func armyOfTheDamnedResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "army_of_the_damned"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	const n = 13
	for i := 0; i < n; i++ {
		token := &gameengine.Card{
			Name:          "Zombie Token",
			Owner:         seat,
			BasePower:     2,
			BaseToughness: 2,
			Types:         []string{"token", "creature", "zombie"},
			Colors:        []string{"B"},
			TypeLine:      "Token Creature — Zombie",
		}
		enterBattlefieldWithETB(gs, seat, token, true /* tapped */)
	}
	emit(gs, slug, "Army of the Damned", map[string]interface{}{
		"seat":   seat,
		"tokens": n,
	})
}
