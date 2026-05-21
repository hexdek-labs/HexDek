package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMultaniMaroSorcerer wires Multani, Maro-Sorcerer (Urza's Saga,
// {3}{G}{G}).
//
// Oracle text (Scryfall, verified):
//
//	Multani's power and toughness are each equal to the number of
//	cards in all players' hands.
//
// Implementation (R55 batch):
//   - Pure characteristic-defining ability (CR §613.2). Layer 7b
//     dynamic P/T via RegisterDynamicSetPT — the compute fn sums
//     hand sizes across every seat each layer pass.
func registerMultaniMaroSorcerer(r *Registry) {
	r.OnETB("Multani, Maro-Sorcerer", multaniETB)
}

func multaniETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "multani_cda_layer7b"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterDynamicSetPT(gs, perm, multaniCountAllHands,
		gameengine.DurationUntilSourceLeaves, "Multani, Maro-Sorcerer", "cda_pt")
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func multaniCountAllHands(gs *gameengine.GameState, perm *gameengine.Permanent) (int, int) {
	if gs == nil {
		return 0, 0
	}
	n := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		n += len(s.Hand)
	}
	return n, n
}
