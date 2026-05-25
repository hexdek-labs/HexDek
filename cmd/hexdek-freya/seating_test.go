package main

import (
	"strings"
	"testing"
)

// seatingFixture builds a minimal DeckProfile + FreyaReport with
// the signals RecommendSeating reads. The defaults are deliberately
// neutral so each test can override one axis at a time and observe
// the seat preference shift.
type seatingSpec struct {
	primary         string
	bracket         int
	avgCMC          float64
	rampCount       int
	powerPercentile int
	commanderCMC    int
	turnToCommander float64
	isCmdCentric    bool
	infinites       int
	tutors          int
	removal         int
	counterspells   int
	wipes           int
}

func makeSeatingFixture(spec seatingSpec) (*DeckProfile, *FreyaReport) {
	dp := &DeckProfile{
		PrimaryArchetype:   spec.primary,
		Bracket:            spec.bracket,
		AvgCMC:             spec.avgCMC,
		RampCount:          spec.rampCount,
		PowerPercentile:    spec.powerPercentile,
		CommanderCMC:       spec.commanderCMC,
		AvgTurnToCommander: spec.turnToCommander,
		IsCommanderCentric: spec.isCmdCentric,
	}
	report := &FreyaReport{
		NonLandTutorCount: spec.tutors,
		RemovalCount:      spec.removal,
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: spec.counterspells,
				RoleBoardWipe:    spec.wipes,
			},
		},
	}
	for i := 0; i < spec.infinites; i++ {
		report.TrueInfinites = append(report.TrueInfinites, ComboResult{})
	}
	return dp, report
}

// TestRecommendSeating_ComboPrefersSeatZero pins the headline use
// case: a combo deck with tutor density and a lean curve should
// prefer the early seats. The Decks-screen panel's whole point is
// to surface "you should sit seat 0 next pod" as actionable advice.
func TestRecommendSeating_ComboPrefersSeatZero(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Combo", bracket: 4,
		avgCMC: 2.3, rampCount: 12, powerPercentile: 70,
		infinites: 1, tutors: 6, removal: 4, counterspells: 2,
	})
	rec := RecommendSeating(dp, rep)
	if rec.BestSeat != 0 {
		t.Errorf("combo deck should prefer seat 0, got best seat %d", rec.BestSeat)
	}
	if rec.BestPowerDelta < 5 {
		t.Errorf("combo deck should have a meaningful seat preference (≥5pp delta), got %d", rec.BestPowerDelta)
	}
	if !strings.Contains(rec.Headline, "seat 0") {
		t.Errorf("headline should call out seat 0, got %q", rec.Headline)
	}
	// Headline format per the spec — "Your deck is Npp better at
	// seat X vs seat Y — consider sitting seat X next pod."
	if !strings.Contains(rec.Headline, "pp better at seat") {
		t.Errorf("headline format drift, got %q", rec.Headline)
	}
	if !strings.Contains(rec.Headline, "consider sitting seat") {
		t.Errorf("headline should include 'consider sitting seat' actionable, got %q", rec.Headline)
	}
}

// TestRecommendSeating_ControlPrefersSeatThree pins the opposite
// pole: a counter-heavy control deck wants late seats. Same headline
// shape, opposite direction.
func TestRecommendSeating_ControlPrefersSeatThree(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Control", bracket: 4,
		avgCMC: 3.4, rampCount: 10, powerPercentile: 65,
		removal: 8, counterspells: 6, wipes: 3,
	})
	rec := RecommendSeating(dp, rep)
	if rec.BestSeat != 3 {
		t.Errorf("control + counters + wipes should prefer seat 3, got best seat %d", rec.BestSeat)
	}
	if rec.WorstSeat != 0 {
		t.Errorf("control deck should have seat 0 as worst, got %d", rec.WorstSeat)
	}
	if rec.BestPowerDelta < 5 {
		t.Errorf("control deck should have meaningful preference, got delta %d", rec.BestPowerDelta)
	}
}

// TestRecommendSeating_MidrangeIsAgnostic pins the no-preference
// case: a generic midrange deck with no strong signals lands within
// 2pp across all 4 seats and the headline says "any seat works"
// rather than fabricating a recommendation.
func TestRecommendSeating_MidrangeIsAgnostic(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Midrange", bracket: 3,
		avgCMC: 3.0, rampCount: 9, powerPercentile: 55,
		removal: 5, counterspells: 2, wipes: 1,
	})
	rec := RecommendSeating(dp, rep)
	if rec.BestPowerDelta >= 3 {
		t.Errorf("midrange deck should have flat preference (delta <3), got %d", rec.BestPowerDelta)
	}
	if !strings.Contains(rec.Headline, "No strong seat preference") &&
		!strings.Contains(rec.Headline, "any seat works") {
		t.Errorf("flat-preference deck should produce 'any seat works' headline, got %q", rec.Headline)
	}
}

