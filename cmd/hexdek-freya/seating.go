package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Deck-rotation / seating recommendation. Same deck plays differently
// depending on where it sits in a 4-player pod:
//
//   - Seat 0 (going first): tempo-positive but skips the first untap.
//     Best for combo decks racing to kill turn, voltron/aggro pushing
//     early damage, and stax wanting to lock before opponents develop.
//   - Seat 1: one extra card seen before threats. Slight value bump.
//   - Seat 2: balanced — sees half the board before acting.
//   - Seat 3 (going last): full information when its turn comes around.
//     Best for control / counter-heavy / reactive decks; worst for
//     anything trying to land a fast clock before opponents stabilize.
//
// RecommendSeating computes a 0-100 percentile-like score per seat,
// using the deck's PowerPercentile as the baseline and applying signed
// adjustments per archetype + density signal. The headline output is
// the highest-delta suggestion ("Your deck is 3pp better at seat 0
// vs seat 3 — consider sitting seat 0 next pod").
// ---------------------------------------------------------------------------

// SeatScore is one row of the recommendation — the per-seat power
// projection plus the adjustment relative to the deck's baseline.
type SeatScore struct {
	Seat       int      // 0-3
	PowerPct   int      // clamped to [1, 99] like the rest of the percentile pipeline
	Adjustment int      // signed delta from the deck's PowerPercentile baseline
	Reasoning  []string // why this seat is better/worse for THIS deck
}

// SeatRecommendation is the top-level shape: 4 SeatScore entries
// (sorted by seat number for deterministic rendering), the
// best/worst seat call-outs, the percentage-point delta between
// them, and the user-facing headline string.
type SeatRecommendation struct {
	BestSeat       int
	WorstSeat      int
	BestPowerDelta int       // BestSeat.PowerPct − WorstSeat.PowerPct
	Seats          []SeatScore
	Headline       string
}

// seatRule is one applied signal. Its Delta is a [4]int: per-seat
// percentile points to add when the predicate matches. Predicate
// fires once per deck; the same signed delta vector applies to all
// 4 seats. Reason is the human-readable why string that gets
// attached to the seats that benefited (positive delta) or suffered
// (negative delta) — the strongest swing wins the explanation slot
// for the worst seat too.
type seatRule struct {
	Name      string
	Predicate func(dp *DeckProfile, report *FreyaReport) bool
	Delta     [4]int
	Reason    string
}

