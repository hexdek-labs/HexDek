package analytics

import (
	"math"
	"strings"
	"testing"
)

// keystone_r60_test.go — pins the CardKeystoneImpact math + render
// shape. The metric isolates whether a card is LOAD-BEARING in its
// deck (positive shift = wins lift when card hits play) vs.
// coincidentally present in winning games (zero shift) vs. a bait /
// removal magnet (negative shift).

// keystoneApprox compares two float64s for the small-numeric
// tolerance used by the per-card ratio math.
func keystoneApprox(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// findKeystone is the test-side lookup helper.
func findKeystone(impacts []CardKeystoneImpact, name string) *CardKeystoneImpact {
	for i := range impacts {
		if impacts[i].Name == name {
			return &impacts[i]
		}
	}
	return nil
}

// TestKeystone_LoadBearingCard pins the canonical positive-shift
// case: card cast in 5 of 10 games, wins all 5. In the other 5
// games the card is in CardsPlayed (TurnCast == 0 = stuck in hand /
// countered) and the controller wins only 1. Shift = +80pp.
func TestKeystone_LoadBearingCard(t *testing.T) {
	mk := func(cast bool, won bool) *GameAnalysis {
		turn := 0
		if cast {
			turn = 3
		}
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{
					{Name: "Big Engine", TurnCast: turn},
				},
			}},
		}
	}
	games := []*GameAnalysis{
		mk(true, true), mk(true, true), mk(true, true), mk(true, true), mk(true, true),
		mk(false, true), mk(false, false), mk(false, false), mk(false, false), mk(false, false),
	}
	impacts := ComputeKeystoneImpacts(games)
	k := findKeystone(impacts, "Big Engine")
	if k == nil {
		t.Fatal("Big Engine missing from impacts")
	}
	if k.GamesCast != 5 || k.GamesNotCast != 5 {
		t.Errorf("buckets = (%d cast, %d not-cast), want (5, 5)", k.GamesCast, k.GamesNotCast)
	}
	if !keystoneApprox(k.WinRateWhenCast, 1.0) {
		t.Errorf("WinRateWhenCast = %f, want 1.0", k.WinRateWhenCast)
	}
	if !keystoneApprox(k.WinRateWhenNotCast, 0.2) {
		t.Errorf("WinRateWhenNotCast = %f, want 0.2", k.WinRateWhenNotCast)
	}
	if !keystoneApprox(k.Shift, 0.8) {
		t.Errorf("Shift = %f, want +0.8 (+80pp)", k.Shift)
	}
	if k.Confidence != "medium" {
		// min bucket = 5 → medium confidence.
		t.Errorf("Confidence = %q, want %q (min bucket = 5)", k.Confidence, "medium")
	}
}

// TestKeystone_NegativeShiftRedFlag covers the bait/overcommit case:
// card cast in 4 games, won 1 (25%). Not cast in 4 games, won 3
// (75%). Shift = -50pp — the deck loses more when this card hits
// play. Should still be ranked (negative shifts are useful signal).
func TestKeystone_NegativeShiftRedFlag(t *testing.T) {
	mk := func(cast bool, won bool) *GameAnalysis {
		turn := 0
		if cast {
			turn = 2
		}
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{
					{Name: "Bait Card", TurnCast: turn},
				},
			}},
		}
	}
	games := []*GameAnalysis{
		mk(true, true), mk(true, false), mk(true, false), mk(true, false),
		mk(false, true), mk(false, true), mk(false, true), mk(false, false),
	}
	impacts := ComputeKeystoneImpacts(games)
	k := findKeystone(impacts, "Bait Card")
	if k == nil {
		t.Fatal("Bait Card missing")
	}
	if !keystoneApprox(k.Shift, -0.5) {
		t.Errorf("Shift = %f, want -0.5", k.Shift)
	}
}

