package tournament

import (
	"math"
	"sort"
	"strings"
	"testing"
)

func TestWilsonInterval_KnownValues(t *testing.T) {
	cases := []struct {
		wins, games int
		wantLo      float64
		wantHi      float64
		tol         float64
	}{
		// 5/10 → centered around 0.5; reference Wilson @ 95%: [0.237, 0.763]
		{5, 10, 0.237, 0.763, 0.01},
		// 0/10 — degenerate proportion, Wilson upper bound ≈ 0.278
		{0, 10, 0.0, 0.278, 0.01},
		// 100/100 — degenerate the other way, Wilson lower bound ≈ 0.964
		{100, 100, 0.964, 1.0, 0.01},
		// 0 games → zero interval (avoid div-by-zero).
		{0, 0, 0, 0, 0.0001},
	}
	for _, c := range cases {
		lo, hi := wilsonInterval(c.wins, c.games)
		if math.Abs(lo-c.wantLo) > c.tol {
			t.Errorf("wilsonInterval(%d,%d).lo = %.3f, want %.3f ±%.3f", c.wins, c.games, lo, c.wantLo, c.tol)
		}
		if math.Abs(hi-c.wantHi) > c.tol {
			t.Errorf("wilsonInterval(%d,%d).hi = %.3f, want %.3f ±%.3f", c.wins, c.games, hi, c.wantHi, c.tol)
		}
	}
}

func TestComputeRivalries_FiltersBelowMinGames(t *testing.T) {
	r := makeResultFixture(map[string]map[string]int{
		"Korvold": {"Yuriko": 4, "Najeela": 6}, // 4/8 = 50% (too few games)
		"Yuriko":  {"Korvold": 4},
		"Najeela": {"Korvold": 2},
	}, map[string]map[string]int{
		"Korvold": {"Yuriko": 8, "Najeela": 8},
		"Yuriko":  {"Korvold": 8},
		"Najeela": {"Korvold": 8},
	}, map[string]int{
		"Korvold": 30, "Yuriko": 10, "Najeela": 5,
	}, map[string]int{
		"Korvold": 100, "Yuriko": 30, "Najeela": 30,
	})

	opts := DefaultRivalryOptions() // MinGames=10
	got := ComputeRivalries(r, opts)
	if len(got) != 0 {
		t.Fatalf("expected zero rivalries when every pair has < 10 games, got %d: %+v", len(got), got)
	}
}

func TestComputeRivalries_FlagsLopsidedMatchup(t *testing.T) {
	// Korvold's overall winrate is 30/100 = 30%.
	// Korvold over-performs against Yuriko: 16/20 = 80% (Δ = +50%).
	// 20 games is well above the default MinGames=10; Wilson 95% lower
	// bound ≈ 0.58, far above the 30% baseline → significant.
	r := makeResultFixture(map[string]map[string]int{
		"Korvold": {"Yuriko": 16},
		"Yuriko":  {"Korvold": 4},
	}, map[string]map[string]int{
		"Korvold": {"Yuriko": 20},
		"Yuriko":  {"Korvold": 20},
	}, map[string]int{
		"Korvold": 30, "Yuriko": 4,
	}, map[string]int{
		"Korvold": 100, "Yuriko": 20,
	})

	got := ComputeRivalries(r, DefaultRivalryOptions())
	if len(got) == 0 {
		t.Fatal("expected Korvold-vs-Yuriko rivalry to surface")
	}
	first := got[0]
	if first.Commander != "Korvold" || first.Opponent != "Yuriko" {
		t.Fatalf("expected first rivalry to be Korvold→Yuriko, got %+v", first)
	}
	if first.Games != 20 || first.Wins != 16 {
		t.Errorf("games/wins: got %d/%d, want 20/16", first.Games, first.Wins)
	}
	if math.Abs(first.Winrate-0.8) > 1e-9 {
		t.Errorf("winrate: got %f, want 0.80", first.Winrate)
	}
	if math.Abs(first.BaselineWinrate-0.3) > 1e-9 {
		t.Errorf("baseline: got %f, want 0.30", first.BaselineWinrate)
	}
	if math.Abs(first.Deviation-0.5) > 1e-9 {
		t.Errorf("deviation: got %f, want +0.50", first.Deviation)
	}
	if !first.Significant {
		t.Errorf("expected significant=true (baseline 0.30 outside Wilson CI [%.3f, %.3f])", first.WilsonLow, first.WilsonHigh)
	}
}

