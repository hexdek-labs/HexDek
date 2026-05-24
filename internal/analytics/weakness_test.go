package analytics

import (
	"math"
	"testing"
)

// mkLoss builds a GameAnalysis containing exactly one player (the named
// commander) who lost. fn lets each test specialize the PlayerAnalysis +
// game-level fields before returning the analysis. Keeps the test bodies
// focused on what each criterion exercises rather than boilerplate.
func mkLoss(commander string, fn func(*GameAnalysis, *PlayerAnalysis)) *GameAnalysis {
	ga := &GameAnalysis{}
	pa := PlayerAnalysis{
		Seat:          0,
		CommanderName: commander,
		Won:           false,
	}
	ga.Players = []PlayerAnalysis{pa}
	if fn != nil {
		fn(ga, &ga.Players[0])
	}
	return ga
}

// dupLoss returns n copies of the same loss analysis, used by tests that
// just need the 10-game / 5-loss minimums to be met.
func dupLoss(n int, commander string, fn func(*GameAnalysis, *PlayerAnalysis)) []*GameAnalysis {
	out := make([]*GameAnalysis, n)
	for i := range out {
		out[i] = mkLoss(commander, fn)
	}
	return out
}

func approx(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// TestDeriveWeakness_BelowGameMinimum pins the 10-game threshold: with
// 9 games, DeriveWeakness returns nil (insufficient signal). This
// avoids overconfident verdicts on small samples.
func TestDeriveWeakness_BelowGameMinimum(t *testing.T) {
	games := dupLoss(9, "Atraxa", nil)
	if got := DeriveWeakness(games, "Atraxa"); got != nil {
		t.Errorf("9-game sample should return nil, got %+v", got)
	}
}

// TestDeriveWeakness_AtGameMinimum confirms the boundary: 10 games of
// losses qualifies for analysis (the 10-game gate is >= not >). With
// 10 unflagged losses, the signal struct should populate but every
// ratio is zero.
func TestDeriveWeakness_AtGameMinimum(t *testing.T) {
	games := dupLoss(10, "Atraxa", nil)
	got := DeriveWeakness(games, "Atraxa")
	if got == nil {
		t.Fatal("10-game sample should return non-nil")
	}
	if got.VulnerableToWipes != 0 || got.VulnerableToCounter != 0 ||
		got.SlowToClose != 0 || got.ManaScrew != 0 || got.OverExtends != 0 {
		t.Errorf("clean losses should yield zero ratios, got %+v", got)
	}
}

// TestDeriveWeakness_BelowLossMinimum pins the second gate: 10 games
// played but fewer than 5 are losses returns nil. The function divides
// by `losses`, so a too-small denominator would produce noise.
func TestDeriveWeakness_BelowLossMinimum(t *testing.T) {
	games := make([]*GameAnalysis, 10)
	for i := range games {
		games[i] = &GameAnalysis{
			Players: []PlayerAnalysis{
				// 4 losses + 6 wins; targeted commander only loses 4.
				{CommanderName: "Atraxa", Won: i >= 4},
			},
		}
	}
	if got := DeriveWeakness(games, "Atraxa"); got != nil {
		t.Errorf("4-loss sample (below 5-loss min) should return nil, got %+v", got)
	}
}

// TestDeriveWeakness_VulnerableToWipes pins the wipe criterion:
// FirstWipe > 0 AND TurnOfDeath - FirstWipe <= 3. Tight time-window
// between board wipe and death = vulnerable to wipes.
func TestDeriveWeakness_VulnerableToWipes(t *testing.T) {
	// 10 losses, all with wipe-then-death within 3 turns.
	games := dupLoss(10, "Atraxa", func(ga *GameAnalysis, pa *PlayerAnalysis) {
		ga.FirstWipe = 5
		pa.TurnOfDeath = 8 // exactly +3 — boundary
	})
	got := DeriveWeakness(games, "Atraxa")
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if !approx(got.VulnerableToWipes, 1.0) {
		t.Errorf("VulnerableToWipes = %f, want 1.0 (every loss matched the wipe pattern)",
			got.VulnerableToWipes)
	}

	// Same shape but death FOUR turns after wipe — outside window.
	games = dupLoss(10, "Atraxa", func(ga *GameAnalysis, pa *PlayerAnalysis) {
		ga.FirstWipe = 5
		pa.TurnOfDeath = 9 // +4 — outside the ≤3 window
	})
	got = DeriveWeakness(games, "Atraxa")
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if !approx(got.VulnerableToWipes, 0.0) {
		t.Errorf("VulnerableToWipes = %f, want 0.0 (death outside 3-turn window)",
			got.VulnerableToWipes)
	}
}

// TestDeriveWeakness_VulnerableToCounter pins the SpellsCountered ≥ 2
// criterion. One countered spell isn't a pattern; two+ is.
func TestDeriveWeakness_VulnerableToCounter(t *testing.T) {
	games := dupLoss(10, "Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
		pa.SpellsCountered = 2 // boundary
	})
	got := DeriveWeakness(games, "Atraxa")
	if !approx(got.VulnerableToCounter, 1.0) {
		t.Errorf("VulnerableToCounter at SpellsCountered=2 = %f, want 1.0",
			got.VulnerableToCounter)
	}

	games = dupLoss(10, "Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
		pa.SpellsCountered = 1 // below threshold
	})
	got = DeriveWeakness(games, "Atraxa")
	if !approx(got.VulnerableToCounter, 0.0) {
		t.Errorf("VulnerableToCounter at SpellsCountered=1 = %f, want 0.0",
			got.VulnerableToCounter)
	}
}

