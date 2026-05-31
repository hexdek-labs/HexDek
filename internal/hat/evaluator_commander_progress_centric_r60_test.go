package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// CommanderProgress r60-extension regressions:
//
//   - Facet A: IsCommanderCentric (Freya flag) routes into both the
//     on-field synergyBonus path AND the command-zone tax-penalty
//     path. Pre-r60 the dimension only escalated on raw
//     CommanderSynergy > 0.5 — a Voltron / engine-commander deck with
//     CommanderSynergy = 0.30 (e.g. an engine commander whose
//     gameplan-around tagging Freya marked centric via engine-phrase
//     heuristic, not synergy density) got no escalation at all.
//
//   - Facet B: discrete partial-Voltron thresholds on max-per-opp
//     commander damage at 7 / 14 / 18. The existing convex p² + 0.2p
//     ramps smoothly; the threshold flats encode the qualitative
//     "first knock" / "two-thirds clock" / "one-swing-from-lethal"
//     signposts that drive human play and MCTS prioritization.

const centricEps = 1e-9

func centricFloatClose(a, b float64) bool {
	return math.Abs(a-b) < centricEps
}

// buildCentricGame: 4-seat commander game with seat 0's commander in
// command zone, no on-board commander, optional IsCommanderCentric +
// CommanderSynergy on Strategy.
func buildCentricGame(t *testing.T, cmdrCMC, tax, lands int) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 4)
	gs.CommanderFormat = true
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	cmdr := newTestCardMinimal("Centric Cmdr", []string{"creature", "legendary"}, cmdrCMC, nil)
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, cmdr)
	gs.Seats[0].CommanderNames = []string{"Centric Cmdr"}
	gs.Seats[0].CommanderCastCounts = map[string]int{"Centric Cmdr": tax}
	for i := 0; i < lands; i++ {
		newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	}
	return gs
}

// ---------------------------------------------------------------------
// Facet A — IsCommanderCentric tax escalation
// ---------------------------------------------------------------------

// TestScoreCommander_IsCommanderCentric_TaxEscalatesPast0_5SynergyGate:
// A deck with CommanderSynergy=0.30 (below the 0.5 raw-synergy gate)
// but IsCommanderCentric=true should still get heavier tax penalty
// than the default 0.15-per-tax path.
func TestScoreCommander_IsCommanderCentric_TaxEscalatesPast0_5SynergyGate(t *testing.T) {
	gs := buildCentricGame(t, 3, 4, 6) // 3 CMC + 4 tax = 7 total, 6 lands → ratio 1.17 → factor 1.2

	ev := NewEvaluator(&StrategyProfile{
		IsCommanderCentric: true,
		CommanderSynergy:   0.30, // below the 0.5 raw-synergy gate
	})
	centric := ev.scoreCommander(gs, 0)

	gsBaseline := buildCentricGame(t, 3, 4, 6)
	evDefault := NewEvaluator(nil)
	baseline := evDefault.scoreCommander(gsBaseline, 0)

	// Centric path uses taxPenalty=0.28; baseline path uses 0.15.
	// Both apply the same 1.2 castabilityFactor → centric = -0.28*4*1.2 = -1.344;
	// baseline = -0.15*4*1.2 = -0.72. Centric must be more negative.
	if centric >= baseline {
		t.Errorf("IsCommanderCentric should escalate tax penalty even when CommanderSynergy<0.5; centric=%.4f baseline=%.4f",
			centric, baseline)
	}
	wantCentric := -0.28 * 4 * 1.2
	if !centricFloatClose(centric, wantCentric) {
		t.Errorf("centric tax: got %.4f, want %.4f", centric, wantCentric)
	}
}

// TestScoreCommander_IsCommanderCentric_BeatsSynergyGate: when both
// flags are set, IsCommanderCentric (0.28) wins over the synergy>0.5
// path (0.20) since the switch checks IsCommanderCentric first.
func TestScoreCommander_IsCommanderCentric_BeatsSynergyGate(t *testing.T) {
	gs := buildCentricGame(t, 2, 3, 10) // 2+3=5, 10 lands → ratio 0.5 → factor 1.0
	ev := NewEvaluator(&StrategyProfile{
		IsCommanderCentric: true,
		CommanderSynergy:   0.85, // would normally trip the 0.20 path
	})
	got := ev.scoreCommander(gs, 0)

	// taxPenalty = 0.28, castabilityFactor = 1.0 (ratio 0.5 is the BOUNDARY —
	// the switch uses < 0.5 so 0.5 itself falls through to the default 1.0).
	want := -0.28 * 3 * 1.0
	if !centricFloatClose(got, want) {
		t.Errorf("centric beats synergy gate: got %.4f, want %.4f", got, want)
	}
}