func TestComputeRivalries_SortedByAbsDeviation(t *testing.T) {
	// Three matchups against Korvold (baseline 40/100 = 40%):
	//   Yuriko:    16/20 = 80% (Δ = +40%, Wilson clears baseline easily)
	//   Najeela:   2/20  = 10% (Δ = −30%, Wilson upper ≈ 0.30 vs baseline 0.40 → significant)
	//   Atraxa:    9/20  = 45% (Δ = +5%, Wilson encompasses baseline → filtered)
	r := makeResultFixture(map[string]map[string]int{
		"Korvold": {"Yuriko": 16, "Najeela": 2, "Atraxa": 9},
	}, map[string]map[string]int{
		"Korvold": {"Yuriko": 20, "Najeela": 20, "Atraxa": 20},
	}, map[string]int{
		"Korvold": 40,
	}, map[string]int{
		"Korvold": 100,
	})

	got := ComputeRivalries(r, DefaultRivalryOptions())
	if len(got) < 2 {
		t.Fatalf("expected at least 2 rivalries (Yuriko +40, Najeela -30), got %d: %+v", len(got), got)
	}
	if got[0].Opponent != "Yuriko" {
		t.Errorf("most lopsided should be Yuriko (Δ=+40%%), got %s (Δ=%.2f)", got[0].Opponent, got[0].Deviation)
	}
	if got[1].Opponent != "Najeela" {
		t.Errorf("second most lopsided should be Najeela (Δ=-30%%), got %s (Δ=%.2f)", got[1].Opponent, got[1].Deviation)
	}
	for _, riv := range got {
		if riv.Opponent == "Atraxa" {
			t.Errorf("Atraxa (Δ=+5%%) should be filtered by significance gate, got %+v", riv)
		}
	}
}

func TestComputeRivalries_RequireSignificanceFalse_KeepsLowSampleLargeEffect(t *testing.T) {
	// 8 wins / 10 games = 80% vs baseline 50%. Wilson 95% CI at 10 games
	// is ≈ [0.49, 0.94], so the baseline 0.5 sits at the lower edge —
	// borderline, may not pass the strict significance gate. With
	// RequireSignificance=false the 30-point deviation still surfaces.
	r := makeResultFixture(map[string]map[string]int{
		"Korvold": {"Yuriko": 8},
	}, map[string]map[string]int{
		"Korvold": {"Yuriko": 10},
	}, map[string]int{
		"Korvold": 50,
	}, map[string]int{
		"Korvold": 100,
	})

	opts := RivalryOptions{
		MinGames:            10,
		MinAbsDeviation:     0.05,
		RequireSignificance: false,
	}
	got := ComputeRivalries(r, opts)
	if len(got) == 0 {
		t.Fatal("expected Korvold-vs-Yuriko to surface with RequireSignificance=false")
	}
}

func TestWriteRivalriesSection_RendersAndCaps(t *testing.T) {
	r := makeResultFixture(map[string]map[string]int{
		"Korvold": {"Yuriko": 16, "Najeela": 2},
		"Yuriko":  {"Korvold": 4},
		"Najeela": {"Korvold": 18},
	}, map[string]map[string]int{
		"Korvold": {"Yuriko": 20, "Najeela": 20},
		"Yuriko":  {"Korvold": 20},
		"Najeela": {"Korvold": 20},
	}, map[string]int{
		"Korvold": 30, "Yuriko": 4, "Najeela": 50,
	}, map[string]int{
		"Korvold": 100, "Yuriko": 20, "Najeela": 100,
	})

	var b strings.Builder
	WriteRivalriesSection(&b, r, DefaultRivalryOptions(), 1)
	out := b.String()

	if !strings.Contains(out, "## Notable Rivalries") {
		t.Fatalf("expected header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Korvold") || !strings.Contains(out, "Yuriko") {
		t.Errorf("expected most-lopsided pair in output, got:\n%s", out)
	}
	// Cap=1: should NOT include the Najeela row even though it qualifies.
	if strings.Contains(out, "Najeela") {
		t.Errorf("cap=1 should suppress second rivalry, got:\n%s", out)
	}
}

func TestWriteRivalriesSection_EmptyOnNoData(t *testing.T) {
	var b strings.Builder
	WriteRivalriesSection(&b, &TournamentResult{}, DefaultRivalryOptions(), 0)
	if b.Len() != 0 {
		t.Fatalf("expected no output for empty result, got: %q", b.String())
	}
}

// makeResultFixture wires the minimum fields ComputeRivalries reads
// (CommanderNames, MatchupMatrix, MatchupGames, WinsByCommander,
// GamesPlayedByCommander) so tests don't depend on the full aggregate
// pipeline.
func makeResultFixture(matrix, games map[string]map[string]int, wins, played map[string]int) *TournamentResult {
	names := make([]string, 0, len(matrix))
	for k := range matrix {
		names = append(names, k)
	}
	// Stable order so tests don't flake on map-iteration randomness.
	// The earlier shape here was a no-op `for k := range append(...)`
	// loop that allocated a copy but never sorted it — flagged by
	// `go vet` (append with no values). Actually sort the slice.
	sort.Strings(names)
	return &TournamentResult{
		CommanderNames:         names,
		MatchupMatrix:          matrix,
		MatchupGames:           games,
		WinsByCommander:        wins,
		GamesPlayedByCommander: played,
		Games:                  100,
	}
}
