package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWorldspineWurm wires Worldspine Wurm (Muninn parser-gap rank
// ~160, big-creature Eldrazi-tier finisher).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{8}{G}{G}{G}
//	Creature — Wurm
//	Trample
//	When this creature dies, create three 5/5 green Wurm creature tokens
//	with trample.
//	When Worldspine Wurm is put into a graveyard from anywhere, shuffle
//	it into its owner's library.
//
// Implementation:
//   - Trample via AST keyword pipeline.
//   - OnTrigger("creature_dies"): gated on dying perm being Worldspine
//     itself. Create three 5/5 Wurm tokens with kw:trample under
//     perm.Controller via gameengine.CreateCreatureToken.
//   - The "put into graveyard from anywhere → shuffle into library"
//     trigger fires on graveyard arrival regardless of source zone
//     (battlefield, library, exile, hand). The engine's per_card
//     pipeline currently only exposes battlefield-related trigger
//     events (permanent_ltb, creature_dies). To cover the non-battlefield
//     paths we'd need a generic "graveyard_arrival" event hook. We
//     handle the battlefield path here (route through MoveCard from
//     graveyard → library_top, then shuffle) and emitPartial about the
//     other zones (Liliana-style discard-to-graveyard, mill-from-library,
//     etc., where Worldspine should also reshuffle).
func registerWorldspineWurm(r *Registry) {
	r.OnTrigger("Worldspine Wurm", "creature_dies", worldspineWurmDies)
}

func worldspineWurmDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "worldspine_wurm_dies"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying != perm {
		return
	}
	seat := perm.Controller
	for i := 0; i < 3; i++ {
		tok := gameengine.CreateCreatureToken(gs, seat, "Wurm Token",
			[]string{"creature", "wurm"}, 5, 5)
		if tok != nil {
			if tok.Flags == nil {
				tok.Flags = map[string]int{}
			}
			gameengine.SetKeywordFlags(gs, tok, "trample")
		}
	}
	// Reshuffle Worldspine into owner's library. Worldspine is in its
	// owner's graveyard now (creature_dies fires post-zone-move).
	owner := perm.Card.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		owner = perm.Controller
	}
	gameengine.MoveCard(gs, perm.Card, owner, "graveyard", "library", "worldspine_wurm_reshuffle")
	shuffleLibraryPerCard(gs, owner)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"tokens": 3,
		"owner":  owner,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"reshuffle_only_modelled_for_battlefield_death_path_other_zones_unhooked")
}