// ---------------------------------------------------------------------
// Facet A continued — IsCommanderCentric on-field synergyBonus floor
// ---------------------------------------------------------------------

// TestScoreCommander_IsCommanderCentric_OnFieldBonusFloor: an engine-
// commander deck (CommanderSynergy=0.30, below the 0.5 gate) with
// IsCommanderCentric=true must register the on-field bonus at the
// 0.55 floor — not the pre-r60 0.30 default.
func TestScoreCommander_IsCommanderCentric_OnFieldBonusFloor(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.CommanderFormat = true
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	cmdr := newTestCardMinimal("Engine Cmdr", []string{"creature", "legendary"}, 3, nil)
	gs.Seats[0].CommanderNames = []string{"Engine Cmdr"}
	newTestPermanent(gs.Seats[0], cmdr, 2, 2)

	evCentric := NewEvaluator(&StrategyProfile{
		IsCommanderCentric: true,
		CommanderSynergy:   0.30,
	})
	centric := evCentric.scoreCommander(gs, 0)

	evDefault := NewEvaluator(nil)
	def := evDefault.scoreCommander(gs, 0)

	// Centric floor: 0.55 + 0.30*0.30 = 0.64. Default: 0.30.
	wantCentric := 0.55 + 0.30*0.30
	wantDefault := 0.30

	if !centricFloatClose(centric, wantCentric) {
		t.Errorf("centric on-field bonus: got %.4f, want %.4f", centric, wantCentric)
	}
	if !centricFloatClose(def, wantDefault) {
		t.Errorf("default on-field bonus: got %.4f, want %.4f", def, wantDefault)
	}
	if centric <= def {
		t.Errorf("centric on-field bonus (%.4f) must exceed default (%.4f)", centric, def)
	}
}

// ---------------------------------------------------------------------
// Facet B — partial-Voltron damage thresholds
// ---------------------------------------------------------------------

