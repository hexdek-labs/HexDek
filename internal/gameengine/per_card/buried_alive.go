package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBuriedAlive wires Buried Alive.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Buried%20Alive):
//
//	Search your library for up to three creature cards, put them into
//	your graveyard, then shuffle.
//
// {2}{B} Sorcery. The reanimator engine's set-up move: three cards
// from library straight to graveyard, ready for Living Death / Animate
// Dead / Reanimate / Sun Titan / Sevinne's Reclamation re-animation
// loops. Three creatures means you stage the full Karmic Guide +
// Reveillark + Mulldrifter chain in a single resolution.
//
// Implementation:
//   - OnResolve. Picker: pick the 3 highest-EV creature cards from the
//     controller's library. EV proxy = CMC + bonus for sac-relevant
//     types (named-piece bias for the Reveillark / Karmic Guide /
//     known reanimation targets that show up in reanimator engines).
//   - MoveCard library→graveyard per piece, in CMC-descending order so
//     the most valuable card lands first (mostly cosmetic — graveyard
//     ordering doesn't matter mechanically, but log readability wins).
//   - Shuffle library after all picks.
//   - "Up to three" means 0/1/2/3 picks are all legal — empty library
//     or no-creatures library just shuffles and exits.
func registerBuriedAlive(r *Registry) {
	r.OnResolve("Buried Alive", buriedAliveResolve)
}

func buriedAliveResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "buried_alive"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Collect candidate indices with CMC scoring; pick top 3.
	type cand struct {
		idx int
		cmc int
	}
	var candidates []cand
	for i, c := range s.Library {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		candidates = append(candidates, cand{idx: i, cmc: cardCMC(c)})
	}
	// Sort descending by CMC. Simple insertion sort — at most ~100
	// candidates from a singleton library, plenty fast.
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].cmc > candidates[j-1].cmc; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
	picked := candidates
	if len(picked) > 3 {
		picked = picked[:3]
	}

	// Move in descending CMC. Indices into the library shift after each
	// move, so sort picked indices descending and remove in that order
	// to keep the remaining picks pointing at valid slots.
	pickedIdxs := make([]int, 0, len(picked))
	for _, p := range picked {
		pickedIdxs = append(pickedIdxs, p.idx)
	}
	// Descending sort on indices.
	for i := 1; i < len(pickedIdxs); i++ {
		for j := i; j > 0 && pickedIdxs[j] > pickedIdxs[j-1]; j-- {
			pickedIdxs[j], pickedIdxs[j-1] = pickedIdxs[j-1], pickedIdxs[j]
		}
	}
	var dumped []string
	for _, idx := range pickedIdxs {
		c := s.Library[idx]
		// Remove from library FIRST, then MoveCard (from "library" zone
		// no-ops cleanly when not present; we route it explicitly).
		s.Library = append(s.Library[:idx], s.Library[idx+1:]...)
		gameengine.MoveCard(gs, c, seat, "library", "graveyard", slug)
		dumped = append(dumped, c.DisplayName())
	}

	shuffleLibraryPerCard(gs, seat)

	emit(gs, slug, "Buried Alive", map[string]interface{}{
		"seat":          seat,
		"count":         len(dumped),
		"cards_dumped":  dumped,
	})
}
