package hat

// R60 round 5 — rollout seed determinism. Pre-fix the rollout RNG
// seed was derived from `var rolloutSeedCounter int64` — a package-
// level mutable global mutated by every call site (MCTSHat /
// YggdrasilHat simulateRollout, multiRolloutForCard). Three concrete
// failure modes:
//
//   1. Test-order dependence: a test running rollouts changed the
//      global, so a later test starting from the same game state
//      sampled a different RNG stream → potentially different chosen
//      action → flaky CI.
//   2. Data race under parallel tournament runners: the `++` is not
//      atomic and the var has no mutex, so two goroutines bumping
//      simultaneously could produce duplicate seeds or torn writes.
//   3. Cross-game leakage: within one process, game N's rollout seed
//      state carried into game N+1, so two "fresh" games starting
//      from the same configured GameState diverged.
//
// Fix: per-hat `rolloutSeed int64` field, reset on game_start.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestMCTSHat_RolloutSeed_IsPerHat(t *testing.T) {
	// Two independent hats — bumping one's seed must not affect the
	// other's. With a package-level counter, this assertion fails.
	a := NewMCTSHat(&GreedyHat{}, nil, 200)
	b := NewMCTSHat(&GreedyHat{}, nil, 200)

	gs := newTestGame(t, 2)
	a.TurnRunner = func(*gameengine.GameState) {}
	b.TurnRunner = func(*gameengine.GameState) {}

	_ = a.simulateRollout(gs, 0, func(*gameengine.GameState) {})
	_ = a.simulateRollout(gs, 0, func(*gameengine.GameState) {})
	_ = a.simulateRollout(gs, 0, func(*gameengine.GameState) {})

	// Hat A bumped its seed 3 times. Hat B should still be at 0 → 1 on
	// its first rollout, not 4.
	if a.rolloutSeed != 3 {
		t.Errorf("hat A: expected rolloutSeed=3 after 3 calls; got %d", a.rolloutSeed)
	}
	if b.rolloutSeed != 0 {
		t.Errorf("hat B should be unaffected by hat A's rollouts; got rolloutSeed=%d", b.rolloutSeed)
	}

	_ = b.simulateRollout(gs, 0, func(*gameengine.GameState) {})
	if b.rolloutSeed != 1 {
		t.Errorf("hat B first rollout should set rolloutSeed=1; got %d", b.rolloutSeed)
	}
}

func TestMCTSHat_RolloutSeed_ResetsOnGameStart(t *testing.T) {
	// Within a single hat across games: the seed must reset on
	// game_start so game N+1 reproduces game N's rollout sequence
	// given identical inputs.
	h := NewMCTSHat(&GreedyHat{}, nil, 200)
	h.TurnRunner = func(*gameengine.GameState) {}
	gs := newTestGame(t, 2)

	_ = h.simulateRollout(gs, 0, func(*gameengine.GameState) {})
	_ = h.simulateRollout(gs, 0, func(*gameengine.GameState) {})
	if h.rolloutSeed != 2 {
		t.Fatalf("expected rolloutSeed=2 after two rollouts; got %d", h.rolloutSeed)
	}

	// Simulate end-of-game → start-of-next-game.
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	if h.rolloutSeed != 0 {
		t.Errorf("game_start must reset rolloutSeed to 0; got %d", h.rolloutSeed)
	}
}

func TestYggdrasilHat_RolloutSeed_IsPerHat(t *testing.T) {
	a := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 200)
	b := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 200)

	gs := newTestGame(t, 2)
	a.TurnRunner = func(*gameengine.GameState) {}
	b.TurnRunner = func(*gameengine.GameState) {}

	_ = a.simulateRollout(gs, 0, func(*gameengine.GameState) {})
	_ = a.simulateRollout(gs, 0, func(*gameengine.GameState) {})

	if a.rolloutSeed != 2 {
		t.Errorf("YggdrasilHat A: expected rolloutSeed=2; got %d", a.rolloutSeed)
	}
	if b.rolloutSeed != 0 {
		t.Errorf("YggdrasilHat B must be independent; got rolloutSeed=%d", b.rolloutSeed)
	}
}

func TestYggdrasilHat_RolloutSeed_ResetsOnGameStart(t *testing.T) {
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 200)
	h.TurnRunner = func(*gameengine.GameState) {}
	gs := newTestGame(t, 2)

	_ = h.simulateRollout(gs, 0, func(*gameengine.GameState) {})
	_ = h.simulateRollout(gs, 0, func(*gameengine.GameState) {})
	if h.rolloutSeed != 2 {
		t.Fatalf("expected rolloutSeed=2 after two rollouts; got %d", h.rolloutSeed)
	}

	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	if h.rolloutSeed != 0 {
		t.Errorf("game_start must reset rolloutSeed to 0; got %d", h.rolloutSeed)
	}
}

// End-to-end: two fresh hats with identical configuration, each
// game_start'd then asked for ONE rollout from the same starting
// GameState, must produce the same evaluator score. Pre-fix the
// package global meant the two hats sampled different seeds and
// could return different floats.
func TestRolloutSeed_TwoFreshHatsReproduceScore(t *testing.T) {
	build := func() (*MCTSHat, *gameengine.GameState) {
		gs := newTestGame(t, 2)
		gs.Seats[0].Life = 40
		gs.Seats[0].StartingLife = 40
		gs.Seats[1].Life = 40
		gs.Seats[1].StartingLife = 40
		h := NewMCTSHat(&GreedyHat{}, nil, 200)
		h.TurnRunner = func(*gameengine.GameState) {}
		h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
		return h, gs
	}

	hatA, gsA := build()
	scoreA := hatA.simulateRollout(gsA, 0, func(*gameengine.GameState) {})

	// Simulate "an unrelated hat in the same process ran some
	// rollouts in between" — pre-fix this would pollute hatB's seed.
	noisy := NewMCTSHat(&GreedyHat{}, nil, 200)
	noisy.TurnRunner = func(*gameengine.GameState) {}
	for i := 0; i < 7; i++ {
		_ = noisy.simulateRollout(gsA, 0, func(*gameengine.GameState) {})
	}

	hatB, gsB := build()
	scoreB := hatB.simulateRollout(gsB, 0, func(*gameengine.GameState) {})

	if scoreA != scoreB {
		t.Errorf("two fresh hats from identical state must produce same rollout score regardless of intervening rollouts in the process: A=%.6f B=%.6f",
			scoreA, scoreB)
	}
}
