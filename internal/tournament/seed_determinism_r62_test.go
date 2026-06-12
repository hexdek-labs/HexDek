package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// TestSeedDeterminism_NoisyYggdrasilGameRepeatsExactly is the r62
// seed-determinism regression: the same seed + the same decks must
// produce the identical game outcome twice — with the PRODUCTION hat
// (YggdrasilHat, noise 0.2), not the GreedyHat workaround the older
// contract e2e test had to use.
//
// Pre-r62 this failed three ways:
//   - the hat noise RNG was seeded from the global math/rand at
//     construction (wall-clock-ish), so targeting diverged per run;
//   - several per_card handlers (Vial Smasher, Krark, Zndrsplt,
//     Viconia, Niv-Mizzet Reborn, Atraxa Grand Unifier) drew from the
//     global source;
//   - the turn runner cut turns on a 15s wall-clock budget, so host
//     load changed outcomes.
//
// Skips (like every e2e here) when the corpus / decks are unavailable.
func TestSeedDeterminism_NoisyYggdrasilGameRepeatsExactly(t *testing.T) {
	const nSeats = 3
	decks := e2eDecks(t, nSeats)

	noisyHats := func() []HatFactory {
		hats := make([]HatFactory, nSeats)
		for i := range hats {
			hats[i] = func() gameengine.Hat {
				return hat.NewYggdrasilHatWithNoise(nil, 50, 0.2)
			}
		}
		return hats
	}

	// 3 seats × 20 turns: multiplayer decision surface (attack-direction
	// and targeting choices among multiple opponents, selectAmongTop
	// ties) so the noise RNG actually flips choices, while staying well
	// inside the suite's 600s budget. Sizing notes (r62): a 2-seat
	// 12-turn game PASSED on pre-fix code — too few near-tie decisions
	// for the nondeterministic noise stream to change an outcome — and
	// data/decks/personal carries only 3 decks, so 4 seats would skip.
	// The unit tests in hat/noise_determinism_r62_test.go and per_card/
	// rng_determinism_r62_test.go pin each nondeterminism source
	// mechanically regardless of game size.
	run := func() GameOutcome {
		return runOneGame(0, decks, noisyHats(), nSeats, 424242, 20, true, false, false, nil, contractParams{
			key:           []byte("test-key-1234567890abcdef"),
			context:       "test:determinism-r62",
			engineVersion: "test-build",
		})
	}

	first := run()
	second := run()

	if first.Winner != second.Winner || first.Turns != second.Turns {
		t.Fatalf("noisy-hat game nondeterministic under identical seed:\n first: winner=%d turns=%d\n second: winner=%d turns=%d",
			first.Winner, first.Turns, second.Winner, second.Turns)
	}
	if first.SeedContract == nil || second.SeedContract == nil {
		t.Fatal("SeedContract not attached to outcome")
	}
	if first.SeedContract.OutcomeDigest == "" {
		t.Fatal("OutcomeDigest not sealed on first run")
	}
	if first.SeedContract.OutcomeDigest != second.SeedContract.OutcomeDigest {
		t.Fatalf("outcome digest nondeterministic with noisy Yggdrasil hats: %s vs %s",
			first.SeedContract.OutcomeDigest, second.SeedContract.OutcomeDigest)
	}
}
