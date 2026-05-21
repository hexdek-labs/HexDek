package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLordOfExtinction wires Lord of Extinction (Alara Reborn, {2}{B}{G}).
//
// Oracle text (Scryfall, verified):
//
//	Lord of Extinction's power and toughness are each equal to the
//	number of cards in all graveyards.
//
// Implementation (R55 batch):
//   - Pure characteristic-defining ability (CR §613.2). Layer 7b
//     dynamic P/T via RegisterDynamicSetPT — the compute fn sums
//     graveyard sizes across every seat each layer pass.
//   - No counters / triggered abilities / activated abilities; the
//     CDA is the entire mechanical text.
func registerLordOfExtinction(r *Registry) {
	r.OnETB("Lord of Extinction", lordOfExtinctionETB)
}

func lordOfExtinctionETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lord_of_extinction_cda_layer7b"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterDynamicSetPT(gs, perm, lordOfExtinctionGraveyardCount,
		gameengine.DurationUntilSourceLeaves, "Lord of Extinction", "cda_pt")
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func lordOfExtinctionGraveyardCount(gs *gameengine.GameState, perm *gameengine.Permanent) (int, int) {
	if gs == nil {
		return 0, 0
	}
	n := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Graveyard {
			if c != nil {
				n++
			}
		}
	}
	return n, n
}