// TestDeriveWeakness_SlowToClose pins the StallIndicators.HitTurnCap
// criterion: every loss in a stalled game counts toward SlowToClose.
func TestDeriveWeakness_SlowToClose(t *testing.T) {
	games := dupLoss(10, "Atraxa", func(ga *GameAnalysis, _ *PlayerAnalysis) {
		ga.StallIndicators = &StallReport{HitTurnCap: true}
	})
	got := DeriveWeakness(games, "Atraxa")
	if !approx(got.SlowToClose, 1.0) {
		t.Errorf("SlowToClose with all-stalls = %f, want 1.0", got.SlowToClose)
	}

	// Stall report present but HitTurnCap false: doesn't count.
	games = dupLoss(10, "Atraxa", func(ga *GameAnalysis, _ *PlayerAnalysis) {
		ga.StallIndicators = &StallReport{HitTurnCap: false}
	})
	got = DeriveWeakness(games, "Atraxa")
	if !approx(got.SlowToClose, 0.0) {
		t.Errorf("SlowToClose with HitTurnCap=false = %f, want 0.0", got.SlowToClose)
	}
}

// TestDeriveWeakness_ManaScrew pins the mana-screw heuristic:
// LandsPlayed ≤ 3 AND TurnOfDeath > 0 AND TurnOfDeath ≤ 8. Early death
// with few lands played → mana issues.
func TestDeriveWeakness_ManaScrew(t *testing.T) {
	games := dupLoss(10, "Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
		pa.LandsPlayed = 3
		pa.TurnOfDeath = 8 // boundary
	})
	got := DeriveWeakness(games, "Atraxa")
	if !approx(got.ManaScrew, 1.0) {
		t.Errorf("ManaScrew at lands=3, turn=8 = %f, want 1.0", got.ManaScrew)
	}

	// LandsPlayed = 4 → above threshold.
	games = dupLoss(10, "Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
		pa.LandsPlayed = 4
		pa.TurnOfDeath = 8
	})
	got = DeriveWeakness(games, "Atraxa")
	if !approx(got.ManaScrew, 0.0) {
		t.Errorf("ManaScrew at lands=4 (above threshold) = %f, want 0.0", got.ManaScrew)
	}

	// TurnOfDeath = 9 → outside early-death window.
	games = dupLoss(10, "Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
		pa.LandsPlayed = 3
		pa.TurnOfDeath = 9
	})
	got = DeriveWeakness(games, "Atraxa")
	if !approx(got.ManaScrew, 0.0) {
		t.Errorf("ManaScrew at turn=9 (outside ≤8 window) = %f, want 0.0", got.ManaScrew)
	}

	// TurnOfDeath = 0 (survived) → not counted as mana-screw death.
	games = dupLoss(10, "Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
		pa.LandsPlayed = 2
		pa.TurnOfDeath = 0
	})
	got = DeriveWeakness(games, "Atraxa")
	if !approx(got.ManaScrew, 0.0) {
		t.Errorf("ManaScrew with TurnOfDeath=0 (survived) = %f, want 0.0",
			got.ManaScrew)
	}
}

// TestDeriveWeakness_OverExtends pins the all-in heuristic:
// PeakBoardSize ≥ 6 AND CardsInHand ≤ 1. Dumped the hand onto board
// and got punished.
func TestDeriveWeakness_OverExtends(t *testing.T) {
	games := dupLoss(10, "Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
		pa.PeakBoardSize = 6 // boundary
		pa.CardsInHand = 1   // boundary
	})
	got := DeriveWeakness(games, "Atraxa")
	if !approx(got.OverExtends, 1.0) {
		t.Errorf("OverExtends at board=6, hand=1 = %f, want 1.0", got.OverExtends)
	}

	// Board=5 → below threshold.
	games = dupLoss(10, "Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
		pa.PeakBoardSize = 5
		pa.CardsInHand = 1
	})
	got = DeriveWeakness(games, "Atraxa")
	if !approx(got.OverExtends, 0.0) {
		t.Errorf("OverExtends at board=5 = %f, want 0.0", got.OverExtends)
	}

	// Hand=2 → above threshold.
	games = dupLoss(10, "Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
		pa.PeakBoardSize = 7
		pa.CardsInHand = 2
	})
	got = DeriveWeakness(games, "Atraxa")
	if !approx(got.OverExtends, 0.0) {
		t.Errorf("OverExtends at hand=2 = %f, want 0.0", got.OverExtends)
	}
}

