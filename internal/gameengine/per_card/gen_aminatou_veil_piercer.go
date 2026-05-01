package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAminatouVeilPiercer wires Aminatou, Veil Piercer.
//
// Oracle text:
//
//   At the beginning of your upkeep, surveil 2. (Look at the top two cards of your library, then put any number of them into your graveyard and the rest on top of your library in any order.)
//   Each enchantment card in your hand has miracle. Its miracle cost is equal to its mana cost reduced by {4}. (You may cast a card for its miracle cost when you draw it if it's the first card you drew this turn.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerAminatouVeilPiercer(r *Registry) {
	r.OnETB("Aminatou, Veil Piercer", aminatouVeilPiercerStaticETB)
}

func aminatouVeilPiercerStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "aminatou_veil_piercer_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
