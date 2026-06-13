package per_card

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Battle Screech — {3}{W} Sorcery.
//
//	Create two 1/1 white Bird creature tokens with flying.
//	Flashback—Tap three untapped white creatures you control.
//
// The token creation parsed to a `custom` slug with no handler, so the
// spell resolved to a no-op. This handler creates the two flying Birds.
// Each token carries an AST with a flying Keyword so the engine's
// HasKeyword/layer path grants flight (the Card struct has no Keywords
// field — keywords come from the AST). The flashback cost is a separate
// cast-time mechanic and is not implemented here.
func init() {
	registerBattleScreech(Global())
	AddResetHook(registerBattleScreech)
}

func registerBattleScreech(r *Registry) {
	r.OnResolve("Battle Screech", battleScreechResolve)
}

func battleScreechResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "battle_screech"
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
			Name:          "Bird Token",
			Owner:         seat,
			BasePower:     1,
			BaseToughness: 1,
			Types:         []string{"token", "creature", "bird"},
			Colors:        []string{"W"},
			TypeLine:      "Token Creature — Bird",
			AST: &gameast.CardAST{
				Name: "Bird Token",
				Abilities: []gameast.Ability{
					&gameast.Keyword{Name: "flying"},
				},
			},
		}
		enterBattlefieldWithETB(gs, seat, token, false)
	}
	emit(gs, slug, "Battle Screech", map[string]interface{}{
		"seat":   seat,
		"tokens": n,
	})
}
