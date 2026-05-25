package main

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// makeCorpusReport builds a minimal FreyaReport for corpus-stats
// rollup tests. The aggregator reads DeckProfile fields (Bracket,
// PrimaryArchetype, AvgCMC, ManaBaseGrade, RampCount, DrawCount,
// WinLineCount, SynergyClusters, GameChangerCount) plus a handful
// of FreyaReport fields (NonLandTutorCount, RemovalCount,
// TrueInfinites/Determined/Finishers, Roles.RoleCounts[BoardWipe /
// Counterspell], CurveShape, TotalCards). Caller supplies the
// signal-bearing values; the rest stays zero.
type corpusReportSpec struct {
	bracket         int
	bracketLabel    string
	archetype       string
	avgCMC          float64
	manaGrade       string
	curveShape      string
	rampCount       int
	drawCount       int
	tutors          int
	removal         int
	counterspells   int
	boardWipes      int
	winLines        int
	clusters        int
	gameChangers    int
	totalCards      int
	hasInfinite     bool
	hasDetermined   bool
	hasFinisher     bool
}

func makeCorpusReport(spec corpusReportSpec) *FreyaReport {
	r := &FreyaReport{
		TotalCards:        spec.totalCards,
		AvgCMC:            spec.avgCMC,
		CurveShape:        spec.curveShape,
		NonLandTutorCount: spec.tutors,
		RemovalCount:      spec.removal,
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: spec.counterspells,
				RoleBoardWipe:    spec.boardWipes,
			},
		},
		Profile: &DeckProfile{
			Bracket:          spec.bracket,
			BracketLabel:     spec.bracketLabel,
			PrimaryArchetype: spec.archetype,
			AvgCMC:           spec.avgCMC,
			ManaBaseGrade:    spec.manaGrade,
			RampCount:        spec.rampCount,
			DrawCount:        spec.drawCount,
			WinLineCount:     spec.winLines,
			GameChangerCount: spec.gameChangers,
		},
	}
	// Synergy clusters — the aggregator only counts len(), so empty
	// shells satisfy the requirement.
	for i := 0; i < spec.clusters; i++ {
		r.Profile.SynergyClusters = append(r.Profile.SynergyClusters, SynergyCluster{})
	}
	if spec.hasInfinite {
		r.TrueInfinites = []ComboResult{{}}
	}
	if spec.hasDetermined {
		r.Determined = []ComboResult{{}}
	}
	if spec.hasFinisher {
		r.Finishers = []ComboResult{{}}
	}
	return r
}