// TestKeystone_NoSignalWhenAlwaysCast covers the always-cast case:
// the card was cast in every game so GamesNotCast == 0 → no
// counterfactual → shift cannot be computed. Confidence should be
// empty (no signal) and Shift left at 0.
func TestKeystone_NoSignalWhenAlwaysCast(t *testing.T) {
	mk := func(won bool) *GameAnalysis {
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{
					{Name: "Always Cast", TurnCast: 1},
				},
			}},
		}
	}
	impacts := ComputeKeystoneImpacts([]*GameAnalysis{
		mk(true), mk(true), mk(false), mk(false),
	})
	k := findKeystone(impacts, "Always Cast")
	if k == nil {
		t.Fatal("Always Cast missing")
	}
	if k.GamesCast != 4 || k.GamesNotCast != 0 {
		t.Errorf("buckets = (%d cast, %d not-cast), want (4, 0)", k.GamesCast, k.GamesNotCast)
	}
	if k.Confidence != "" {
		t.Errorf("Confidence = %q, want \"\" (no counterfactual)", k.Confidence)
	}
	if k.Shift != 0 {
		t.Errorf("Shift = %f, want 0 (no counterfactual)", k.Shift)
	}
}

// TestKeystone_DedupesAcrossMirrorSeats pins the per-game dedupe.
// Game 1: two seats both have "Same Card" in CardsPlayed; seat 0
// cast it, seat 1 didn't, seat 0 won. The game should count ONCE in
// GamesCast (with the win flag), NOT once in each bucket.
func TestKeystone_DedupesAcrossMirrorSeats(t *testing.T) {
	games := []*GameAnalysis{
		{
			Players: []PlayerAnalysis{
				{
					Won: true,
					CardsPlayed: []CardPerformance{
						{Name: "Same Card", TurnCast: 2},
					},
				},
				{
					Won: false,
					CardsPlayed: []CardPerformance{
						{Name: "Same Card", TurnCast: 0},
					},
				},
			},
		},
	}
	impacts := ComputeKeystoneImpacts(games)
	k := findKeystone(impacts, "Same Card")
	if k == nil {
		t.Fatal("missing")
	}
	if k.GamesCast != 1 {
		t.Errorf("GamesCast = %d, want 1 (per-game dedupe — cast bucket wins ties)", k.GamesCast)
	}
	if k.GamesNotCast != 0 {
		t.Errorf("GamesNotCast = %d, want 0 (game counts as cast since any seat cast it)", k.GamesNotCast)
	}
	if k.WinsCast != 1 {
		t.Errorf("WinsCast = %d, want 1", k.WinsCast)
	}
}

// TestKeystone_ConfidenceTiers pins the thresholds: <5 = low,
// 5-19 = medium, ≥20 = high. Tested by varying the min(cast, not)
// across three runs.
func TestKeystone_ConfidenceTiers(t *testing.T) {
	mk := func(name string, cast bool, won bool) *GameAnalysis {
		turn := 0
		if cast {
			turn = 1
		}
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{
					{Name: name, TurnCast: turn},
				},
			}},
		}
	}
	cases := []struct {
		castN, notCastN int
		wantConfidence  string
	}{
		{3, 3, "low"},
		{5, 5, "medium"},
		{12, 8, "medium"}, // min bucket = 8 → medium
		{25, 22, "high"},
	}
	for _, c := range cases {
		games := make([]*GameAnalysis, 0, c.castN+c.notCastN)
		name := "TierCard"
		for i := 0; i < c.castN; i++ {
			games = append(games, mk(name, true, false))
		}
		for i := 0; i < c.notCastN; i++ {
			games = append(games, mk(name, false, false))
		}
		impacts := ComputeKeystoneImpacts(games)
		k := findKeystone(impacts, name)
		if k == nil {
			t.Errorf("case (%d, %d): missing", c.castN, c.notCastN)
			continue
		}
		if k.Confidence != c.wantConfidence {
			t.Errorf("case (%d cast, %d not-cast): Confidence = %q, want %q",
				c.castN, c.notCastN, k.Confidence, c.wantConfidence)
		}
	}
}

