package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGalazethPrismari wires Galazeth Prismari.
//
// Oracle text:
//
//   Flying
//   When Galazeth Prismari enters, create a Treasure token.
//   Artifacts you control have "{T}: Add one mana of any color. Spend this mana only to cast an instant or sorcery spell."
//
// Auto-generated ETB handler.
func registerGalazethPrismari(r *Registry) {
	r.OnETB("Galazeth Prismari", galazethPrismariETB)
}

func galazethPrismariETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "galazeth_prismari_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Creature Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "creature"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