// seatRules is the canonical adjustment ledger. Adding rules here
// extends the recommender without touching the scoring loop. Each
// rule fires independently — totals are summed across applicable
// rules per seat. Heuristic magnitudes calibrated so a coherent
// archetype signal (combo, control, stax) lands in the 5-8 pp
// range; a density signal (heavy ramp, many wipes) in the 2-4 pp
// range. A clearly-positioned deck ends up with a 6-10 pp swing
// between its best and worst seat; an unfocused deck has a small
// (<3 pp) swing and the recommender effectively says "any seat
// works."
var seatRules = []seatRule{
	{
		Name: "fast_curve",
		Predicate: func(dp *DeckProfile, _ *FreyaReport) bool {
			return dp.AvgCMC > 0 && dp.AvgCMC < 2.5
		},
		Delta:  [4]int{+5, +3, -1, -3},
		Reason: "lean curve plays into the early-seat tempo window",
	},
	{
		Name: "slow_curve",
		Predicate: func(dp *DeckProfile, _ *FreyaReport) bool {
			return dp.AvgCMC > 3.8
		},
		Delta:  [4]int{-3, -1, +2, +4},
		Reason: "heavy curve wants extra turn cycles to assemble — late seats help",
	},
	{
		Name: "heavy_ramp",
		Predicate: func(dp *DeckProfile, _ *FreyaReport) bool {
			return dp.RampCount >= 12
		},
		Delta:  [4]int{+3, +2, 0, -1},
		Reason: "heavy ramp converts early seats into a turn advantage",
	},
	{
		Name: "combo_with_tutors",
		Predicate: func(_ *DeckProfile, report *FreyaReport) bool {
			return len(report.TrueInfinites) > 0 && report.NonLandTutorCount >= 5
		},
		Delta:  [4]int{+4, +2, -1, -3},
		Reason: "combo + tutor density wants to assemble before opponents stabilize",
	},
	{
		Name: "voltron_or_aggro",
		Predicate: func(dp *DeckProfile, _ *FreyaReport) bool {
			arch := strings.ToLower(dp.PrimaryArchetype)
			return strings.Contains(arch, "voltron") ||
				strings.Contains(arch, "aggro") ||
				strings.Contains(arch, "go wide")
		},
		Delta:  [4]int{+3, +2, 0, -2},
		Reason: "combat-pressure decks benefit from landing damage before chump blockers arrive",
	},
	{
		Name: "stax",
		Predicate: func(dp *DeckProfile, _ *FreyaReport) bool {
			return strings.Contains(strings.ToLower(dp.PrimaryArchetype), "stax")
		},
		Delta:  [4]int{+3, +1, 0, -1},
		Reason: "stax wants to lock the table before opponents develop their boards",
	},
	{
		Name: "control_heavy",
		Predicate: func(dp *DeckProfile, report *FreyaReport) bool {
			if strings.Contains(strings.ToLower(dp.PrimaryArchetype), "control") {
				return true
			}
			interaction := report.RemovalCount
			if report.Roles != nil {
				interaction += report.Roles.RoleCounts[RoleCounterspell]
			}
			return interaction >= 12
		},
		Delta:  [4]int{-2, 0, +2, +4},
		Reason: "reactive decks see more before they act — late seats let you answer the right threat",
	},
	{
		Name: "counter_dense",
		Predicate: func(_ *DeckProfile, report *FreyaReport) bool {
			if report.Roles == nil {
				return false
			}
			return report.Roles.RoleCounts[RoleCounterspell] >= 5
		},
		Delta:  [4]int{-3, -1, +1, +3},
		Reason: "counterspell suite is most efficient when you can see what to fight over",
	},
	{
		Name: "many_board_wipes",
		Predicate: func(_ *DeckProfile, report *FreyaReport) bool {
			if report.Roles == nil {
				return false
			}
			return report.Roles.RoleCounts[RoleBoardWipe] >= 3
		},
		Delta:  [4]int{-2, 0, +2, +3},
		Reason: "multiple wipes pay off when the table has built a wider board to reset",
	},
	{
		Name: "reanimator",
		Predicate: func(dp *DeckProfile, _ *FreyaReport) bool {
			return strings.Contains(strings.ToLower(dp.PrimaryArchetype), "reanimator")
		},
		Delta:  [4]int{-2, -1, +1, +2},
		Reason: "reanimator needs setup turns to fill the graveyard — later seats give breathing room",
	},
	{
		Name: "group_hug_or_political",
		Predicate: func(dp *DeckProfile, _ *FreyaReport) bool {
			arch := strings.ToLower(dp.PrimaryArchetype)
			return strings.Contains(arch, "group hug") || strings.Contains(arch, "pillowfort")
		},
		Delta:  [4]int{-2, 0, +1, +2},
		Reason: "political decks read the table before committing — late seats see the threats clearly",
	},
	{
		Name: "commander_centric_slow",
		Predicate: func(dp *DeckProfile, _ *FreyaReport) bool {
			return dp.IsCommanderCentric && dp.AvgTurnToCommander >= 5
		},
		Delta:  [4]int{-2, -1, +1, +2},
		Reason: "commander lands late — extra setup time benefits from later seats",
	},
}

