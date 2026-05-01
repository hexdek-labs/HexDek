package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYennaRedtoothRegent wires Yenna, Redtooth Regent.
//
// Oracle text:
//
//   {2}, {T}: Choose target enchantment you control that doesn't have the same name as another permanent you control. Create a token that's a copy of it, except it isn't legendary. If the token is an Aura, untap Yenna, then scry 2. Activate only as a sorcery.
//
// Auto-generated activated ability handler.
func registerYennaRedtoothRegent(r *Registry) {
	r.OnActivated("Yenna, Redtooth Regent", yennaRedtoothRegentActivate)
}

func yennaRedtoothRegentActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "yenna_redtooth_regent_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Creature Token",
		Owner:         src.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "creature"},
	}
	enterBattlefieldWithETB(gs, src.Controller, token, false)
	gameengine.Scry(gs, src.Controller, 2)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
