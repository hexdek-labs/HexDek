package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// board_presence_evasion_r60_test.go — regressions for the per-creature
// evasion multiplier wired into effectiveBoardPower. Pre-r60 the
// BoardPresence dimension scored a flying 3/3 identically to a vanilla
// 3/3 (raw power 3 in both cases), missing that the flyer reliably
// connects through pods full of ground bodies while the vanilla mostly
// trades. See poker.go::evasionMultiplier.

// -----------------------------------------------------------------------------
// evasionMultiplier — unit
// -----------------------------------------------------------------------------

func TestEvasionMultiplier_NoEvasionBaseline(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 3, 3)

	if m := evasionMultiplier(p, 3); m != 1.0 {
		t.Errorf("vanilla 3/3 should have multiplier 1.00; got %.2f", m)
	}
}

func TestEvasionMultiplier_HardEvasionBumpsTwentyPercent(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, kw := range []string{"flying", "unblockable", "shadow", "horsemanship"} {
		p := newTestPermanent(gs.Seats[0], newTestCardMinimal(kw, []string{"creature"}, 2, nil), 3, 3)
		addKeyword(p, kw)
		if m := evasionMultiplier(p, 3); m != 1.20 {
			t.Errorf("%s 3/3 should have multiplier 1.20; got %.2f", kw, m)
		}
	}
}

func TestEvasionMultiplier_SoftEvasionBumpsTenPercent(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, kw := range []string{"menace", "fear", "intimidate", "skulk"} {
		p := newTestPermanent(gs.Seats[0], newTestCardMinimal(kw, []string{"creature"}, 2, nil), 3, 3)
		addKeyword(p, kw)
		if m := evasionMultiplier(p, 3); m != 1.10 {
			t.Errorf("%s 3/3 should have multiplier 1.10; got %.2f", kw, m)
		}
	}
}

// Hard evasion takes precedence over soft (typical case: a creature with
// flying is in the hard tier; even if it also has menace, the hard tier
// is what matters). The if/else-if structure ensures we don't double-
// dip the same "bypass blockers" component.
func TestEvasionMultiplier_HardExcludesSoftDoubleCount(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Skyguard", []string{"creature"}, 3, nil), 3, 3)
	addKeyword(p, "flying")
	addKeyword(p, "menace")
	if m := evasionMultiplier(p, 3); m != 1.20 {
		t.Errorf("flying+menace 3/3 should stay at 1.20 (hard tier wins, soft skipped); got %.2f", m)
	}
}

// Trample on a small body shouldn't qualify — chip damage past chumps
// only matters when the attacker outsizes the typical 1/1 chump.
func TestEvasionMultiplier_TrampleBelowFourPowerSkipped(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Tramp Elf", []string{"creature"}, 1, nil), 2, 2)
	addKeyword(p, "trample")
	if m := evasionMultiplier(p, 2); m != 1.0 {
		t.Errorf("trample 2/2 below 4-power threshold should stay at 1.00; got %.2f", m)
	}
}

func TestEvasionMultiplier_TrampleAtOrAboveFourPowerBumps(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Erhnam", []string{"creature"}, 4, nil), 4, 5)
	addKeyword(p, "trample")
	if m := evasionMultiplier(p, 4); m != 1.10 {
		t.Errorf("trample 4-power should bump 1.10; got %.2f", m)
	}
}

// Flying + trample stack (both signals — bypass and leak) up to the
// +0.30 cap. Stormtide-style designs.
func TestEvasionMultiplier_FlyingPlusTrampleStacksToCap(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Skyshroud", []string{"creature"}, 6, nil), 6, 6)
	addKeyword(p, "flying")
	addKeyword(p, "trample")
	if m := evasionMultiplier(p, 6); m != 1.30 {
		t.Errorf("flying+trample(6pw) should hit the +0.30 cap (1.30); got %.2f", m)
	}
}

