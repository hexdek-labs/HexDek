package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEmmaraSoulOfTheAccord wires Emmara, Soul of the Accord.
//
// Oracle text:
//
//	Whenever Emmara becomes tapped, create a 1/1 white Soldier creature
//	token with lifelink.
func registerEmmaraSoulOfTheAccord(r *Registry) {
	r.OnTrigger("Emmara, Soul of the Accord", "permanent_tapped", emmaraTapped)
}

func emmaraTapped(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "emmara_tap_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	tapped, _ := ctx["perm"].(*gameengine.Permanent)
	if tapped != perm {
		return
	}
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Soldier Token",
		[]string{"creature", "soldier", "pip:W"}, 1, 1)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		gameengine.SetKeywordFlags(gs, tok, "lifelink")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