func approxEqualCS(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

// TestComputeCorpusStats_HeadlineAverages pins the user-facing
// headline numbers: average bracket, median bracket, average curve,
// average mana grade, most-common archetype. These are the values
// the "across all 1000 decks" header surfaces.
func TestComputeCorpusStats_HeadlineAverages(t *testing.T) {
	reports := []*FreyaReport{
		makeCorpusReport(corpusReportSpec{
			bracket: 3, archetype: "Midrange", avgCMC: 3.4,
			manaGrade: "B", curveShape: "midrange", rampCount: 8,
			drawCount: 9, totalCards: 99,
		}),
		makeCorpusReport(corpusReportSpec{
			bracket: 4, archetype: "Combo", avgCMC: 2.8,
			manaGrade: "A", curveShape: "control", rampCount: 12,
			drawCount: 11, totalCards: 99,
		}),
		makeCorpusReport(corpusReportSpec{
			bracket: 2, archetype: "Midrange", avgCMC: 3.7,
			manaGrade: "C", curveShape: "midrange", rampCount: 6,
			drawCount: 7, totalCards: 99,
		}),
	}
	cs := ComputeCorpusStats(reports)

	if cs.DeckCount != 3 {
		t.Errorf("DeckCount = %d, want 3", cs.DeckCount)
	}
	if cs.CardCount != 297 {
		t.Errorf("CardCount = %d, want 297", cs.CardCount)
	}
	if !approxEqualCS(cs.AvgBracket, 3.0) {
		t.Errorf("AvgBracket = %.4f, want 3.0", cs.AvgBracket)
	}
	if cs.MedianBracket != 3 {
		t.Errorf("MedianBracket = %d, want 3", cs.MedianBracket)
	}
	wantAvgCMC := (3.4 + 2.8 + 3.7) / 3.0
	if !approxEqualCS(cs.AvgCMC, wantAvgCMC) {
		t.Errorf("AvgCMC = %.4f, want %.4f", cs.AvgCMC, wantAvgCMC)
	}
	// Mana grades: A=5, B=4, C=3 → avg = 4.0 → maps to "B"
	if !approxEqualCS(cs.AvgManaBaseScore, 4.0) {
		t.Errorf("AvgManaBaseScore = %.4f, want 4.0", cs.AvgManaBaseScore)
	}
	if cs.MostCommonArchetype != "Midrange" {
		t.Errorf("MostCommonArchetype = %q, want Midrange", cs.MostCommonArchetype)
	}
}

// TestComputeCorpusStats_ArchetypeDistributionSorted pins that the
// archetype distribution is sorted by DeckCount desc, with
// alphabetical tiebreak for stability across runs.
func TestComputeCorpusStats_ArchetypeDistributionSorted(t *testing.T) {
	reports := []*FreyaReport{
		makeCorpusReport(corpusReportSpec{bracket: 3, archetype: "Combo"}),
		makeCorpusReport(corpusReportSpec{bracket: 3, archetype: "Midrange"}),
		makeCorpusReport(corpusReportSpec{bracket: 3, archetype: "Midrange"}),
		makeCorpusReport(corpusReportSpec{bracket: 3, archetype: "Midrange"}),
		makeCorpusReport(corpusReportSpec{bracket: 3, archetype: "Voltron"}),
		makeCorpusReport(corpusReportSpec{bracket: 3, archetype: "Voltron"}),
		makeCorpusReport(corpusReportSpec{bracket: 3, archetype: "Aggro"}), // 1, alphabetical tiebreak below Voltron
	}
	cs := ComputeCorpusStats(reports)
	want := []struct {
		archetype string
		count     int
	}{
		{"Midrange", 3},
		{"Voltron", 2},
		{"Aggro", 1},  // tied with Combo on 1; alphabetical → Aggro first
		{"Combo", 1},
	}
	if len(cs.ArchetypeDistribution) != len(want) {
		t.Fatalf("ArchetypeDistribution length = %d, want %d", len(cs.ArchetypeDistribution), len(want))
	}
	for i, w := range want {
		got := cs.ArchetypeDistribution[i]
		if got.Archetype != w.archetype || got.DeckCount != w.count {
			t.Errorf("[%d] got %s/%d, want %s/%d",
				i, got.Archetype, got.DeckCount, w.archetype, w.count)
		}
	}
	// Percent sanity: sum of percents = 100 (modulo rounding).
	var sumPct float64
	for _, a := range cs.ArchetypeDistribution {
		sumPct += a.Percent
	}
	if math.Abs(sumPct-100.0) > 0.01 {
		t.Errorf("archetype percentages sum to %.4f, want ~100", sumPct)
	}
}

// TestComputeCorpusStats_BracketDistributionAscending pins that the
// bracket histogram is sorted by bracket number ascending — so the
// text render shows B1 → B5 left-to-right. Labels propagate through.
func TestComputeCorpusStats_BracketDistributionAscending(t *testing.T) {
	reports := []*FreyaReport{
		makeCorpusReport(corpusReportSpec{bracket: 4, bracketLabel: "Optimized", archetype: "Combo"}),
		makeCorpusReport(corpusReportSpec{bracket: 2, bracketLabel: "Core", archetype: "Midrange"}),
		makeCorpusReport(corpusReportSpec{bracket: 4, bracketLabel: "Optimized", archetype: "Combo"}),
		makeCorpusReport(corpusReportSpec{bracket: 1, bracketLabel: "Casual", archetype: "Aggro"}),
	}
	cs := ComputeCorpusStats(reports)
	if len(cs.BracketDistribution) != 3 {
		t.Fatalf("BracketDistribution length = %d, want 3 unique brackets",
			len(cs.BracketDistribution))
	}
	for i := 1; i < len(cs.BracketDistribution); i++ {
		if cs.BracketDistribution[i].Bracket <= cs.BracketDistribution[i-1].Bracket {
			t.Errorf("bracket distribution not ascending: [%d]=%d after [%d]=%d",
				i, cs.BracketDistribution[i].Bracket,
				i-1, cs.BracketDistribution[i-1].Bracket)
		}
	}
	// Labels propagate.
	for _, b := range cs.BracketDistribution {
		if b.Bracket == 4 && b.Label != "Optimized" {
			t.Errorf("B4 label = %q, want Optimized", b.Label)
		}
	}
	// Most common: B4 (2 decks).
	if cs.MostCommonBracket != 4 {
		t.Errorf("MostCommonBracket = %d, want 4", cs.MostCommonBracket)
	}
}

// TestComputeCorpusStats_DensityAverages pins the per-deck density
// signal averages (ramp, draw, tutors, interaction, win lines,
// clusters, game changers). Interaction = removal + counterspells
// summed per deck, then averaged.
func TestComputeCorpusStats_DensityAverages(t *testing.T) {
	reports := []*FreyaReport{
		makeCorpusReport(corpusReportSpec{
			bracket: 3, archetype: "Combo",
			rampCount: 10, drawCount: 12, tutors: 5,
			removal: 6, counterspells: 4, // interaction = 10
			winLines: 3, clusters: 2, gameChangers: 1,
		}),
		makeCorpusReport(corpusReportSpec{
			bracket: 3, archetype: "Combo",
			rampCount: 8, drawCount: 8, tutors: 3,
			removal: 4, counterspells: 2, // interaction = 6
			winLines: 1, clusters: 1, gameChangers: 0,
		}),
	}
	cs := ComputeCorpusStats(reports)
	if !approxEqualCS(cs.AvgRamp, 9.0) {
		t.Errorf("AvgRamp = %.4f, want 9.0", cs.AvgRamp)
	}
	if !approxEqualCS(cs.AvgDraw, 10.0) {
		t.Errorf("AvgDraw = %.4f, want 10.0", cs.AvgDraw)
	}
	if !approxEqualCS(cs.AvgTutors, 4.0) {
		t.Errorf("AvgTutors = %.4f, want 4.0", cs.AvgTutors)
	}
	if !approxEqualCS(cs.AvgInteraction, 8.0) {
		t.Errorf("AvgInteraction = %.4f, want 8.0", cs.AvgInteraction)
	}
	if !approxEqualCS(cs.AvgWinLines, 2.0) {
		t.Errorf("AvgWinLines = %.4f, want 2.0", cs.AvgWinLines)
	}
	if !approxEqualCS(cs.AvgClusters, 1.5) {
		t.Errorf("AvgClusters = %.4f, want 1.5", cs.AvgClusters)
	}
	if !approxEqualCS(cs.AvgGameChangers, 0.5) {
		t.Errorf("AvgGameChangers = %.4f, want 0.5", cs.AvgGameChangers)
	}
}

// TestComputeCorpusStats_PresencePercentages pins the % of decks
// carrying combo presence / board wipes / tutors. Used by the
// "presence" section of the text render — answers "what fraction
// of decks in this corpus run combos?"
func TestComputeCorpusStats_PresencePercentages(t *testing.T) {
	reports := []*FreyaReport{
		// 5 decks: 2 with infinite, 1 determined, 3 finishers, 4 wipes, 3 tutors
		makeCorpusReport(corpusReportSpec{
			bracket: 4, archetype: "Combo",
			hasInfinite: true, hasDetermined: true, hasFinisher: true,
			boardWipes: 2, tutors: 5,
		}),
		makeCorpusReport(corpusReportSpec{
			bracket: 4, archetype: "Combo",
			hasInfinite: true, hasFinisher: true,
			boardWipes: 1, tutors: 4,
		}),
		makeCorpusReport(corpusReportSpec{
			bracket: 3, archetype: "Midrange",
			hasFinisher: true, boardWipes: 2, tutors: 2,
		}),
		makeCorpusReport(corpusReportSpec{
			bracket: 2, archetype: "Aggro",
			boardWipes: 1,
		}),
		makeCorpusReport(corpusReportSpec{
			bracket: 1, archetype: "Aggro",
		}),
	}
	cs := ComputeCorpusStats(reports)
	if !approxEqualCS(cs.PctWithInfiniteCombo, 40.0) {
		t.Errorf("PctWithInfiniteCombo = %.1f, want 40.0", cs.PctWithInfiniteCombo)
	}
	if !approxEqualCS(cs.PctWithDeterminedCombo, 20.0) {
		t.Errorf("PctWithDeterminedCombo = %.1f, want 20.0", cs.PctWithDeterminedCombo)
	}
	if !approxEqualCS(cs.PctWithFinisher, 60.0) {
		t.Errorf("PctWithFinisher = %.1f, want 60.0", cs.PctWithFinisher)
	}
	if !approxEqualCS(cs.PctWithBoardWipe, 80.0) {
		t.Errorf("PctWithBoardWipe = %.1f, want 80.0", cs.PctWithBoardWipe)
	}
	if !approxEqualCS(cs.PctWithTutors, 60.0) {
		t.Errorf("PctWithTutors = %.1f, want 60.0", cs.PctWithTutors)
	}
}

// TestComputeCorpusStats_EmptyInputReturnsZero pins safe-default
// behavior: nil / empty reports yields a zero-valued CorpusStats so
// JSON callers always get a parseable shape and PrintCorpusStats
// emits a single line saying so.
func TestComputeCorpusStats_EmptyInputReturnsZero(t *testing.T) {
	cs := ComputeCorpusStats(nil)
	if cs == nil {
		t.Fatal("ComputeCorpusStats(nil) returned nil, want zero-valued struct")
	}
	if cs.DeckCount != 0 || cs.CardCount != 0 {
		t.Errorf("expected zero counts on empty input, got DeckCount=%d CardCount=%d",
			cs.DeckCount, cs.CardCount)
	}
	cs2 := ComputeCorpusStats([]*FreyaReport{})
	if cs2.DeckCount != 0 {
		t.Errorf("empty []reports: DeckCount = %d, want 0", cs2.DeckCount)
	}
	var buf bytes.Buffer
	PrintCorpusStats(&buf, cs)
	if !strings.Contains(buf.String(), "no decks analyzed") {
		t.Errorf("empty-corpus render should say 'no decks analyzed', got %q", buf.String())
	}
}

// TestComputeCorpusStats_UnscoredManaGradeExcluded pins that decks
// missing a ManaBaseGrade (empty string) are excluded from the
// AvgManaBaseScore mean rather than treated as zeros — would otherwise
// pull a fully-A-graded corpus down to 2.5 because half the decks
// haven't been graded yet.
func TestComputeCorpusStats_UnscoredManaGradeExcluded(t *testing.T) {
	reports := []*FreyaReport{
		makeCorpusReport(corpusReportSpec{bracket: 4, archetype: "Combo", manaGrade: "A"}),
		makeCorpusReport(corpusReportSpec{bracket: 4, archetype: "Combo", manaGrade: "A"}),
		makeCorpusReport(corpusReportSpec{bracket: 4, archetype: "Combo", manaGrade: ""}),
	}
	cs := ComputeCorpusStats(reports)
	if !approxEqualCS(cs.AvgManaBaseScore, 5.0) {
		t.Errorf("AvgManaBaseScore = %.4f, want 5.0 (2 graded A, 1 unscored should not pull the mean)",
			cs.AvgManaBaseScore)
	}
	// The unscored deck DOES appear in the grade distribution under
	// the "" key so consumers can see how many decks are still
	// unscored.
	foundUnscored := false
	for _, g := range cs.ManaGradeDistribution {
		if g.Grade == "" {
			foundUnscored = true
			if g.DeckCount != 1 {
				t.Errorf("unscored bucket count = %d, want 1", g.DeckCount)
			}
		}
	}
	if !foundUnscored {
		t.Errorf("expected '' bucket in mana grade distribution for unscored decks")
	}
}

// TestPrintCorpusStats_HeadlineFormat pins the text render: the
// "across N decks" headline + named most-common archetype + numeric
// averages all appear. The user-quoted format string in the spec
// said "across all 1000 analyzed decks, average bracket=3.2,
// average archetype=Midrange, average curve=3.4" — we surface those
// exact signals (worded as a multi-line block, not one comma-line).
func TestPrintCorpusStats_HeadlineFormat(t *testing.T) {
	reports := []*FreyaReport{
		makeCorpusReport(corpusReportSpec{
			bracket: 3, archetype: "Midrange", avgCMC: 3.4,
			manaGrade: "B", curveShape: "midrange",
		}),
		makeCorpusReport(corpusReportSpec{
			bracket: 3, archetype: "Midrange", avgCMC: 3.4,
			manaGrade: "B", curveShape: "midrange",
		}),
		makeCorpusReport(corpusReportSpec{
			bracket: 4, archetype: "Combo", avgCMC: 3.0,
			manaGrade: "A", curveShape: "control",
		}),
	}
	cs := ComputeCorpusStats(reports)
	var buf bytes.Buffer
	PrintCorpusStats(&buf, cs)
	out := buf.String()
	wanted := []string{
		"Corpus Stats (across 3 decks",
		"Avg bracket",
		"Avg curve",
		"Avg mana grade",
		"Most common    : Midrange",
		"midrange", // curve shape appears under "most common shape" or distribution
		"Bracket distribution",
		"Archetype distribution",
	}
	for _, w := range wanted {
		if !strings.Contains(out, w) {
			t.Errorf("PrintCorpusStats output missing %q. Got:\n%s", w, out)
		}
	}
}

// TestCorpusStats_JSONRoundTrip pins the JSON shape since the
// --corpus-stats-out CLI flag emits this directly for downstream
// tooling. Every field round-trips through Marshal+Unmarshal.
func TestCorpusStats_JSONRoundTrip(t *testing.T) {
	reports := []*FreyaReport{
		makeCorpusReport(corpusReportSpec{
			bracket: 3, archetype: "Midrange", avgCMC: 3.2,
			manaGrade: "B", curveShape: "midrange",
			rampCount: 10, drawCount: 10, tutors: 3,
		}),
	}
	cs := ComputeCorpusStats(reports)

	data, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var roundtripped CorpusStats
	if err := json.Unmarshal(data, &roundtripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if roundtripped.DeckCount != cs.DeckCount {
		t.Errorf("DeckCount roundtrip mismatch: %d vs %d", roundtripped.DeckCount, cs.DeckCount)
	}
	if roundtripped.MostCommonArchetype != cs.MostCommonArchetype {
		t.Errorf("MostCommonArchetype roundtrip mismatch: %q vs %q",
			roundtripped.MostCommonArchetype, cs.MostCommonArchetype)
	}
	if !approxEqualCS(roundtripped.AvgBracket, cs.AvgBracket) {
		t.Errorf("AvgBracket roundtrip mismatch: %.4f vs %.4f",
			roundtripped.AvgBracket, cs.AvgBracket)
	}
	// JSON keys downstream tools will parse — fail loudly if a field
	// rename breaks the on-the-wire contract.
	body := string(data)
	wantedKeys := []string{
		`"DeckCount"`, `"AvgBracket"`, `"MedianBracket"`, `"AvgCMC"`,
		`"AvgManaBaseScore"`, `"AvgRamp"`, `"AvgDraw"`, `"AvgTutors"`,
		`"BracketDistribution"`, `"ArchetypeDistribution"`,
		`"MostCommonArchetype"`, `"MostCommonBracket"`,
		`"PctWithInfiniteCombo"`, `"PctWithBoardWipe"`,
	}
	for _, key := range wantedKeys {
		if !strings.Contains(body, key) {
			t.Errorf("CorpusStats JSON missing expected key %s", key)
		}
	}
}

// TestCorpusStats_DegradesGracefullyOnNilProfile pins that reports
// with a nil Profile (an earlier-pipeline failure mode) don't crash
// the aggregator — the raw FreyaReport-level signals still count
// (NonLandTutorCount, RemovalCount, TrueInfinites, etc.) even when
// Profile is missing.
func TestCorpusStats_DegradesGracefullyOnNilProfile(t *testing.T) {
	reports := []*FreyaReport{
		// A report with no profile (broken pipeline).
		{TotalCards: 99, NonLandTutorCount: 5, RemovalCount: 6,
			TrueInfinites: []ComboResult{{}}},
		// A normal report.
		makeCorpusReport(corpusReportSpec{
			bracket: 3, archetype: "Midrange",
			tutors: 2, removal: 4, counterspells: 3,
		}),
	}
	cs := ComputeCorpusStats(reports)
	if cs.DeckCount != 2 {
		t.Errorf("DeckCount = %d, want 2 (nil-profile report still counted)", cs.DeckCount)
	}
	// 1 report has infinite → 50% presence.
	if !approxEqualCS(cs.PctWithInfiniteCombo, 50.0) {
		t.Errorf("PctWithInfiniteCombo = %.1f, want 50.0", cs.PctWithInfiniteCombo)
	}
	// Bracket-derived stats only count the normal report.
	if !approxEqualCS(cs.AvgBracket, 3.0) {
		t.Errorf("AvgBracket should average only profiles with Bracket > 0, got %.4f", cs.AvgBracket)
	}
}