func buildDamageGame(t *testing.T, dmg int) *gameengine.GameState {
	t.Helper()
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

// TestScoreCommander_ThresholdCrossing_7: the 6→7 step should produce
// a discrete jump (threshold bonus +0.08) beyond what the smooth
// convex curve alone produces for the same +1 increment elsewhere.
func TestScoreCommander_ThresholdCrossing_7(t *testing.T) {
	ev := NewEvaluator(nil)
	at6 := ev.scoreCommander(buildDamageGame(t, 6), 0)
	at7 := ev.scoreCommander(buildDamageGame(t, 7), 0)

	// Smooth-curve baseline +1 step: 8→9 (well inside the 7-threshold zone).
	at8 := ev.scoreCommander(buildDamageGame(t, 8), 0)
	at9 := ev.scoreCommander(buildDamageGame(t, 9), 0)

	thresholdStep := at7 - at6
	smoothStep := at9 - at8

	// Threshold step should exceed the smooth step by ~0.08.
	if thresholdStep <= smoothStep+0.05 {
		t.Errorf("7-threshold not discrete enough: 6→7 step=%.4f vs 8→9 smooth step=%.4f (want delta ≥0.05)",
			thresholdStep, smoothStep)
	}
}

// TestScoreCommander_ThresholdCrossing_14: 13→14 step is the "two-thirds"
// jump — opponent now needs to answer or die in two combats.
func TestScoreCommander_ThresholdCrossing_14(t *testing.T) {
	ev := NewEvaluator(nil)
	at13 := ev.scoreCommander(buildDamageGame(t, 13), 0)
	at14 := ev.scoreCommander(buildDamageGame(t, 14), 0)

	at11 := ev.scoreCommander(buildDamageGame(t, 11), 0)
	at12 := ev.scoreCommander(buildDamageGame(t, 12), 0)

	thresholdStep := at14 - at13
	smoothStep := at12 - at11

	if thresholdStep <= smoothStep+0.08 {
		t.Errorf("14-threshold not discrete enough: 13→14=%.4f vs 11→12 smooth=%.4f",
			thresholdStep, smoothStep)
	}
}

// TestScoreCommander_ThresholdCrossing_18: 17→18 step is the "one-swing-
// from-lethal" jump.
func TestScoreCommander_ThresholdCrossing_18(t *testing.T) {
	ev := NewEvaluator(nil)
	at17 := ev.scoreCommander(buildDamageGame(t, 17), 0)
	at18 := ev.scoreCommander(buildDamageGame(t, 18), 0)

	at15 := ev.scoreCommander(buildDamageGame(t, 15), 0)
	at16 := ev.scoreCommander(buildDamageGame(t, 16), 0)

	thresholdStep := at18 - at17
	smoothStep := at16 - at15

	if thresholdStep <= smoothStep+0.10 {
		t.Errorf("18-threshold not discrete enough: 17→18=%.4f vs 15→16 smooth=%.4f",
			thresholdStep, smoothStep)
	}
}

// TestScoreCommander_ThresholdsStack: at 18 commander damage, all
// three thresholds have fired — total bonus from the discrete block
// should be 0.35 (0.08 + 0.12 + 0.15).
func TestScoreCommander_ThresholdsStack(t *testing.T) {
	ev := NewEvaluator(nil)
	at18 := ev.scoreCommander(buildDamageGame(t, 18), 0)
	at6 := ev.scoreCommander(buildDamageGame(t, 6), 0) // no thresholds yet

	// Threshold block contribution at 18: +0.35 vs at-6 zero.
	// The convex curve also rises (p=6/21≈0.286 → 0.139 vs p=18/21≈0.857 → 0.906).
	// We're isolating the threshold contribution alone via the explicit
	// math: total delta minus convex-curve delta should be ≥ 0.30 (allowing
	// for small floating-point drift below the strict 0.35).
	p6 := 6.0 / 21.0
	p18 := 18.0 / 21.0
	convexDelta := (p18*p18 + p18*0.2) - (p6*p6 + p6*0.2)

	thresholdContribution := (at18 - at6) - convexDelta
	if thresholdContribution < 0.30 {
		t.Errorf("threshold contribution at 18 too small: got %.4f (want ≥0.30 of the 0.35 budget)",
			thresholdContribution)
	}
	if thresholdContribution > 0.40 {
		t.Errorf("threshold contribution at 18 too large: got %.4f (want ≤0.40)",
			thresholdContribution)
	}
}

// TestScoreCommander_ThresholdsRespectMaxNotSpread: thresholds key off
// max-per-opponent, NOT sum across opponents. 5+5+5 = 15 sum but max=5
// should NOT trigger ANY threshold; 7+0+0 = 7 sum but max=7 SHOULD
// trigger the 7-floor.
func TestScoreCommander_ThresholdsRespectMaxNotSpread(t *testing.T) {
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
			gs.Seats[oppIdx].CommanderDamage = map[int]map[string]int{
				0: {"Voltron Cmdr": dmg},
			}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	spread := ev.scoreCommander(build([]int{5, 5, 5}), 0)        // max=5, no threshold
	concentrated := ev.scoreCommander(build([]int{7, 0, 0}), 0) // max=7, +0.08 floor

	// Concentrated should be HIGHER despite lower sum (15 vs 7) because
	// the threshold cross matters MORE than the ambient sum delta.
	if concentrated <= spread {
		t.Errorf("threshold should fire on max not spread: concentrated(7on1)=%.4f spread(5x3)=%.4f",
			concentrated, spread)
	}
}

// TestScoreCommander_ThresholdsOnlyInCommanderFormat: non-commander
// games skip scoreCommander entirely (returns 0 at the top). Confirms
// the threshold block doesn't accidentally fire in a non-EDH context.
func TestScoreCommander_ThresholdsOnlyInCommanderFormat(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.CommanderFormat = false // explicitly off
	gs.Seats[0].CommanderNames = []string{"Cmdr"}
	gs.Seats[1].CommanderDamage = map[int]map[string]int{
		0: {"Cmdr": 21},
	}
	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	if got != 0 {
		t.Errorf("scoreCommander should return 0 outside commander format; got %.4f", got)
	}
}