// RecommendSeating produces the per-seat power projection for the deck.
// Nil-safe for empty inputs; returns a zero-deltas result so callers
// can render "no recommendation" cleanly rather than nil-check every
// field.
func RecommendSeating(dp *DeckProfile, report *FreyaReport) *SeatRecommendation {
	if dp == nil {
		return nil
	}
	if report == nil {
		report = &FreyaReport{} // safe-default — rules guard their own field reads
	}
	baseline := dp.PowerPercentile
	if baseline == 0 {
		baseline = 50 // mid-corpus default when the profile hasn't been scored yet
	}

	rec := &SeatRecommendation{
		Seats: make([]SeatScore, 4),
	}
	// Pre-fill seats so reasoning collection can append by index.
	for i := 0; i < 4; i++ {
		rec.Seats[i].Seat = i
	}

	// Apply each rule. Per-rule reason is attached to the most
	// impacted side (positive deltas → boost seats see the reason;
	// negative deltas → penalized seats see it so the user knows why
	// the worst seat is bad).
	for _, rule := range seatRules {
		if !rule.Predicate(dp, report) {
			continue
		}
		// Track the magnitude across the 4 deltas; only attach reason
		// to seats that materially benefit (delta > 0) or suffer
		// (delta < 0). A delta of 0 on a seat means the rule applied
		// but this particular seat is neutral — no reason needed.
		for i, d := range rule.Delta {
			rec.Seats[i].Adjustment += d
			if d > 0 {
				rec.Seats[i].Reasoning = append(rec.Seats[i].Reasoning,
					fmt.Sprintf("+%d: %s", d, rule.Reason))
			} else if d < 0 {
				rec.Seats[i].Reasoning = append(rec.Seats[i].Reasoning,
					fmt.Sprintf("%d: %s", d, rule.Reason))
			}
		}
	}

	for i := range rec.Seats {
		rec.Seats[i].PowerPct = clampPercentile(baseline + rec.Seats[i].Adjustment)
	}

	// Best / worst — break ties by lower seat number for
	// determinism (consistent rendering across runs).
	best := 0
	worst := 0
	for i := 1; i < 4; i++ {
		if rec.Seats[i].PowerPct > rec.Seats[best].PowerPct {
			best = i
		}
		if rec.Seats[i].PowerPct < rec.Seats[worst].PowerPct {
			worst = i
		}
	}
	rec.BestSeat = best
	rec.WorstSeat = worst
	rec.BestPowerDelta = rec.Seats[best].PowerPct - rec.Seats[worst].PowerPct

	rec.Headline = composeSeatHeadline(rec)

	// Sort the reasoning per seat so renders are stable. Magnitude
	// desc, alphabetical tiebreak.
	for i := range rec.Seats {
		sort.SliceStable(rec.Seats[i].Reasoning, func(a, b int) bool {
			return rec.Seats[i].Reasoning[a] < rec.Seats[i].Reasoning[b]
		})
	}
	return rec
}

// composeSeatHeadline produces the user-facing one-liner that the
// Decks-screen panel shows above the per-seat table. Format matches
// the spec exactly: "Your deck is 3pp better at seat 0 vs seat 3 —
// consider sitting seat 0 next pod." When the deck has no
// meaningful preference (delta < 2 pp), the headline says so
// instead of pretending there's a recommendation.
func composeSeatHeadline(rec *SeatRecommendation) string {
	if rec.BestPowerDelta < 2 {
		return "No strong seat preference (all 4 seats score within 2pp) — any seat works for this deck."
	}
	return fmt.Sprintf("Your deck is %dpp better at seat %d vs seat %d — consider sitting seat %d next pod.",
		rec.BestPowerDelta, rec.BestSeat, rec.WorstSeat, rec.BestSeat)
}

// FormatSeatRecommendation renders the recommendation as a compact
// text block for CLI output / Decks-screen text rendering.
func FormatSeatRecommendation(rec *SeatRecommendation) string {
	if rec == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Seating Recommendation\n")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("=", 60))
	fmt.Fprintf(&b, "  %s\n\n", rec.Headline)
	for _, s := range rec.Seats {
		marker := " "
		switch s.Seat {
		case rec.BestSeat:
			marker = "▲"
		case rec.WorstSeat:
			marker = "▼"
		}
		adj := ""
		if s.Adjustment > 0 {
			adj = fmt.Sprintf(" (+%d)", s.Adjustment)
		} else if s.Adjustment < 0 {
			adj = fmt.Sprintf(" (%d)", s.Adjustment)
		}
		fmt.Fprintf(&b, "  %s Seat %d: %d%s\n", marker, s.Seat, s.PowerPct, adj)
	}
	// Reasoning — only show for the best and worst seats. Reasoning
	// is the same content type as a coaching tip's Action; not all
	// 4 seats need to dump 8 lines each.
	if len(rec.Seats[rec.BestSeat].Reasoning) > 0 {
		fmt.Fprintf(&b, "\n  Why seat %d wins:\n", rec.BestSeat)
		for _, r := range rec.Seats[rec.BestSeat].Reasoning {
			fmt.Fprintf(&b, "    %s\n", r)
		}
	}
	if rec.WorstSeat != rec.BestSeat && len(rec.Seats[rec.WorstSeat].Reasoning) > 0 {
		fmt.Fprintf(&b, "\n  Why seat %d loses:\n", rec.WorstSeat)
		for _, r := range rec.Seats[rec.WorstSeat].Reasoning {
			fmt.Fprintf(&b, "    %s\n", r)
		}
	}
	return b.String()
}

// clampPercentile mirrors the [1, 99] clamp used by
// estimatePowerPercentile so seat scores stay on the same axis as the
// deck's baseline.
func clampPercentile(v int) int {
	if v < 1 {
		return 1
	}
	if v > 99 {
		return 99
	}
	return v
}
