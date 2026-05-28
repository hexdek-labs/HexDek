package main

import (
	"math"
	"strings"
	"testing"
)

func almostEqualForecast(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

// TestForecastPodMatchups_BelowMinimum verifies the nil-result guard
// when the pod has fewer than 2 archetypes.
func TestForecastPodMatchups_BelowMinimum(t *testing.T) {
	if f := ForecastPodMatchups(nil); f != nil {
		t.Errorf("nil pod: want nil forecast, got %+v", f)
	}
	if f := ForecastPodMatchups([]string{}); f != nil {
		t.Errorf("empty pod: want nil forecast, got %+v", f)
	}
	if f := ForecastPodMatchups([]string{"Combo"}); f != nil {
		t.Errorf("1-deck pod: want nil forecast, got %+v", f)
	}
}

// TestForecastPodMatchups_ProbabilitiesSum verifies the
// fundamental invariant: per-seat win probabilities sum to 1.0
// (within float tolerance) across the pod.
func TestForecastPodMatchups_ProbabilitiesSum(t *testing.T) {
	pods := [][]string{
		{"Combo", "Combo", "Combo", "Combo"},
		{"Combo", "Stax", "Voltron", "Aggro"},
		{"Reanimator", "Aristocrats", "Enchantress", "Control"},
		{"Storm", "Stax", "Control", "Combo"},
	}
	for _, pod := range pods {
		f := ForecastPodMatchups(pod)
		if f == nil {
			t.Fatalf("pod %v: nil forecast", pod)
		}
		sum := 0.0
		for _, s := range f.Seats {
			sum += s.WinProbability
		}
		if !almostEqualForecast(sum, 1.0, 1e-9) {
			t.Errorf("pod %v: probabilities sum to %.6f, want 1.0", pod, sum)
		}
	}
}

// TestForecastPodMatchups_MirrorMatch verifies the symmetric case:
// 4 decks of the same archetype produce uniform 0.25 win probability
// across all seats (no edge between mirrors).
func TestForecastPodMatchups_MirrorMatch(t *testing.T) {
	f := ForecastPodMatchups([]string{"Combo", "Combo", "Combo", "Combo"})
	if f == nil {
		t.Fatal("nil forecast")
	}
	for i, s := range f.Seats {
		if !almostEqualForecast(s.WinProbability, 0.25, 1e-9) {
			t.Errorf("mirror seat %d: want 0.25, got %.4f", i, s.WinProbability)
		}
	}
	if f.AvgEdgeMagnitude != 0 {
		t.Errorf("mirror pod: want AvgEdgeMagnitude=0, got %.4f", f.AvgEdgeMagnitude)
	}
	if f.Asymmetric {
		t.Error("mirror pod: want Asymmetric=false")
	}
}

// TestForecastPodMatchups_StaxDominantVsAllOpponents verifies a
// known matchup pattern: Stax favored vs Combo, Storm, Reanimator,
// Aristocrats, Voltron, Enchantress (per metaMatchupDB). A pod of
// Stax + 3 of those archetypes should put Stax solidly above 0.25.
func TestForecastPodMatchups_StaxDominantVsAllOpponents(t *testing.T) {
	// Stax is favored vs Combo (strong), Storm (strong),
	// Reanimator (strong) per metaMatchupDB + overrides.
	f := ForecastPodMatchups([]string{"Stax", "Combo", "Storm", "Reanimator"})
	if f == nil {
		t.Fatal("nil forecast")
	}
	stax := f.Seats[0]
	if stax.WinProbability <= 0.40 {
		t.Errorf("Stax vs Combo/Storm/Reanimator: want WinProb > 0.40, got %.4f", stax.WinProbability)
	}
	if f.DominantSeat != 0 {
		t.Errorf("Stax should be dominant seat, got %d (%s)", f.DominantSeat, f.Seats[f.DominantSeat].Archetype)
	}
	if !f.Asymmetric {
		t.Error("Stax-favored pod: want Asymmetric=true")
	}
}

// TestForecastPodMatchups_StormHatedByStax verifies the inverse —
// Storm in a pod with Stax should land BELOW 0.25, since Storm is
// unfavored × strong vs Stax (and unfavored vs Control/Aggro).
func TestForecastPodMatchups_StormHatedByStax(t *testing.T) {
	f := ForecastPodMatchups([]string{"Storm", "Stax", "Control", "Aggro"})
	if f == nil {
		t.Fatal("nil forecast")
	}
	storm := f.Seats[0]
	if storm.WinProbability >= 0.20 {
		t.Errorf("Storm vs Stax+Control+Aggro: want WinProb < 0.20, got %.4f", storm.WinProbability)
	}
	// Storm should NOT be the dominant seat.
	if f.DominantSeat == 0 {
		t.Errorf("Storm should not be dominant in this pod, got dominant=%s",
			f.Seats[f.DominantSeat].Archetype)
	}
}

// TestForecastPodMatchups_VoltronUnderdog verifies Voltron's
// signature weakness: it's unfavored vs Control, Stax, Combo,
// Aristocrats, Reanimator, Storm, Enchantress (per matchup DB).
// A Voltron deck in a pod with 3 of those archetypes should be the
// underdog.
func TestForecastPodMatchups_VoltronUnderdog(t *testing.T) {
	f := ForecastPodMatchups([]string{"Voltron", "Control", "Stax", "Aristocrats"})
	if f == nil {
		t.Fatal("nil forecast")
	}
	voltron := f.Seats[0]
	if voltron.WinProbability >= 0.20 {
		t.Errorf("Voltron vs Control+Stax+Aristocrats: want WinProb < 0.20, got %.4f", voltron.WinProbability)
	}
}

// TestForecastPodMatchups_RockPaperScissors_AggroComboStaxControl
// pins the classic 4-way RPS triangle. Each archetype has
// directional edges against the others; the forecast should
// produce a balanced-but-not-uniform distribution.
func TestForecastPodMatchups_RockPaperScissors_AggroComboStaxControl(t *testing.T) {
	f := ForecastPodMatchups([]string{"Aggro", "Combo", "Stax", "Control"})
	if f == nil {
		t.Fatal("nil forecast")
	}
	// Stax should be the dominant seat (favored vs Combo strong +
	// favored vs Aggro + neutral vs Control).
	if f.Seats[2].Archetype != "Stax" {
		t.Fatalf("expected seat 2 to be Stax, got %s", f.Seats[2].Archetype)
	}
	if f.DominantSeat != 2 {
		t.Errorf("RPS pod: want Stax (seat 2) dominant, got %s (seat %d)",
			f.Seats[f.DominantSeat].Archetype, f.DominantSeat)
	}
	// All seats should be within a reasonable range — the RPS
	// triangle doesn't have a 60%+ favorite.
	for i, s := range f.Seats {
		if s.WinProbability < 0.05 || s.WinProbability > 0.60 {
			t.Errorf("seat %d (%s) win prob %.4f out of expected [0.05, 0.60]",
				i, s.Archetype, s.WinProbability)
		}
	}
}

// TestForecastPodMatchups_EdgesPopulated verifies that each seat's
// EdgesVsOthers slice has exactly N-1 entries with the correct
// opponent seat indexes (and references the right opponents).
func TestForecastPodMatchups_EdgesPopulated(t *testing.T) {
	pod := []string{"Combo", "Stax", "Voltron", "Aggro"}
	f := ForecastPodMatchups(pod)
	if f == nil {
		t.Fatal("nil forecast")
	}
	for i, s := range f.Seats {
		if len(s.EdgesVsOthers) != 3 {
			t.Errorf("seat %d: want 3 edges, got %d", i, len(s.EdgesVsOthers))
		}
		// Edge OpponentSeat values should be {0,1,2,3} \ {i}.
		seenOpps := map[int]bool{}
		for _, e := range s.EdgesVsOthers {
			if e.OpponentSeat == i {
				t.Errorf("seat %d: edge against self at OpponentSeat=%d", i, e.OpponentSeat)
			}
			if seenOpps[e.OpponentSeat] {
				t.Errorf("seat %d: duplicate edge against OpponentSeat=%d", i, e.OpponentSeat)
			}
			seenOpps[e.OpponentSeat] = true
			if e.OpponentArchetype != pod[e.OpponentSeat] {
				t.Errorf("seat %d edge vs %d: archetype %q does not match pod %q",
					i, e.OpponentSeat, e.OpponentArchetype, pod[e.OpponentSeat])
			}
		}
	}
}

// TestEdgeMagnitudeFor_Table pins the (rating, strength) → numeric
// edge table at every combination.
func TestEdgeMagnitudeFor_Table(t *testing.T) {
	cases := []struct {
		rating, strength string
		want             float64
	}{
		{"favored", "strong", 0.30},
		{"favored", "moderate", 0.15},
		{"favored", "slight", 0.05},
		{"neutral", "", 0},
		{"unfavored", "slight", -0.05},
		{"unfavored", "moderate", -0.15},
		{"unfavored", "strong", -0.30},
		{"", "moderate", 0},        // missing rating treated as neutral
		{"favored", "unknown", 0.15}, // unknown strength → moderate default
	}
	for _, c := range cases {
		got := edgeMagnitudeFor(c.rating, c.strength)
		if !almostEqualForecast(got, c.want, 1e-9) {
			t.Errorf("(%q, %q): want %.4f, got %.4f", c.rating, c.strength, c.want, got)
		}
	}
}

// TestForecastPodMatchups_VerdictIncludesDominantSeat verifies the
// verdict string includes the dominant seat's archetype and win
// probability.
func TestForecastPodMatchups_VerdictIncludesDominantSeat(t *testing.T) {
	f := ForecastPodMatchups([]string{"Combo", "Voltron", "Midrange", "Aggro"})
	if f == nil {
		t.Fatal("nil forecast")
	}
	dom := f.Seats[f.DominantSeat]
	if !strings.Contains(f.Verdict, dom.Archetype) {
		t.Errorf("verdict missing dominant archetype %q: %q", dom.Archetype, f.Verdict)
	}
	if !strings.Contains(f.Verdict, "Forecast:") {
		t.Errorf("verdict missing 'Forecast:' header: %q", f.Verdict)
	}
}

// TestSortedSeatsByWinProbability_NonMutating verifies the helper
// doesn't mutate the input forecast.
func TestSortedSeatsByWinProbability_NonMutating(t *testing.T) {
	f := ForecastPodMatchups([]string{"Combo", "Stax", "Voltron", "Aggro"})
	if f == nil {
		t.Fatal("nil forecast")
	}
	originalFirstSeat := f.Seats[0].Archetype
	sorted := SortedSeatsByWinProbability(f)
	if len(sorted) != 4 {
		t.Errorf("sorted len: want 4, got %d", len(sorted))
	}
	// Verify the sorted slice is descending by WinProbability.
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1].WinProbability < sorted[i].WinProbability {
			t.Errorf("not sorted desc at index %d: %.4f < %.4f",
				i, sorted[i-1].WinProbability, sorted[i].WinProbability)
		}
	}
	// Input untouched: seat 0 still its original archetype.
	if f.Seats[0].Archetype != originalFirstSeat {
		t.Errorf("input mutated: seat 0 was %q, now %q",
			originalFirstSeat, f.Seats[0].Archetype)
	}
	// Returned copy is a new backing array.
	sorted[0].Archetype = "MUTATED"
	if f.Seats[f.DominantSeat].Archetype == "MUTATED" {
		t.Error("sorted slice shares backing array with input")
	}
}

