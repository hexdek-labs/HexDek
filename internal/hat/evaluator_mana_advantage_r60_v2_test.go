package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ManaAdvantage r60-v2 regressions covering three layered signals:
//
//   - Signal A: fast-mana extra-yield asymmetry. CountManaRocksAndLands
//     credits Sol Ring and Plains identically at 1; pre-r60 the
//     dimension was blind to the per-tap yield difference.
//   - Signal B: ramp-in-hand future-source preview. Hand-side ramp
//     spells / mana dorks / mana rocks count as fractional future
//     sources (0.5 each, capped at +2.0).
//   - Signal C: symmetric application — opp fast-mana penalizes us by
//     the same shape (single Sol Ring on opp side reads us-down, not
//     neutral as pre-r60).
//
// Existing handColorScrewPenalty / ritualFuelBonus / response-mana
// pre-r60 paths preserved — these are layered additions, not rewrites.

// ---------------------------------------------------------------------
// Shared builders
// ---------------------------------------------------------------------

// buildManaGame: 2-seat game, both at 40/40, no permanents.
func buildManaGame(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 2)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	gs.Active = 0
	return gs
}

func addPlains(seat *gameengine.Seat, n int) {
	for i := 0; i < n; i++ {
		newTestPermanent(seat, newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	}
}

func addRock(seat *gameengine.Seat, name string, oracle string, cmc int) {
	rock := gyCardWithOracle(name, []string{"artifact"}, cmc, oracle)
	newTestPermanent(seat, rock, 0, 0)
}

// ---------------------------------------------------------------------
// Signal A — fast-mana extra-yield (own side)
// ---------------------------------------------------------------------

func TestFastManaExtraYield_KnownCards(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{"Sol Ring", 1},
		{"Mana Crypt", 1},
		{"Mana Vault", 2},
		{"Grim Monolith", 2},
		{"Basalt Monolith", 2},
		{"Thran Dynamo", 1},
		{"Worn Powerstone", 1},
		{"Gilded Lotus", 2},
		{"Mox Sapphire", 0}, // tap-for-1: same as a land, no extra yield
		{"Plains", 0},        // baseline
		{"Black Lotus", 0},   // one-shot, intentionally excluded
	}
	seat := &gameengine.Seat{Idx: 0}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seat.Battlefield = nil
			newTestPermanent(seat, newTestCardMinimal(tc.name, []string{"artifact"}, 0, nil), 0, 0)
			got := fastManaExtraYield(seat)
			if got != tc.want {
				t.Errorf("%s: fastManaExtraYield = %d want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestScoreManaR60v2_SolRingBeatsPlains(t *testing.T) {
	gsRing := buildManaGame(t)
	addPlains(gsRing.Seats[0], 4)
	newTestPermanent(gsRing.Seats[0],
		gyCardWithOracle("Sol Ring", []string{"artifact"}, 1, "{t}: add {c}{c}"), 0, 0)
	addPlains(gsRing.Seats[1], 5)

	gsPlains := buildManaGame(t)
	addPlains(gsPlains.Seats[0], 5)
	addPlains(gsPlains.Seats[1], 5)

	ev := NewEvaluator(nil)
	withRing := ev.scoreMana(gsRing, 0)
	noRing := ev.scoreMana(gsPlains, 0)

	if withRing <= noRing {
		t.Errorf("Sol Ring should out-score same-count Plains; with=%.4f no=%.4f", withRing, noRing)
	}
	// Sol Ring = +1 extra source / 4 = +0.25 expected delta.
	delta := withRing - noRing
	if delta < 0.20 || delta > 0.30 {
		t.Errorf("Sol Ring delta off: %.4f (want ~0.25)", delta)
	}
}

func TestScoreManaR60v2_ManaVaultBiggerYieldThanSolRing(t *testing.T) {
	mk := func(rockName string) *gameengine.GameState {
		gs := buildManaGame(t)
		addPlains(gs.Seats[0], 3)
		newTestPermanent(gs.Seats[0],
			gyCardWithOracle(rockName, []string{"artifact"}, 1, "{t}: add {c}{c}"), 0, 0)
		addPlains(gs.Seats[1], 4)
		return gs
	}
	ev := NewEvaluator(nil)
	withSolRing := ev.scoreMana(mk("Sol Ring"), 0)    // +1
	withManaVault := ev.scoreMana(mk("Mana Vault"), 0) // +2
	if withManaVault <= withSolRing {
		t.Errorf("Mana Vault should out-score Sol Ring; vault=%.4f ring=%.4f",
			withManaVault, withSolRing)
	}
}

// ---------------------------------------------------------------------
// Signal C — symmetric opp fast-mana penalty
// ---------------------------------------------------------------------

func TestScoreManaR60v2_OppSolRingPenalizesUs(t *testing.T) {
	gsOppRing := buildManaGame(t)
	addPlains(gsOppRing.Seats[0], 5)
	addPlains(gsOppRing.Seats[1], 4)
	newTestPermanent(gsOppRing.Seats[1],
		gyCardWithOracle("Sol Ring", []string{"artifact"}, 1, "{t}: add {c}{c}"), 0, 0)

	gsEven := buildManaGame(t)
	addPlains(gsEven.Seats[0], 5)
	addPlains(gsEven.Seats[1], 5)

	ev := NewEvaluator(nil)
	oppHasRing := ev.scoreMana(gsOppRing, 0)
	even := ev.scoreMana(gsEven, 0)

	if oppHasRing >= even {
		t.Errorf("opp Sol Ring should worsen our score; opp_ring=%.4f even=%.4f",
			oppHasRing, even)
	}
}

func TestScoreManaR60v2_MutualSolRingCancels(t *testing.T) {
	gsBoth := buildManaGame(t)
	addPlains(gsBoth.Seats[0], 4)
	newTestPermanent(gsBoth.Seats[0],
		gyCardWithOracle("Sol Ring", []string{"artifact"}, 1, "{t}: add {c}{c}"), 0, 0)
	addPlains(gsBoth.Seats[1], 4)
	newTestPermanent(gsBoth.Seats[1],
		gyCardWithOracle("Sol Ring", []string{"artifact"}, 1, "{t}: add {c}{c}"), 0, 0)

	gsNeither := buildManaGame(t)
	addPlains(gsNeither.Seats[0], 5)
	addPlains(gsNeither.Seats[1], 5)

	ev := NewEvaluator(nil)
	both := ev.scoreMana(gsBoth, 0)
	neither := ev.scoreMana(gsNeither, 0)

	// Same raw counts on each side (5 each via 4 lands + 1 Sol Ring's
	// extra). The fast-mana bonus is symmetric, so net should be ~equal.
	delta := both - neither
	if delta < -0.05 || delta > 0.05 {
		t.Errorf("mutual Sol Ring should net-zero; both=%.4f neither=%.4f delta=%.4f",
			both, neither, delta)
	}
}

// ---------------------------------------------------------------------
// Signal B — ramp-in-hand future-source preview
// ---------------------------------------------------------------------

func TestRampInHandFutureSources_KnownPatterns(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
		types  []string
		cmc    int
		want   float64
	}{
		{"Cultivate", "search your library for up to two basic land cards, reveal them, put one onto the battlefield tapped and the other into your hand", []string{"sorcery"}, 3, 0.5},
		{"Rampant Growth", "search your library for a basic land card, put it onto the battlefield tapped", []string{"sorcery"}, 2, 0.5},
		{"Llanowar Elves", "{t}: add {g}.", []string{"creature"}, 1, 0.5},
		{"Sol Ring (in hand)", "{t}: add {c}{c}", []string{"artifact"}, 1, 0.5},
		{"Lightning Bolt", "deal 3 damage to any target", []string{"instant"}, 1, 0.0},
		{"Plain creature", "flying, vigilance", []string{"creature"}, 4, 0.0},
		{"Land card in hand", "{t}: add {g}", []string{"land", "basic"}, 0, 0.0}, // skip lands
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seat := &gameengine.Seat{Idx: 0}
			seat.Hand = []*gameengine.Card{gyCardWithOracle(tc.name, tc.types, tc.cmc, tc.oracle)}
			got := rampInHandFutureSources(seat)
			if got != tc.want {
				t.Errorf("%s: rampInHandFutureSources = %.2f want %.2f", tc.name, got, tc.want)
			}
		})
	}
}