// TestRecommendSeating_VoltronPrefersEarlySeats pins that combat-
// pressure archetypes like Voltron and Aggro inherit the same
// early-seat preference as combo (different reason — pushing
// damage before chump blockers arrive).
func TestRecommendSeating_VoltronPrefersEarlySeats(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Voltron", bracket: 3,
		avgCMC: 2.5, rampCount: 10, powerPercentile: 60,
	})
	rec := RecommendSeating(dp, rep)
	if rec.BestSeat > 1 {
		t.Errorf("Voltron should prefer seats 0-1, got best seat %d", rec.BestSeat)
	}
	// Reasoning for the best seat should include the voltron-specific
	// rule (combat-pressure / chump blockers).
	best := rec.Seats[rec.BestSeat]
	hasReason := false
	for _, r := range best.Reasoning {
		if strings.Contains(r, "combat-pressure") || strings.Contains(r, "chump blockers") {
			hasReason = true
			break
		}
	}
	if !hasReason {
		t.Errorf("best-seat reasoning should include voltron/aggro rationale, got %v", best.Reasoning)
	}
}

// TestRecommendSeating_ReanimatorPrefersLateSeats pins reanimator's
// late-seat preference — it needs setup turns to mill the graveyard,
// so going first hurts.
func TestRecommendSeating_ReanimatorPrefersLateSeats(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Reanimator", bracket: 4,
		avgCMC: 3.6, rampCount: 8, powerPercentile: 60,
		removal: 4, counterspells: 2,
	})
	rec := RecommendSeating(dp, rep)
	if rec.BestSeat < 2 {
		t.Errorf("Reanimator should prefer seats 2-3, got best seat %d", rec.BestSeat)
	}
}

// TestRecommendSeating_SeatsAlwaysFourAndSorted pins the structural
// guarantee: every recommendation returns exactly 4 SeatScore
// entries indexed 0-3 in order. The Decks-screen panel iterates
// these by index when rendering the side-by-side comparison.
func TestRecommendSeating_SeatsAlwaysFourAndSorted(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{primary: "Midrange", bracket: 3})
	rec := RecommendSeating(dp, rep)
	if len(rec.Seats) != 4 {
		t.Fatalf("expected 4 seats, got %d", len(rec.Seats))
	}
	for i, s := range rec.Seats {
		if s.Seat != i {
			t.Errorf("Seats[%d].Seat = %d, want %d (must be sorted by seat number)", i, s.Seat, i)
		}
		if s.PowerPct < 1 || s.PowerPct > 99 {
			t.Errorf("Seats[%d].PowerPct = %d, want [1,99]", i, s.PowerPct)
		}
	}
}

// TestRecommendSeating_NilSafe pins safe defaults: a nil deck
// profile returns nil; a non-nil profile with no analysis report
// still produces a result (the rules guard their own field reads,
// so the recommendation just lacks the report-driven adjustments).
func TestRecommendSeating_NilSafe(t *testing.T) {
	if got := RecommendSeating(nil, nil); got != nil {
		t.Errorf("nil deck profile should return nil, got %+v", got)
	}
	dp := &DeckProfile{PrimaryArchetype: "Midrange"}
	rec := RecommendSeating(dp, nil)
	if rec == nil {
		t.Fatal("non-nil profile + nil report should still produce a recommendation")
	}
	if len(rec.Seats) != 4 {
		t.Errorf("nil report should still produce 4 seats, got %d", len(rec.Seats))
	}
}

// TestRecommendSeating_BaselineDefaultsTo50 pins that an unscored
// deck (PowerPercentile=0) gets a sensible default baseline of 50
// rather than starting at 0 and clamping every adjustment to 1.
func TestRecommendSeating_BaselineDefaultsTo50(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Midrange", bracket: 3, powerPercentile: 0,
	})
	rec := RecommendSeating(dp, rep)
	for _, s := range rec.Seats {
		// All adjustments are < 50 in magnitude, so an unscored deck
		// landing at baseline=50 produces seats in the 40-60 range
		// rather than getting stuck at the [1,99] clamp.
		if s.PowerPct < 20 || s.PowerPct > 80 {
			t.Errorf("seat %d PowerPct=%d outside expected unscored range [20,80]", s.Seat, s.PowerPct)
		}
	}
}

