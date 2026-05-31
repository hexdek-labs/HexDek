package analytics

import (
	"math"
	"strings"
	"testing"
)

// threat_resolution_r60_test.go — pins the per-commander
// ThreatResolution math + render surface.

func trApprox(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

func findThreat(trs []ThreatResolution, name string) *ThreatResolution {
	for i := range trs {
		if trs[i].CommanderName == name {
			return &trs[i]
		}
	}
	return nil
}

// mkGameWithCommander builds a minimal GameAnalysis where the named
// commander sits in seat 0 with the specified resolution turn (0 = not
// resolved). Seat 0 wins iff `won` is true.
func mkGameWithCommander(commander string, resolutionTurn int, won bool) *GameAnalysis {
	pa := PlayerAnalysis{
		CommanderName: commander,
		Won:           won,
	}
	if resolutionTurn > 0 {
		pa.CardsPlayed = []CardPerformance{{Name: commander, TurnCast: resolutionTurn}}
	}
	return &GameAnalysis{
		Players: []PlayerAnalysis{pa},
	}
}

// TestThreatResolution_PositiveShift covers a load-bearing commander:
// 10 games, 6 resolved (4 wins), 4 unresolved (0 wins) → WinRateResolved =
// 0.667, WinRateUnresolved = 0.0, Shift = +66.7pp.
func TestThreatResolution_PositiveShift(t *testing.T) {
	games := []*GameAnalysis{
		mkGameWithCommander("Big Threat", 4, true),
		mkGameWithCommander("Big Threat", 5, true),
		mkGameWithCommander("Big Threat", 3, true),
		mkGameWithCommander("Big Threat", 6, true),
		mkGameWithCommander("Big Threat", 7, false),
		mkGameWithCommander("Big Threat", 4, false),
		mkGameWithCommander("Big Threat", 0, false),
		mkGameWithCommander("Big Threat", 0, false),
		mkGameWithCommander("Big Threat", 0, false),
		mkGameWithCommander("Big Threat", 0, false),
	}
	trs := ComputeThreatResolutions(games)
	tr := findThreat(trs, "Big Threat")
	if tr == nil {
		t.Fatal("Big Threat missing")
	}
	if tr.GamesResolved != 6 || tr.GamesUnresolved != 4 {
		t.Errorf("buckets = (%d resolved, %d unresolved), want (6, 4)",
			tr.GamesResolved, tr.GamesUnresolved)
	}
	if tr.WinsResolved != 4 || tr.WinsUnresolved != 0 {
		t.Errorf("wins = (%d resolved, %d unresolved), want (4, 0)",
			tr.WinsResolved, tr.WinsUnresolved)
	}
	if !trApprox(tr.WinRateResolved, 4.0/6.0) {
		t.Errorf("WinRateResolved = %f, want %f", tr.WinRateResolved, 4.0/6.0)
	}
	if !trApprox(tr.WinRateUnresolved, 0.0) {
		t.Errorf("WinRateUnresolved = %f, want 0.0", tr.WinRateUnresolved)
	}
	if !trApprox(tr.Shift, 4.0/6.0) {
		t.Errorf("Shift = %f, want %f", tr.Shift, 4.0/6.0)
	}
	// First-resolution turns: 4,5,3,6,7,4 → mean = 29/6.
	wantAvg := 29.0 / 6.0
	if !trApprox(tr.AvgTurnToFirstResolution, wantAvg) {
		t.Errorf("AvgTurnToFirstResolution = %f, want %f", tr.AvgTurnToFirstResolution, wantAvg)
	}
	// min bucket = 4 (4 unresolved) → "low" confidence.
	if tr.Confidence != "low" {
		t.Errorf("Confidence = %q, want low (min bucket = 4)", tr.Confidence)
	}
}

// TestThreatResolution_NegativeShift pins the red-flag case: a
// brittle commander that the deck wins WITHOUT more often than WITH.
func TestThreatResolution_NegativeShift(t *testing.T) {
	games := []*GameAnalysis{
		mkGameWithCommander("Glass Cannon", 3, false),
		mkGameWithCommander("Glass Cannon", 4, false),
		mkGameWithCommander("Glass Cannon", 5, false),
		mkGameWithCommander("Glass Cannon", 6, false),
		mkGameWithCommander("Glass Cannon", 7, true),
		mkGameWithCommander("Glass Cannon", 0, true),
		mkGameWithCommander("Glass Cannon", 0, true),
		mkGameWithCommander("Glass Cannon", 0, true),
		mkGameWithCommander("Glass Cannon", 0, false),
		mkGameWithCommander("Glass Cannon", 0, false),
	}
	trs := ComputeThreatResolutions(games)
	tr := findThreat(trs, "Glass Cannon")
	if tr == nil {
		t.Fatal("Glass Cannon missing")
	}
	// 5 resolved / 1 win = 20%; 5 unresolved / 3 wins = 60% → shift = -40pp.
	if !trApprox(tr.Shift, -0.4) {
		t.Errorf("Shift = %f, want -0.4", tr.Shift)
	}
	if tr.Confidence != "medium" {
		t.Errorf("Confidence = %q, want medium (min bucket = 5)", tr.Confidence)
	}
}

// TestThreatResolution_AlwaysResolves_NoSignal pins the
// no-counterfactual guard: commander resolves in every game →
// GamesUnresolved == 0 → no shift signal, Confidence empty.
func TestThreatResolution_AlwaysResolves_NoSignal(t *testing.T) {
	games := []*GameAnalysis{
		mkGameWithCommander("Always Casts", 3, true),
		mkGameWithCommander("Always Casts", 4, true),
		mkGameWithCommander("Always Casts", 5, false),
	}
	trs := ComputeThreatResolutions(games)
	tr := findThreat(trs, "Always Casts")
	if tr == nil {
		t.Fatal("missing")
	}
	if tr.GamesResolved != 3 || tr.GamesUnresolved != 0 {
		t.Errorf("buckets = (%d, %d), want (3, 0)", tr.GamesResolved, tr.GamesUnresolved)
	}
	if tr.Confidence != "" {
		t.Errorf("Confidence = %q, want \"\" (no counterfactual)", tr.Confidence)
	}
	if tr.Shift != 0 {
		t.Errorf("Shift = %f, want 0 (no counterfactual)", tr.Shift)
	}
}

// TestThreatResolution_NeverResolves_NoSignal pins the inverse no-
// signal case: commander never resolved in any analyzed game.
func TestThreatResolution_NeverResolves_NoSignal(t *testing.T) {
	games := []*GameAnalysis{
		mkGameWithCommander("Mana Screwed", 0, false),
		mkGameWithCommander("Mana Screwed", 0, false),
		mkGameWithCommander("Mana Screwed", 0, true),
	}
	trs := ComputeThreatResolutions(games)
	tr := findThreat(trs, "Mana Screwed")
	if tr == nil {
		t.Fatal("missing")
	}
	if tr.GamesResolved != 0 {
		t.Errorf("GamesResolved = %d, want 0", tr.GamesResolved)
	}
	if tr.Confidence != "" {
		t.Errorf("Confidence = %q, want \"\"", tr.Confidence)
	}
}

// TestThreatResolution_ConfidenceTiers pins the same thresholds the
// keystone metric uses (high ≥20, medium ≥5, low <5; shares
// keystoneConfidence implementation).
func TestThreatResolution_ConfidenceTiers(t *testing.T) {
	cases := []struct {
		resN, unresN   int
		wantConfidence string
	}{
		{3, 3, "low"},
		{5, 5, "medium"},
		{12, 8, "medium"},
		{25, 22, "high"},
	}
	for _, c := range cases {
		games := make([]*GameAnalysis, 0, c.resN+c.unresN)
		name := "TierCmdr"
		for i := 0; i < c.resN; i++ {
			games = append(games, mkGameWithCommander(name, 4, false))
		}
		for i := 0; i < c.unresN; i++ {
			games = append(games, mkGameWithCommander(name, 0, false))
		}
		trs := ComputeThreatResolutions(games)
		tr := findThreat(trs, name)
		if tr == nil {
			t.Errorf("case (%d, %d) missing", c.resN, c.unresN)
			continue
		}
		if tr.Confidence != c.wantConfidence {
			t.Errorf("case (%d res / %d unres): Confidence = %q, want %q",
				c.resN, c.unresN, tr.Confidence, c.wantConfidence)
		}
	}
}

// TestThreatResolution_EmptyInput pins the nil guard.
func TestThreatResolution_EmptyInput(t *testing.T) {
	if got := ComputeThreatResolutions(nil); got != nil {
		t.Errorf("nil: got %v, want nil", got)
	}
	if got := ComputeThreatResolutions([]*GameAnalysis{}); got != nil {
		t.Errorf("empty: got %v, want nil", got)
	}
}

// TestThreatResolution_SortByAbsoluteShift mirrors the keystone sort
// test: signal rows first by absolute shift desc, no-signal at the
// tail alphabetical.
func TestThreatResolution_SortByAbsoluteShift(t *testing.T) {
	games := []*GameAnalysis{}
	// BigPos +60pp: 5 res/all win, 5 unres/2 wins.
	for i := 0; i < 5; i++ {
		games = append(games, mkGameWithCommander("BigPos", 4, true))
	}
	for i := 0; i < 5; i++ {
		games = append(games, mkGameWithCommander("BigPos", 0, i < 2))
	}
	// BigNeg -50pp: 4 res/1 win, 4 unres/3 wins.
	for i := 0; i < 4; i++ {
		games = append(games, mkGameWithCommander("BigNeg", 3, i == 0))
	}
	for i := 0; i < 4; i++ {
		games = append(games, mkGameWithCommander("BigNeg", 0, i < 3))
	}
	// Never (no-signal): always resolved.
	for i := 0; i < 4; i++ {
		games = append(games, mkGameWithCommander("Never", 5, true))
	}
	trs := ComputeThreatResolutions(games)
	if len(trs) < 3 {
		t.Fatalf("expected ≥3 entries; got %d", len(trs))
	}
	if trs[0].CommanderName != "BigPos" {
		t.Errorf("rank 1 = %q, want BigPos", trs[0].CommanderName)
	}
	if trs[1].CommanderName != "BigNeg" {
		t.Errorf("rank 2 = %q, want BigNeg", trs[1].CommanderName)
	}
	last := trs[len(trs)-1]
	if last.CommanderName != "Never" {
		t.Errorf("last = %q, want Never (no-signal tail)", last.CommanderName)
	}
}

// TestWriteThreatResolutions_RenderShape pins the markdown surface:
// section header, sign prefix, AvgTurn column, footnote.
func TestWriteThreatResolutions_RenderShape(t *testing.T) {
	games := []*GameAnalysis{}
	for i := 0; i < 10; i++ {
		games = append(games, mkGameWithCommander("Hot Cmdr", 4, true))
	}
	for i := 0; i < 10; i++ {
		games = append(games, mkGameWithCommander("Hot Cmdr", 0, false))
	}
	trs := ComputeThreatResolutions(games)
	r := &AnalyticsReport{
		Analyses:          games,
		ThreatResolutions: trs,
		TotalGames:        20,
	}
	var b strings.Builder
	r.writeThreatResolutions(&b, 10)
	out := b.String()

	if !strings.Contains(out, "## Threat Resolution Attribution") {
		t.Errorf("missing section header; got:\n%s", out)
	}
	for _, col := range []string{"| Rank |", "Commander", "Shift", "Win % Resolved", "Win % Unresolved", "Games Resolved", "Games Unresolved", "Avg Turn 1st Res.", "Confidence"} {
		if !strings.Contains(out, col) {
			t.Errorf("missing column %q in:\n%s", col, out)
		}
	}
	if !strings.Contains(out, "+100pp") {
		t.Errorf("expected +100pp shift; got:\n%s", out)
	}
	if !strings.Contains(out, "| 1 | Hot Cmdr |") {
		t.Errorf("Hot Cmdr rank 1 missing; got:\n%s", out)
	}
	if !strings.Contains(out, "Filtered:") {
		t.Errorf("missing filter footnote; got:\n%s", out)
	}
}

// TestWriteThreatResolutions_NoData pins the nil-data guard.
func TestWriteThreatResolutions_NoData(t *testing.T) {
	var b strings.Builder
	(&AnalyticsReport{}).writeThreatResolutions(&b, 10)
	out := b.String()
	if !strings.Contains(out, "## Threat Resolution Attribution") {
		t.Errorf("missing header; got:\n%s", out)
	}
	if !strings.Contains(out, "No threat-resolution data") {
		t.Errorf("expected nil-data message; got:\n%s", out)
	}
}

// TestWriteThreatResolutions_FiltersLowConfidence pins the
// low-confidence filter + footnote count, mirroring the keystone
// behavior.
func TestWriteThreatResolutions_FiltersLowConfidence(t *testing.T) {
	games := []*GameAnalysis{}
	for i := 0; i < 20; i++ {
		games = append(games, mkGameWithCommander("HighCmdr", 4, true))
	}
	for i := 0; i < 20; i++ {
		games = append(games, mkGameWithCommander("HighCmdr", 0, false))
	}
	for i := 0; i < 3; i++ {
		games = append(games, mkGameWithCommander("LowCmdr", 4, true))
	}
	for i := 0; i < 3; i++ {
		games = append(games, mkGameWithCommander("LowCmdr", 0, false))
	}
	trs := ComputeThreatResolutions(games)
	r := &AnalyticsReport{
		Analyses:          games,
		ThreatResolutions: trs,
		TotalGames:        len(games),
	}
	var b strings.Builder
	r.writeThreatResolutions(&b, 10)
	out := b.String()
	if !strings.Contains(out, "HighCmdr") {
		t.Errorf("HighCmdr should render; got:\n%s", out)
	}
	if strings.Contains(out, "| LowCmdr |") {
		t.Errorf("LowCmdr should be filtered; got:\n%s", out)
	}
	if !strings.Contains(out, "1 low-confidence") {
		t.Errorf("expected '1 low-confidence' in footnote; got:\n%s", out)
	}
}