// Nil-safe defensive guard — the helper is called from effectiveBoardPower
// in a loop that already skips nils, but defending here protects against
// future refactors.
func TestEvasionMultiplier_NilSafe(t *testing.T) {
	if m := evasionMultiplier(nil, 0); m != 1.0 {
		t.Errorf("nil permanent should return 1.00 (no-op); got %.2f", m)
	}
}

// -----------------------------------------------------------------------------
// effectiveBoardPower — integration
// -----------------------------------------------------------------------------

// A flying 3/3 contributes more raw "effective power" than a vanilla 3/3.
// 3 * 1.20 = 3.6 → rounds to 4; vs vanilla 3 * 1.00 = 3.
func TestEffectiveBoardPower_FlyingScoresAboveVanilla(t *testing.T) {
	mkGame := func(t *testing.T, flying bool) (*gameengine.GameState, *gameengine.Seat) {
		gs := newTestGame(t, 2)
		p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Body", []string{"creature"}, 2, nil), 3, 3)
		if flying {
			addKeyword(p, "flying")
		}
		return gs, gs.Seats[0]
	}

	fgs, fseat := mkGame(t, true)
	vgs, vseat := mkGame(t, false)

	flyingScore := effectiveBoardPower(fgs, fseat)
	vanillaScore := effectiveBoardPower(vgs, vseat)

	if !(flyingScore > vanillaScore) {
		t.Errorf("flying 3/3 (%d) should score above vanilla 3/3 (%d) under the r60 evasion multiplier",
			flyingScore, vanillaScore)
	}
	// Pin the math: 3 * 1.20 = 3.6 → round-half-up = 4.
	if flyingScore != 4 {
		t.Errorf("flying 3/3 should be 0.20 bonus → 3.6 → 4; got %d", flyingScore)
	}
}

// Tap discount stacks MULTIPLICATIVELY with evasion bonus — a tapped flyer
// still loses defensive value (0.7x) even though its evasion would help
// it connect when it untaps. 3 * 0.7 * 1.20 = 2.52 → rounds to 3.
func TestEffectiveBoardPower_TappedFlyerStillDiscounted(t *testing.T) {
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Skyguard", []string{"creature"}, 3, nil), 3, 3)
	addKeyword(p, "flying")
	p.Tapped = true

	got := effectiveBoardPower(gs, gs.Seats[0])
	if got != 3 {
		t.Errorf("tapped flying 3/3 should be 0.7*1.20*3 = 2.52 → 3; got %d", got)
	}
}

// scoreBoard integration headline: a flying board vs an equal-raw-power
// ground board should score higher under the r60 evasion multiplier.
// Setup: 3 flying 2/2s (raw 6) vs opponent's 3 vanilla 2/2s (raw 6).
// Pre-r60 both sides scored identically — postr60 the flyer board reads
// 6 * 1.20 = 7.2 → 7 effective vs the opp's 6, swinging the differential
// positive.
func TestScoreBoard_FlyingBoardScoresAboveVanillaPeer(t *testing.T) {
	mkGame := func(t *testing.T, ourFly bool) (*GameStateEvaluator, *gameengine.GameState) {
		gs := newTestGame(t, 2)
		for i := 0; i < 3; i++ {
			p := newTestPermanent(gs.Seats[0], newTestCardMinimal("Body", []string{"creature"}, 2, nil), 2, 2)
			if ourFly {
				addKeyword(p, "flying")
			}
		}
		for i := 0; i < 3; i++ {
			_ = newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
		}
		return &GameStateEvaluator{}, gs
	}

	fly, fgs := mkGame(t, true)
	van, vgs := mkGame(t, false)

	flyScore := fly.scoreBoard(fgs, 0)
	vanScore := van.scoreBoard(vgs, 0)

	if !(flyScore > vanScore) {
		t.Errorf("our-flying board (%.3f) should score above our-vanilla board (%.3f) "+
			"with the r60 evasion multiplier (opp constant)", flyScore, vanScore)
	}
}