// TestForecastPodMatchups_CanonicaliserAccepts_VariousAliases
// verifies that the forecaster accepts both raw classifier output
// and canonical keys.
func TestForecastPodMatchups_CanonicaliserAccepts_VariousAliases(t *testing.T) {
	// Raw classifier output names (title case, multi-word).
	rawPod := []string{"Combo", "Go Wide Tokens", "Infinite Combo", "Midrange"}
	rawF := ForecastPodMatchups(rawPod)
	if rawF == nil {
		t.Fatal("nil forecast on raw classifier output")
	}
	// "Combo" and "Infinite Combo" should both canonicalise to
	// "combo"; the forecast should not crash on duplicates.
	for i, s := range rawF.Seats {
		if s.ArchetypeKey == "" {
			t.Errorf("seat %d: empty ArchetypeKey for archetype %q", i, s.Archetype)
		}
	}
}

// TestForecastPodMatchups_AvgEdgeMagnitudeRange verifies the
// AvgEdgeMagnitude lands in a plausible range for a strong-matchup
// pod (> 0.10) and a near-mirror pod (≈ 0).
func TestForecastPodMatchups_AvgEdgeMagnitudeRange(t *testing.T) {
	// Heavy-matchup pod: Stax / Combo / Storm / Reanimator — all
	// archetypes with strong directional opinions.
	heavy := ForecastPodMatchups([]string{"Stax", "Combo", "Storm", "Reanimator"})
	if heavy.AvgEdgeMagnitude < 0.10 {
		t.Errorf("heavy-matchup pod: want AvgEdgeMagnitude > 0.10, got %.4f", heavy.AvgEdgeMagnitude)
	}
	// Mirror pod: 4× same archetype, no edges at all.
	mirror := ForecastPodMatchups([]string{"Midrange", "Midrange", "Midrange", "Midrange"})
	if mirror.AvgEdgeMagnitude > 0.01 {
		t.Errorf("mirror pod: want AvgEdgeMagnitude ≈ 0, got %.4f", mirror.AvgEdgeMagnitude)
	}
}
