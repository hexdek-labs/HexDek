package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// floatClose is a local epsilon comparator to keep this file self-
// contained (other r60 test files have similar helpers that may not be
// merged yet).
func floatClose(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// Pins the CommanderProgress upgrades: castability-weighted tax penalty
// + concentrated commander damage signal.

// setupCommanderGame builds a 4-seat Commander game with seat 0's
// commander in the command zone, configurable tax + lands + commander CMC.
func setupCommanderGame(t *testing.T, cmdrCMC, tax, lands int) *gameengine.GameState {
	gs := newTestGame(t, 4)
	gs.CommanderFormat = true
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}

	cmdr := newTestCardMinimal("Test Commander", []string{"creature", "legendary"}, cmdrCMC, nil)
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, cmdr)
	gs.Seats[0].CommanderNames = []string{"Test Commander"}
	// scoreCommander reads CommanderCastCounts[name] as the raw `tax`
	// multiplier — we pass it through directly.
	gs.Seats[0].CommanderCastCounts = map[string]int{"Test Commander": tax}

	// Lands on seat 0's battlefield.
	for i := 0; i < lands; i++ {
		newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	}
	return gs
}

// TestScoreCommander_UncastableInflatesTaxPenalty: a commander whose
// total cost exceeds 1.5x land count should be penalized MORE than
// the linear baseline.
func TestScoreCommander_UncastableInflatesTaxPenalty(t *testing.T) {
	// 7-CMC commander + 4 tax = total 11; only 5 lands → ratio 2.2 → 1.6x.
	gs := setupCommanderGame(t, 7, 4, 5)
	ev := NewEvaluator(nil)
	uncastable := ev.scoreCommander(gs, 0)

	// Comparable seat with the same tax but abundant lands → ratio < 0.5.
	gs2 := setupCommanderGame(t, 3, 4, 14)
	castable := ev.scoreCommander(gs2, 0)

	if uncastable >= castable {
		t.Errorf("uncastable commander (%.3f) should score more negatively than abundantly-castable (%.3f)",
			uncastable, castable)
	}
}

// TestScoreCommander_AffordableDampensTax: when commander total cost
// is <= 0.5 * landCount, castabilityFactor drops to 0.6 — tax matters
// less because we can keep recasting.
func TestScoreCommander_AffordableDampensTax(t *testing.T) {
	// 2 CMC + 4 tax = 6; 14 lands → ratio 0.43 → 0.6x factor.
	gs := setupCommanderGame(t, 2, 4, 14)
	ev := NewEvaluator(nil)
	soft := ev.scoreCommander(gs, 0)

	// Compute the un-softened baseline manually: -tax * 0.15 * 1.0 = -0.6.
	// Softened: -0.6 * 0.6 = -0.36. Score includes only the tax penalty
	// (no commander on field, no damage), so:
	want := -float64(4) * 0.15 * 0.6
	if !floatClose(soft, want) {
		t.Errorf("affordable-commander tax: got %.4f, want %.4f", soft, want)
	}
}

// TestScoreCommander_NoLandsMaxesPenalty: zero lands + nonzero cost →
// castabilityFactor 1.6 (stuck).
func TestScoreCommander_NoLandsMaxesPenalty(t *testing.T) {
	gs := setupCommanderGame(t, 4, 2, 0)
	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	want := -float64(2) * 0.15 * 1.6
	if !floatClose(got, want) {
		t.Errorf("zero-lands tax: got %.4f, want %.4f", got, want)
	}
}

// TestScoreCommander_ConcentratedDamageBeatsSpread: 20 damage on one
// opp should score higher than 5x4 spread, even though the sums match.
func TestScoreCommander_ConcentratedDamageBeatsSpread(t *testing.T) {
	build := func(perOpp []int) *gameengine.GameState {
		gs := newTestGame(t, 4)
		gs.CommanderFormat = true
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].CommanderNames = []string{"Voltron Cmdr"}
		for i, dmg := range perOpp {
			oppIdx := i + 1
			if dmg == 0 {
				continue
			}
			if gs.Seats[oppIdx].CommanderDamage == nil {
				gs.Seats[oppIdx].CommanderDamage = map[int]map[string]int{}
			}
			gs.Seats[oppIdx].CommanderDamage[0] = map[string]int{"Voltron Cmdr": dmg}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	concentrated := ev.scoreCommander(build([]int{20, 0, 0}), 0)
	spread := ev.scoreCommander(build([]int{5, 5, 5}), 0)

	if concentrated <= spread {
		t.Errorf("concentrated 20-on-one (%.3f) should out-score spread 5×3=15 (%.3f)",
			concentrated, spread)
	}
}

// TestScoreCommander_DamageConvexity: damage near lethal should hurt
// more per-point than damage far from it. Delta(15→20) > delta(5→10).
func TestScoreCommander_DamageConvexity(t *testing.T) {
	build := func(dmg int) *gameengine.GameState {
		gs := newTestGame(t, 4)
		gs.CommanderFormat = true
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].CommanderNames = []string{"Voltron Cmdr"}
		gs.Seats[1].CommanderDamage = map[int]map[string]int{
			0: {"Voltron Cmdr": dmg},
		}
		return gs
	}

	ev := NewEvaluator(nil)
	at5 := ev.scoreCommander(build(5), 0)
	at10 := ev.scoreCommander(build(10), 0)
	at15 := ev.scoreCommander(build(15), 0)
	at20 := ev.scoreCommander(build(20), 0)

	d510 := at10 - at5
	d1520 := at20 - at15

	if d1520 <= d510 {
		t.Errorf("damage convexity: delta(15→20)=%.4f should exceed delta(5→10)=%.4f",
			d1520, d510)
	}
}

// TestScoreCommander_LethalDamageSaturates: 25 damage caps at the same
// score as 21 (per-opponent lethal threshold; extra damage is wasted).
func TestScoreCommander_LethalDamageSaturates(t *testing.T) {
	build := func(dmg int) *gameengine.GameState {
		gs := newTestGame(t, 4)
		gs.CommanderFormat = true
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].CommanderNames = []string{"Voltron Cmdr"}
		gs.Seats[1].CommanderDamage = map[int]map[string]int{
			0: {"Voltron Cmdr": dmg},
		}
		return gs
	}

	ev := NewEvaluator(nil)
	at21 := ev.scoreCommander(build(21), 0)
	at25 := ev.scoreCommander(build(25), 0)

	if at21 != at25 {
		t.Errorf("damage should saturate at 21: at21=%.4f at25=%.4f", at21, at25)
	}
}

// TestScoreCommander_SpreadDamageContributes: spread damage should
// still register a nonzero ambient-pressure signal (it's not nothing).
func TestScoreCommander_SpreadDamageContributes(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.CommanderFormat = true
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	gs.Seats[0].CommanderNames = []string{"Voltron Cmdr"}
	// 5 on opp1 + 5 on opp2 — max=5, spread=5.
	gs.Seats[1].CommanderDamage = map[int]map[string]int{0: {"Voltron Cmdr": 5}}
	gs.Seats[2].CommanderDamage = map[int]map[string]int{0: {"Voltron Cmdr": 5}}

	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)

	// Concentrated portion: p=5/21, score = p² + 0.2p.
	// Spread portion: 5/21 * 0.15.
	p := 5.0 / 21.0
	want := p*p + p*0.2 + (5.0/21.0)*0.15
	if !floatClose(got, want) {
		t.Errorf("spread+concentrated: got %.4f, want %.4f", got, want)
	}
}
