package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKujaGenomeSorcererTranceKujaFateDefied wires Kuja, Genome Sorcerer // Trance Kuja, Fate Defied.
//
// Oracle text:
//
//   At the beginning of your end step, create a tapped 0/1 black Wizard creature token with "Whenever you cast a noncreature spell, this token deals 1 damage to each opponent." Then if you control four or more Wizards, transform Kuja.
//   Flare Star — If a Wizard you control would deal damage to a permanent or player, it deals double that damage instead.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerKujaGenomeSorcererTranceKujaFateDefied(r *Registry) {
	r.OnETB("Kuja, Genome Sorcerer // Trance Kuja, Fate Defied", kujaGenomeSorcererTranceKujaFateDefiedStaticETB)
}

func kujaGenomeSorcererTranceKujaFateDefiedStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kuja_genome_sorcerer_trance_kuja_fate_defied_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