// TestDeriveWeakness_PartialFlags is the load-bearing realism case:
// 10 losses split 5-3-2 between wipe-vulnerable, counter-vulnerable,
// and neither. Ratios must match the hand-computed proportions exactly.
func TestDeriveWeakness_PartialFlags(t *testing.T) {
	games := make([]*GameAnalysis, 10)
	// 5 wipe-pattern losses.
	for i := 0; i < 5; i++ {
		games[i] = mkLoss("Atraxa", func(ga *GameAnalysis, pa *PlayerAnalysis) {
			ga.FirstWipe = 4
			pa.TurnOfDeath = 6
		})
	}
	// 3 counter-pattern losses.
	for i := 5; i < 8; i++ {
		games[i] = mkLoss("Atraxa", func(_ *GameAnalysis, pa *PlayerAnalysis) {
			pa.SpellsCountered = 3
		})
	}
	// 2 plain losses.
	for i := 8; i < 10; i++ {
		games[i] = mkLoss("Atraxa", nil)
	}

	got := DeriveWeakness(games, "Atraxa")
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if !approx(got.VulnerableToWipes, 0.5) {
		t.Errorf("VulnerableToWipes = %f, want 0.5 (5/10)", got.VulnerableToWipes)
	}
	if !approx(got.VulnerableToCounter, 0.3) {
		t.Errorf("VulnerableToCounter = %f, want 0.3 (3/10)", got.VulnerableToCounter)
	}
	if !approx(got.SlowToClose, 0.0) {
		t.Errorf("SlowToClose = %f, want 0.0", got.SlowToClose)
	}
}

// TestDeriveWeakness_OnlyTargetCommanderCounts confirms the commander
// filter: a 10-game sample where ONLY 4 games featured "Atraxa" returns
// nil (below the 5-loss floor for Atraxa specifically), even though
// the total games >= 10 minimum.
func TestDeriveWeakness_OnlyTargetCommanderCounts(t *testing.T) {
	games := make([]*GameAnalysis, 10)
	for i := range games {
		commander := "Atraxa"
		if i >= 4 {
			commander = "Krenko"
		}
		games[i] = &GameAnalysis{
			Players: []PlayerAnalysis{
				{CommanderName: commander, Won: false},
			},
		}
	}
	if got := DeriveWeakness(games, "Atraxa"); got != nil {
		t.Errorf("Atraxa appeared in 4 losses (below min); expected nil, got %+v", got)
	}
	// Krenko appears in 6 losses — meets both gates.
	if got := DeriveWeakness(games, "Krenko"); got == nil {
		t.Error("Krenko (6 losses) should clear the loss floor")
	}
}

// TestDeriveWeakness_WinsExcluded confirms the Won guard: a 10-game
// sample where all games feature the target commander but the
// commander WON every game produces nil (zero losses, below 5-loss
// floor). The Won branch in the inner loop is the gate.
func TestDeriveWeakness_WinsExcluded(t *testing.T) {
	games := make([]*GameAnalysis, 10)
	for i := range games {
		games[i] = &GameAnalysis{
			Players: []PlayerAnalysis{
				{CommanderName: "Atraxa", Won: true},
			},
		}
	}
	if got := DeriveWeakness(games, "Atraxa"); got != nil {
		t.Errorf("commander that always wins should have no weakness signal, got %+v", got)
	}
}

// TestDeriveWeakness_NilGamesSkipped pins the nil-element guard:
// the loop's `if ga == nil { continue }` keeps a contaminated slice
// from panicking.
func TestDeriveWeakness_NilGamesSkipped(t *testing.T) {
	games := []*GameAnalysis{
		nil, nil, nil, nil, nil,
		mkLoss("Atraxa", nil), mkLoss("Atraxa", nil),
		mkLoss("Atraxa", nil), mkLoss("Atraxa", nil),
		mkLoss("Atraxa", nil), mkLoss("Atraxa", nil),
	}
	// 5 nils + 6 losses. Total len = 11 ≥ 10 game floor. losses = 6 ≥ 5
	// loss floor. Should return non-nil without panicking.
	got := DeriveWeakness(games, "Atraxa")
	if got == nil {
		t.Error("expected non-nil after skipping nils")
	}
}