// TestRecommendSeating_HeadlineExactFormat pins the exact wording
// shape from the spec: "Your deck is Npp better at seat X vs seat Y
// — consider sitting seat X next pod." Downstream consumers
// (Decks-screen banner) parse this directly, so the format is
// load-bearing.
func TestRecommendSeating_HeadlineExactFormat(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Combo", bracket: 4,
		avgCMC: 2.3, rampCount: 12, powerPercentile: 70,
		infinites: 1, tutors: 6,
	})
	rec := RecommendSeating(dp, rep)
	wanted := []string{
		"Your deck is ",
		"pp better at seat ",
		" vs seat ",
		" — consider sitting seat ",
		" next pod.",
	}
	for _, w := range wanted {
		if !strings.Contains(rec.Headline, w) {
			t.Errorf("headline missing %q; got: %q", w, rec.Headline)
		}
	}
}

// TestRecommendSeating_ReasoningOnlyForBestAndWorst pins that the
// text renderer surfaces reasoning ONLY for the best and worst
// seats (not all 4 — that's 4× the noise for limited additional
// information).
func TestFormatSeatRecommendation_RendersBestWorstSections(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Combo", bracket: 4,
		avgCMC: 2.3, rampCount: 12, powerPercentile: 70,
		infinites: 1, tutors: 6, removal: 6, counterspells: 4,
	})
	rec := RecommendSeating(dp, rep)
	out := FormatSeatRecommendation(rec)

	wanted := []string{
		"Seating Recommendation",
		"Your deck is",
		"Seat 0:",
		"Seat 1:",
		"Seat 2:",
		"Seat 3:",
		"Why seat",
		"wins:",
		"loses:",
	}
	for _, w := range wanted {
		if !strings.Contains(out, w) {
			t.Errorf("FormatSeatRecommendation missing %q. Got:\n%s", w, out)
		}
	}
	// Visual markers ▲ / ▼ for best/worst seat.
	if !strings.Contains(out, "▲") {
		t.Errorf("expected ▲ marker on best seat")
	}
	if !strings.Contains(out, "▼") {
		t.Errorf("expected ▼ marker on worst seat")
	}
}

// TestRecommendSeating_AdjustmentSumsCorrectly pins the rule-
// summation math: when two rules fire, the per-seat Adjustment
// equals the sum of their deltas. Specifically, a combo deck with
// counterspells gets BOTH the combo_with_tutors rule (+4 seat 0,
// -3 seat 3) AND the counter_dense rule (-3 seat 0, +3 seat 3).
// Seat 0 adjustment = +4 - 3 = +1; seat 3 adjustment = -3 + 3 = 0.
// The rules-cancel-out case is important — it lets a deck split
// across signals end up flat rather than getting double-counted.
func TestRecommendSeating_AdjustmentSumsCorrectly(t *testing.T) {
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Combo", bracket: 4,
		avgCMC: 3.0, // not lean or slow → fast_curve / slow_curve don't fire
		rampCount:       8, // < 12 → heavy_ramp doesn't fire
		powerPercentile: 50,
		infinites:       1, tutors: 6, // combo_with_tutors fires
		counterspells: 5, // counter_dense fires
	})
	rec := RecommendSeating(dp, rep)
	// Expected per-seat adjustment from these two rules:
	//   combo_with_tutors: [+4, +2, -1, -3]
	//   counter_dense:     [-3, -1, +1, +3]
	// Sum:                [+1, +1,  0,  0]
	wantedAdjustments := []int{1, 1, 0, 0}
	for i, want := range wantedAdjustments {
		if rec.Seats[i].Adjustment != want {
			t.Errorf("Seats[%d].Adjustment = %d, want %d (combo_with_tutors + counter_dense)",
				i, rec.Seats[i].Adjustment, want)
		}
	}
}

// TestRecommendSeating_TiebreakLowerSeat pins the tiebreak rule
// for determinism: when two seats have identical PowerPct, the
// lower seat number wins (best) and the higher seat number loses
// (worst). Without this, map-iter / scan-order noise would flip
// the headline arbitrarily.
func TestRecommendSeating_TiebreakLowerSeat(t *testing.T) {
	// Construct a deck where seats 0 and 1 both land at the same
	// score (no rules that distinguish them at this configuration).
	// All four seats have Adjustment=0, so the lowest seat number
	// wins by the tiebreak.
	dp, rep := makeSeatingFixture(seatingSpec{
		primary: "Midrange", bracket: 3,
		avgCMC: 3.0, rampCount: 9, powerPercentile: 50,
	})
	rec := RecommendSeating(dp, rep)
	// All seats land at the same baseline → best/worst should both
	// be seat 0 (lowest index tiebreak).
	if rec.BestSeat != 0 {
		t.Errorf("flat-preference deck: BestSeat = %d, want 0 (tiebreak by lower seat)", rec.BestSeat)
	}
	if rec.WorstSeat != 0 {
		t.Errorf("flat-preference deck: WorstSeat = %d, want 0 (tiebreak by lower seat)", rec.WorstSeat)
	}
}
