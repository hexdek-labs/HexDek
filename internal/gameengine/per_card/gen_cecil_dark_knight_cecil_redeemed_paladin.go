package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCecilDarkKnightCecilRedeemedPaladin wires Cecil, Dark Knight // Cecil, Redeemed Paladin.
//
// Oracle text:
//
//   Deathtouch
//   Darkness — Whenever Cecil deals damage, you lose that much life. Then if your life total is less than or equal to half your starting life total, untap Cecil and transform it.
//   Lifelink
//   Protect — Whenever Cecil attacks, other attacking creatures gain indestructible until end of turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerCecilDarkKnightCecilRedeemedPaladin(r *Registry) {
	r.OnETB("Cecil, Dark Knight // Cecil, Redeemed Paladin", cecilDarkKnightCecilRedeemedPaladinStaticETB)
}

func cecilDarkKnightCecilRedeemedPaladinStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cecil_dark_knight_cecil_redeemed_paladin_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