// TestKeystone_ExcludesLandsAndTokens pins the lands/tokens filter:
// lands and tokens never reach either bucket, regardless of TurnCast.
func TestKeystone_ExcludesLandsAndTokens(t *testing.T) {
	games := []*GameAnalysis{
		{
			Players: []PlayerAnalysis{{
				Won: true,
				CardsPlayed: []CardPerformance{
					{Name: "Forest", TurnCast: 1, IsLand: true},
					{Name: "Soldier Token", TurnCast: 2, IsToken: true},
				},
			}},
		},
	}
	impacts := ComputeKeystoneImpacts(games)
	for _, name := range []string{"Forest", "Soldier Token"} {
		k := findKeystone(impacts, name)
		if k != nil && (k.GamesCast > 0 || k.GamesNotCast > 0) {
			t.Errorf("%s: should be excluded; got %+v", name, k)
		}
	}
}

// TestKeystone_SortByAbsoluteShift pins the sort key: signal rows
// first (sorted by |Shift| descending then GamesCast desc), no-signal
// rows last (alphabetical).
func TestKeystone_SortByAbsoluteShift(t *testing.T) {
	mk := func(name string, cast bool, won bool) *GameAnalysis {
		turn := 0
		if cast {
			turn = 1
		}
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{
					{Name: name, TurnCast: turn},
				},
			}},
		}
	}
	games := []*GameAnalysis{}
	// "BigPos": +60pp shift (5 cast/5 wins, 5 not cast/2 wins).
	for i := 0; i < 5; i++ {
		games = append(games, mk("BigPos", true, true))
	}
	for i := 0; i < 5; i++ {
		games = append(games, mk("BigPos", false, i < 2))
	}
	// "BigNeg": -50pp shift (4 cast/1 win, 4 not/3 wins).
	for i := 0; i < 4; i++ {
		games = append(games, mk("BigNeg", true, i == 0))
	}
	for i := 0; i < 4; i++ {
		games = append(games, mk("BigNeg", false, i < 3))
	}
	// We need each card to appear in the SAME slice of games. The mk
	// helper above creates a different game per card — to avoid the
	// dedupe semantics interfering, re-build with both cards in each
	// game.
	games = nil
	add := func(name string, n int, castWon, notWon int, castTotal, notTotal int) {
		for i := 0; i < castTotal; i++ {
			won := i < castWon
			games = append(games, mk(name, true, won))
		}
		for i := 0; i < notTotal; i++ {
			won := i < notWon
			games = append(games, mk(name, false, won))
		}
		_ = n
	}
	add("BigPos", 10, 5, 2, 5, 5) // shift = 1.0 - 0.4 = +0.6
	add("BigNeg", 8, 1, 3, 4, 4)  // shift = 0.25 - 0.75 = -0.5
	add("Never", 10, 0, 0, 0, 10) // no-signal

	impacts := ComputeKeystoneImpacts(games)

	// Sanity check the order.
	if len(impacts) < 3 {
		t.Fatalf("expected at least 3 entries; got %d", len(impacts))
	}
	if impacts[0].Name != "BigPos" {
		t.Errorf("rank 1 = %q, want BigPos", impacts[0].Name)
	}
	if impacts[1].Name != "BigNeg" {
		t.Errorf("rank 2 = %q, want BigNeg", impacts[1].Name)
	}
	// Last entry should be no-signal (Confidence == "").
	last := impacts[len(impacts)-1]
	if last.Name != "Never" {
		t.Errorf("last = %q, want Never (no-signal tail)", last.Name)
	}
	if last.Confidence != "" {
		t.Errorf("Never Confidence = %q, want \"\"", last.Confidence)
	}
}

