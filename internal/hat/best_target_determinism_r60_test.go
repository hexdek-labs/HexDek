package hat

import (
	"math/rand"
	"testing"
)

// Regression guards for the BestTarget flake disclosed in PRs
// #450/#452/#454/#460. The root cause was that NewYggdrasilHat seeds
// noiseRNG via `rand.New(rand.NewSource(rand.Int63()))` (yggdrasil.go:375),
// and Go 1.20+ auto-seeds the global rand source per process. The test
// fixture primedYggdrasilHat zeroed `Noise` but never re-seeded the
// RNG, so selectAmongTop's `noiseRNG.Intn(topN)` tie-breaker flipped
// across invocations. The fix:
//
//   1. primedYggdrasilHat now seeds noiseRNG with rand.NewSource(1).
//   2. primedYggdrasilHat now sets confidenceThreshold to 0.75 so the
//      margin (1 - threshold = 0.25) is tight enough that clearly-
//      better candidates win outright without consulting the RNG.
//
// These tests pin both halves of the fix so accidental edits to
// primedYggdrasilHat that strip either re-introduce the flake
// immediately.

// TestPrimedHat_SeedsNoiseRNG: noiseRNG must be deterministically
// seeded — same primed hat across invocations produces the same
// Intn sequence.
func TestPrimedHat_SeedsNoiseRNG(t *testing.T) {
	h1 := primedYggdrasilHat(3)
	h2 := primedYggdrasilHat(3)
	if h1.noiseRNG == nil || h2.noiseRNG == nil {
		t.Fatal("primedYggdrasilHat must initialize noiseRNG")
	}
	// 8 draws from each — pin to same sequence (proves deterministic seed).
	for i := 0; i < 8; i++ {
		a, b := h1.noiseRNG.Intn(100), h2.noiseRNG.Intn(100)
		if a != b {
			t.Errorf("draw %d: h1=%d h2=%d (RNG seeds not pinned)", i, a, b)
		}
	}
	// And pin to a fixed source-1 reference sequence so a future
	// change of seed surfaces here, not in some downstream test.
	want := rand.New(rand.NewSource(1))
	h3 := primedYggdrasilHat(3)
	for i := 0; i < 8; i++ {
		w := want.Intn(100)
		g := h3.noiseRNG.Intn(100)
		if g != w {
			t.Errorf("primed hat RNG not seeded with NewSource(1): draw %d got %d want %d", i, g, w)
		}
	}
}

// TestPrimedHat_ConfidenceThresholdTight: an unconfigured hat (no
// strategy profile) has confidenceThreshold = 0, which means
// selectAmongTop's margin is 1.0 — wide enough to pull
// combo-vs-control into the random-pool. primedYggdrasilHat must
// raise the threshold so tests see deterministic ordering by score.
func TestPrimedHat_ConfidenceThresholdTight(t *testing.T) {
	h := primedYggdrasilHat(3)
	if h.confidenceThreshold < 0.5 {
		t.Errorf("primedYggdrasilHat must raise confidenceThreshold (got %.2f) — wide margin lets the RNG flip tie-breaks",
			h.confidenceThreshold)
	}
}

// TestBestTarget_PrefersComboOpponent_Loop: the underlying flake
// was caught only by repeated invocation. Run the combo-preference
// scenario 50 times in-process — if either fix regresses (RNG seed
// dropped OR margin widened), the loop catches it locally instead
// of having to rely on `go test -count=N` catching the flake.
func TestBestTarget_PrefersComboOpponent_Loop(t *testing.T) {
	for i := 0; i < 50; i++ {
		gs := newTestGame(t, 3)
		gs.Turn = 5
		h := primedYggdrasilHat(3)
		gs.Seats[1].Life = 30
		gs.Seats[2].Life = 30

		h.recordOpponentPlay("tutor", "Demonic Tutor", 1, nil, gs.Turn)
		h.recordOpponentPlay("tutor", "Vampiric Tutor", 1, nil, gs.Turn)
		for _, name := range []string{"Dark Ritual", "Cabal Ritual", "Brainstorm"} {
			c := newTestCardMinimal(name, []string{"instant"}, 1, nil)
			h.recordOpponentPlay("cast", c.DisplayName(), 1, c, gs.Turn)
		}
		h.opponentHeldMana[1] = 3

		for _, name := range []string{"Doom Blade", "Swords to Plowshares", "Counterspell"} {
			c := newTestCardMinimal(name, []string{"instant"}, 2, nil)
			c.Types = append(c.Types, "oracle:destroy target creature")
			h.recordOpponentPlay("cast", c.DisplayName(), 2, c, gs.Turn)
		}

		h.classifyOpponent(gs, 1)
		h.classifyOpponent(gs, 2)

		atk := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 2)

		got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
		if got != 1 {
			t.Fatalf("iteration %d: expected seat 1 (combo), got %d", i, got)
		}
	}
}