func TestRampInHandFutureSources_Capped(t *testing.T) {
	// 10 ramp spells in hand → 10 * 0.5 = 5.0; should cap at 2.0.
	seat := &gameengine.Seat{Idx: 0}
	for i := 0; i < 10; i++ {
		seat.Hand = append(seat.Hand,
			gyCardWithOracle("Cultivate", []string{"sorcery"}, 3,
				"search your library for a basic land card, put it onto the battlefield tapped"))
	}
	got := rampInHandFutureSources(seat)
	if got != 2.0 {
		t.Errorf("ramp bonus not capped: got %.2f want 2.0", got)
	}
}

func TestScoreManaR60v2_RampInHandBeatsDeadCards(t *testing.T) {
	mk := func(ramp bool) *gameengine.GameState {
		gs := buildManaGame(t)
		addPlains(gs.Seats[0], 4)
		addPlains(gs.Seats[1], 4)
		if ramp {
			for i := 0; i < 3; i++ {
				gs.Seats[0].Hand = append(gs.Seats[0].Hand,
					gyCardWithOracle("Cultivate", []string{"sorcery"}, 3,
						"search your library for a basic land card, put it onto the battlefield tapped"))
			}
		} else {
			for i := 0; i < 3; i++ {
				gs.Seats[0].Hand = append(gs.Seats[0].Hand,
					gyCardWithOracle("Vanilla 3-drop", []string{"creature"}, 3,
						"flying"))
			}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	withRamp := ev.scoreMana(mk(true), 0)
	noRamp := ev.scoreMana(mk(false), 0)

	if withRamp <= noRamp {
		t.Errorf("ramp in hand should out-score dead cards; ramp=%.4f no=%.4f", withRamp, noRamp)
	}
	// 3 ramp × 0.5 / 4 = 0.375 expected (capped at 2.0/4 = 0.5).
	delta := withRamp - noRamp
	if delta < 0.30 || delta > 0.45 {
		t.Errorf("ramp-in-hand delta off: %.4f (want ~0.375)", delta)
	}
}

// ---------------------------------------------------------------------
// Cross-signal: all three additions stack additively
// ---------------------------------------------------------------------

func TestScoreManaR60v2_AllSignalsStack(t *testing.T) {
	// Us: 4 lands + Sol Ring (+1 fast) + 2 Cultivates in hand (+1.0 future).
	// Opp: 4 lands only.
	gs := buildManaGame(t)
	addPlains(gs.Seats[0], 4)
	newTestPermanent(gs.Seats[0],
		gyCardWithOracle("Sol Ring", []string{"artifact"}, 1, "{t}: add {c}{c}"), 0, 0)
	for i := 0; i < 2; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand,
			gyCardWithOracle("Cultivate", []string{"sorcery"}, 3,
				"search your library for a basic land card, put it onto the battlefield tapped"))
	}
	addPlains(gs.Seats[1], 4)

	gsBaseline := buildManaGame(t)
	addPlains(gsBaseline.Seats[0], 4)
	addPlains(gsBaseline.Seats[1], 4)

	ev := NewEvaluator(nil)
	loaded := ev.scoreMana(gs, 0)
	baseline := ev.scoreMana(gsBaseline, 0)

	// Sol Ring delta = 1/4 = 0.25 (we now have 6 effective vs 4 → 2/4 = 0.5,
	// minus baseline 0 → +0.5). Ramp delta = 2*0.5/4 = 0.25. Total ~0.75.
	delta := loaded - baseline
	if delta < 0.55 {
		t.Errorf("combined signals delta too small: %.4f (want ≥0.55)", delta)
	}
}