// TestKeystone_EmptyInputReturnsNil pins the nil guard.
func TestKeystone_EmptyInputReturnsNil(t *testing.T) {
	if got := ComputeKeystoneImpacts(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
	if got := ComputeKeystoneImpacts([]*GameAnalysis{}); got != nil {
		t.Errorf("empty slice: got %v, want nil", got)
	}
}

// TestWriteKeystoneImpacts_RenderShape pins the markdown surface:
// section header, column layout, sign-prefix for the shift, and the
// confidence-filter footnote.
func TestWriteKeystoneImpacts_RenderShape(t *testing.T) {
	mk := func(name string, cast bool, won bool) *GameAnalysis {
		turn := 0
		if cast {
			turn = 1
		}
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{
					{Name: name, TurnCast: turn},
				},
			}},
		}
	}
	// Build a 20-game "Hot Card": 10 cast / 10 wins, 10 not-cast / 0 wins → shift = +1.0.
	games := make([]*GameAnalysis, 0, 20)
	for i := 0; i < 10; i++ {
		games = append(games, mk("Hot Card", true, true))
	}
	for i := 0; i < 10; i++ {
		games = append(games, mk("Hot Card", false, false))
	}
	impacts := ComputeKeystoneImpacts(games)
	r := &AnalyticsReport{
		Analyses:        games,
		KeystoneImpacts: impacts,
		TotalGames:      20,
	}
	var b strings.Builder
	r.writeKeystoneImpacts(&b, 10)
	out := b.String()

	if !strings.Contains(out, "## Keystone Impact") {
		t.Errorf("missing section header; got:\n%s", out)
	}
	for _, col := range []string{"| Rank |", "| Shift |", "Win % Cast", "Win % Not Cast", "Games Cast", "Games Not Cast", "Confidence"} {
		if !strings.Contains(out, col) {
			t.Errorf("missing column %q in:\n%s", col, out)
		}
	}
	// Sign-prefixed shift, two integer percentages.
	if !strings.Contains(out, "+100pp") {
		t.Errorf("expected +100pp shift; got:\n%s", out)
	}
	if !strings.Contains(out, "| 1 | Hot Card |") {
		t.Errorf("Hot Card on rank 1 missing; got:\n%s", out)
	}
	// Footnote about filtered rows.
	if !strings.Contains(out, "Filtered:") {
		t.Errorf("missing filter footnote; got:\n%s", out)
	}
}

// TestWriteKeystoneImpacts_NoData pins the nil-impacts guard.
func TestWriteKeystoneImpacts_NoData(t *testing.T) {
	r := &AnalyticsReport{}
	var b strings.Builder
	r.writeKeystoneImpacts(&b, 10)
	out := b.String()
	if !strings.Contains(out, "## Keystone Impact") {
		t.Errorf("missing header even when empty")
	}
	if !strings.Contains(out, "No keystone data") {
		t.Errorf("expected nil-data message; got:\n%s", out)
	}
}

// TestWriteKeystoneImpacts_FiltersLowConfidence pins that "low"
// confidence rows are dropped from the rendered table and counted in
// the footnote. Two cards: one high-confidence (20 in each bucket),
// one low-confidence (3 in each). Only the first appears in the
// table; the footnote reports 1 low-confidence drop.
func TestWriteKeystoneImpacts_FiltersLowConfidence(t *testing.T) {
	mk := func(name string, cast bool, won bool) *GameAnalysis {
		turn := 0
		if cast {
			turn = 1
		}
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{
					{Name: name, TurnCast: turn},
				},
			}},
		}
	}
	games := make([]*GameAnalysis, 0)
	for i := 0; i < 20; i++ {
		games = append(games, mk("HighCard", true, true))
	}
	for i := 0; i < 20; i++ {
		games = append(games, mk("HighCard", false, false))
	}
	for i := 0; i < 3; i++ {
		games = append(games, mk("LowCard", true, true))
	}
	for i := 0; i < 3; i++ {
		games = append(games, mk("LowCard", false, false))
	}
	impacts := ComputeKeystoneImpacts(games)
	r := &AnalyticsReport{
		Analyses:        games,
		KeystoneImpacts: impacts,
		TotalGames:      len(games),
	}
	var b strings.Builder
	r.writeKeystoneImpacts(&b, 10)
	out := b.String()

	if !strings.Contains(out, "HighCard") {
		t.Errorf("HighCard should render; got:\n%s", out)
	}
	if strings.Contains(out, "| LowCard |") {
		t.Errorf("LowCard (low confidence) should be filtered out; got:\n%s", out)
	}
	if !strings.Contains(out, "1 low-confidence") {
		t.Errorf("expected '1 low-confidence' in footnote; got:\n%s", out)
	}
}
