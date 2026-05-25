package heimdall

import (
	"math"
	"testing"
)

func evoApprox(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func findEvo(rows []ArchetypeEvolution, slug string) *ArchetypeEvolution {
	for i := range rows {
		if rows[i].Archetype == slug {
			return &rows[i]
		}
	}
	return nil
}

// TestComputeMetaEvolution_Empty pins the no-data envelope.
func TestComputeMetaEvolution_Empty(t *testing.T) {
	out := ComputeMetaEvolution(nil, 1_000_000, 4)
	if out.Archetypes == nil || out.BiggestPlayRateGainers == nil ||
		out.BiggestPlayRateLosers == nil || out.BiggestWinRateGainers == nil ||
		out.BiggestWinRateLosers == nil {
		t.Errorf("slices should be non-nil empty: %+v", out)
	}
	if out.Weeks != 4 || out.TotalGames != 0 {
		t.Errorf("envelope: %+v", out)
	}
}

// TestComputeMetaEvolution_OverallPlayRate pins that OverallPlayRate
// = archetype's total games / window total games.
func TestComputeMetaEvolution_OverallPlayRate(t *testing.T) {
	ref := int64(20 * weekSec)
	mk := func(arch string, n int) []MetaGameInput {
		out := make([]MetaGameInput, n)
		for i := range out {
			out[i] = MetaGameInput{FinishedAt: ref - 2*weekSec, Archetype: arch, Won: false}
		}
		return out
	}
	var games []MetaGameInput
	games = append(games, mk("control", 30)...)
	games = append(games, mk("combo", 10)...)
	games = append(games, mk("voltron", 60)...)
	out := ComputeMetaEvolution(games, ref, 4)
	if out.TotalGames != 100 {
		t.Errorf("TotalGames = %d, want 100", out.TotalGames)
	}
	if c := findEvo(out.Archetypes, "control"); c == nil || !evoApprox(c.OverallPlayRate, 0.3) {
		t.Errorf("control play rate: %+v, want 0.3", c)
	}
	if c := findEvo(out.Archetypes, "voltron"); c == nil || !evoApprox(c.OverallPlayRate, 0.6) {
		t.Errorf("voltron play rate: %+v, want 0.6", c)
	}
	if c := findEvo(out.Archetypes, "combo"); c == nil || !evoApprox(c.OverallPlayRate, 0.1) {
		t.Errorf("combo play rate: %+v, want 0.1", c)
	}
}

// TestComputeMetaEvolution_PlayRateUpDown pins the play-rate delta
// classification: an archetype going from 10% → 30% of the meta is
// "up"; one going from 30% → 10% is "down".
func TestComputeMetaEvolution_PlayRateUpDown(t *testing.T) {
	ref := int64(20 * weekSec)
	mk := func(arch string, weekIdx, n int) []MetaGameInput {
		ts := ref - int64(4-weekIdx)*weekSec
		out := make([]MetaGameInput, n)
		for i := range out {
			out[i] = MetaGameInput{FinishedAt: ts, Archetype: arch, Won: false}
		}
		return out
	}
	var games []MetaGameInput
	// Prior half (weeks 0+1): control 30, combo 70 (total 100)
	games = append(games, mk("control", 0, 30)...)
	games = append(games, mk("combo", 0, 70)...)
	// Recent half (weeks 2+3): control 70, combo 30 (total 100)
	games = append(games, mk("control", 2, 70)...)
	games = append(games, mk("combo", 2, 30)...)

	out := ComputeMetaEvolution(games, ref, 4)
	control := findEvo(out.Archetypes, "control")
	combo := findEvo(out.Archetypes, "combo")
	if control == nil || combo == nil {
		t.Fatalf("missing archetypes")
	}
	// control: prior 30/100 = 0.3, recent 70/100 = 0.7, Δ = +0.4
	if !evoApprox(control.PlayRateDelta, 0.4) {
		t.Errorf("control play-rate Δ = %.3f, want 0.4", control.PlayRateDelta)
	}
	if control.PlayRateDirection != "up" {
		t.Errorf("control play-rate dir = %q, want 'up'", control.PlayRateDirection)
	}
	if !evoApprox(combo.PlayRateDelta, -0.4) {
		t.Errorf("combo play-rate Δ = %.3f, want -0.4", combo.PlayRateDelta)
	}
	if combo.PlayRateDirection != "down" {
		t.Errorf("combo play-rate dir = %q, want 'down'", combo.PlayRateDirection)
	}
}

// TestComputeMetaEvolution_WinAndPlayRatesIndependent pins that an
// archetype can be flat on win rate but up on play rate (and vice
// versa) — the two axes classify independently.
func TestComputeMetaEvolution_WinAndPlayRatesIndependent(t *testing.T) {
	ref := int64(20 * weekSec)
	mk := func(arch string, weekIdx, n int, wins int) []MetaGameInput {
		ts := ref - int64(4-weekIdx)*weekSec
		out := make([]MetaGameInput, n)
		for i := range out {
			out[i] = MetaGameInput{FinishedAt: ts, Archetype: arch, Won: i < wins}
		}
		return out
	}
	// Build a game pool where:
	//   - "growing" goes from 10 games (5w) prior to 50 games (25w)
	//     recent. Play rate moves up; win rate stays at 50%.
	//   - "noise" fills out the meta so totals are sensible: 90 prior,
	//     50 recent.
	var games []MetaGameInput
	games = append(games, mk("growing", 0, 10, 5)...)
	games = append(games, mk("noise", 0, 90, 45)...)
	games = append(games, mk("growing", 2, 50, 25)...)
	games = append(games, mk("noise", 2, 50, 25)...)

	out := ComputeMetaEvolution(games, ref, 4)
	g := findEvo(out.Archetypes, "growing")
	if g == nil {
		t.Fatal("missing growing")
	}
	if g.PlayRateDirection != "up" {
		t.Errorf("growing play-rate dir = %q, want 'up' (Δ=%.3f)", g.PlayRateDirection, g.PlayRateDelta)
	}
	if g.WinRateDirection != "flat" {
		t.Errorf("growing win-rate dir = %q, want 'flat' (Δ=%.3f)", g.WinRateDirection, g.WinRateDelta)
	}
}

// TestComputeMetaEvolution_InsufficientWhenBelowMinSamples pins the
// per-axis insufficient_data label when either half has <10 games.
func TestComputeMetaEvolution_InsufficientWhenBelowMinSamples(t *testing.T) {
	ref := int64(20 * weekSec)
	games := []MetaGameInput{
		// 4 games total, split 2/2 — under the 10-sample floor.
		{FinishedAt: ref - 3*weekSec, Archetype: "spike", Won: true},
		{FinishedAt: ref - 3*weekSec, Archetype: "spike", Won: false},
		{FinishedAt: ref - 1*weekSec, Archetype: "spike", Won: true},
		{FinishedAt: ref - 1*weekSec, Archetype: "spike", Won: false},
	}
	out := ComputeMetaEvolution(games, ref, 4)
	s := findEvo(out.Archetypes, "spike")
	if s == nil {
		t.Fatal("missing spike")
	}
	if s.PlayRateDirection != "insufficient_data" || s.WinRateDirection != "insufficient_data" {
		t.Errorf("directions: play=%q win=%q, want both insufficient_data",
			s.PlayRateDirection, s.WinRateDirection)
	}
}

// TestComputeMetaEvolution_BiggestMoversPartitioned pins that the
// four mover lists each filter to (positive/negative, play/win) Δ
// and cap at metaEvolutionBiggestN.
func TestComputeMetaEvolution_BiggestMoversPartitioned(t *testing.T) {
	ref := int64(20 * weekSec)
	// Build pairs of archetypes for each mover slot.
	mk := func(arch string, weekIdx, n int, wins int) []MetaGameInput {
		ts := ref - int64(4-weekIdx)*weekSec
		out := make([]MetaGameInput, n)
		for i := range out {
			out[i] = MetaGameInput{FinishedAt: ts, Archetype: arch, Won: i < wins}
		}
		return out
	}
	var games []MetaGameInput
	// "play_gain" — grows from 10 to 40 of total; constant win
	games = append(games, mk("play_gain", 0, 10, 5)...)
	games = append(games, mk("play_gain", 2, 40, 20)...)
	// "play_loss" — shrinks from 40 to 10
	games = append(games, mk("play_loss", 0, 40, 20)...)
	games = append(games, mk("play_loss", 2, 10, 5)...)
	// "win_gain" — constant play count but win-rate climbs
	games = append(games, mk("win_gain", 0, 20, 5)...)
	games = append(games, mk("win_gain", 2, 20, 18)...)
	// "win_loss" — constant play count, win-rate falls
	games = append(games, mk("win_loss", 0, 20, 18)...)
	games = append(games, mk("win_loss", 2, 20, 5)...)

	out := ComputeMetaEvolution(games, ref, 4)
	if len(out.BiggestPlayRateGainers) == 0 || out.BiggestPlayRateGainers[0].Archetype != "play_gain" {
		t.Errorf("play_gain not at top of play-rate gainers: %+v", out.BiggestPlayRateGainers)
	}
	if len(out.BiggestPlayRateLosers) == 0 || out.BiggestPlayRateLosers[0].Archetype != "play_loss" {
		t.Errorf("play_loss not at top of play-rate losers: %+v", out.BiggestPlayRateLosers)
	}
	if len(out.BiggestWinRateGainers) == 0 || out.BiggestWinRateGainers[0].Archetype != "win_gain" {
		t.Errorf("win_gain not at top of win-rate gainers: %+v", out.BiggestWinRateGainers)
	}
	if len(out.BiggestWinRateLosers) == 0 || out.BiggestWinRateLosers[0].Archetype != "win_loss" {
		t.Errorf("win_loss not at top of win-rate losers: %+v", out.BiggestWinRateLosers)
	}
}

// TestComputeMetaEvolution_WeeklyBreakdownCarriesPlayAndWinRate pins
// that each Weekly row computes BOTH play rate (vs the week's total)
// and win rate (vs the archetype's games that week).
func TestComputeMetaEvolution_WeeklyBreakdownCarriesPlayAndWinRate(t *testing.T) {
	ref := int64(20 * weekSec)
	mk := func(arch string, weekIdx int, won bool) MetaGameInput {
		ts := ref - int64(4-weekIdx)*weekSec
		return MetaGameInput{FinishedAt: ts, Archetype: arch, Won: won}
	}
	games := []MetaGameInput{
		// week 1: 4 control (2 win), 6 combo (1 win) → 10 total
		mk("control", 1, true), mk("control", 1, true), mk("control", 1, false), mk("control", 1, false),
		mk("combo", 1, true), mk("combo", 1, false), mk("combo", 1, false), mk("combo", 1, false), mk("combo", 1, false), mk("combo", 1, false),
	}
	out := ComputeMetaEvolution(games, ref, 4)
	c := findEvo(out.Archetypes, "control")
	if c == nil {
		t.Fatal("missing control")
	}
	w := c.Weekly[1]
	if w.Games != 4 || w.Wins != 2 || w.TotalGamesWeek != 10 {
		t.Errorf("week[1] = %+v, want 4g/2w/10total", w)
	}
	if !evoApprox(w.PlayRate, 0.4) {
		t.Errorf("week[1].PlayRate = %.3f, want 0.4 (4/10)", w.PlayRate)
	}
	if !evoApprox(w.WinRate, 0.5) {
		t.Errorf("week[1].WinRate = %.3f, want 0.5 (2/4)", w.WinRate)
	}
}

// TestComputeMetaEvolution_OutOfWindowGamesSkipped pins that games
// outside [refUnix-weeks*7d, refUnix) don't contribute.
func TestComputeMetaEvolution_OutOfWindowGamesSkipped(t *testing.T) {
	ref := int64(20 * weekSec)
	games := []MetaGameInput{
		// in window
		{FinishedAt: ref - 1*weekSec, Archetype: "control", Won: true},
		// pre-window
		{FinishedAt: ref - 100*weekSec, Archetype: "control", Won: true},
		// future
		{FinishedAt: ref + 1, Archetype: "control", Won: true},
	}
	out := ComputeMetaEvolution(games, ref, 4)
	if out.TotalGames != 1 {
		t.Errorf("TotalGames = %d, want 1", out.TotalGames)
	}
}

// TestComputeMetaEvolution_ArchetypeSortByOverallGamesDesc pins the
// Archetypes-list ordering.
func TestComputeMetaEvolution_ArchetypeSortByOverallGamesDesc(t *testing.T) {
	ref := int64(20 * weekSec)
	mk := func(arch string, n int) []MetaGameInput {
		out := make([]MetaGameInput, n)
		for i := range out {
			out[i] = MetaGameInput{FinishedAt: ref - 2*weekSec, Archetype: arch}
		}
		return out
	}
	var games []MetaGameInput
	games = append(games, mk("zeta", 100)...)
	games = append(games, mk("alpha", 50)...)
	games = append(games, mk("beta", 30)...)
	out := ComputeMetaEvolution(games, ref, 4)
	want := []string{"zeta", "alpha", "beta"}
	for i, w := range want {
		if out.Archetypes[i].Archetype != w {
			t.Errorf("Archetypes[%d] = %s, want %s", i, out.Archetypes[i].Archetype, w)
		}
	}
}
