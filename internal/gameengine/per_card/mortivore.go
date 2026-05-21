package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMortivore wires Mortivore (Planar Chaos, {3}{B}).
//
// Oracle text (Scryfall, verified):
//
//	Mortivore's power and toughness are each equal to the number of
//	creature cards in all graveyards.
//	{B}: Regenerate Mortivore.
//
// Implementation (R55 batch):
//   - Layer 7b dynamic P/T CDA via RegisterDynamicSetPT — counts
//     creature-typed Cards across every seat's graveyard each layer
//     pass.
//   - Regenerate is a legacy keyword that the engine handles via the
//     AST keyword pipeline. Not wired here.
func registerMortivore(r *Registry) {
	r.OnETB("Mortivore", mortivoreETB)
}

func mortivoreETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mortivore_cda_layer7b"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterDynamicSetPT(gs, perm, mortivoreCountCreatureCardsInYards,
		gameengine.DurationUntilSourceLeaves, "Mortivore", "cda_pt")
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func mortivoreCountCreatureCardsInYards(gs *gameengine.GameState, perm *gameengine.Permanent) (int, int) {
	if gs == nil {
		return 0, 0
	}
	n := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			if cardHasType(c, "creature") {
				n++
			}
		}
	}
	return n, n
}
