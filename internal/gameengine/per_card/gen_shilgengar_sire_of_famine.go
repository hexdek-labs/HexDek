package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerShilgengarSireOfFamine wires Shilgengar, Sire of Famine.
//
// Oracle text:
//
//   Flying
//   Sacrifice another creature: Create a Blood token. If you sacrificed an Angel this way, create a number of Blood tokens equal to its toughness instead.
//   {W/B}{W/B}{W/B}, Sacrifice six Blood tokens: Return each creature card from your graveyard to the battlefield with a finality counter on it. Those creatures are Vampires in addition to their other types.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerShilgengarSireOfFamine(r *Registry) {
	r.OnETB("Shilgengar, Sire of Famine", shilgengarSireOfFamineStaticETB)
}

func shilgengarSireOfFamineStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "shilgengar_sire_of_famine_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
