package per_card

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// rngIntn returns a uniform int in [0, n) drawn from the per-game
// deterministic RNG (gs.Rng) when available, falling back to the
// process-global locked source otherwise.
//
// Handlers must NEVER call bare rand.Intn/rand.Shuffle: the global
// stream interleaves across Loki's parallel workers, so any game
// containing such a card is not reproducible from its seed and its
// outcome varies with --workers (r62 seed-determinism fix; same root
// as the r55 Krark gs.Rng preference). TestPerCardNoBareGlobalRand
// pins this rule.
func rngIntn(gs *gameengine.GameState, n int) int {
	if gs != nil && gs.Rng != nil {
		return gs.Rng.Intn(n)
	}
	return rand.Intn(n)
}

// rngShuffle is the gs.Rng-preferring counterpart of rand.Shuffle.
// See rngIntn for why bare global rand is forbidden in handlers.
func rngShuffle(gs *gameengine.GameState, n int, swap func(i, j int)) {
	if gs != nil && gs.Rng != nil {
		gs.Rng.Shuffle(n, swap)
		return
	}
	rand.Shuffle(n, swap)
}
