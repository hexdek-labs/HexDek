package main

import (
	"testing"
)

// 2-card true-infinite Thoracle line is the maximum weight a deck can earn
// per win line — categorical-win class, minimum piece count, full tutor
// coverage. Pinned to 17 so any future re-tuning is forced to update the
// concrete number rather than silently drift.
func TestWinLineWeight_TwoCardThoracleMaxesOut(t *testing.T) {
	wl := &WinLine{
		Pieces: []string{"Thassa's Oracle", "Demonic Consultation"},
		Type:   "infinite",
		Class:  ComboClassLibraryExileWin,
		TutorPaths: []TutorChain{
			{Tutor: "Demonic Tutor", Finds: "Thassa's Oracle", Delivery: "hand", Restricted: "any"},
			{Tutor: "Demonic Tutor", Finds: "Demonic Consultation", Delivery: "hand", Restricted: "any"},
		},
	}
	got := computeWinLineWeight(wl)
	if got != 17 {
		t.Fatalf("Thoracle 2-card infinite weight = %d, want 17 (typeBase 10 + classBonus 5 + pieceMult 1.00 + tutorBonus 2)", got)
	}
}

// A 4-card grindy infinite ETB pile with no tutors lands well below the
// 2-card Thoracle line — same "infinite" type but pieceMultiplier 0.70
// drops it to 8 vs Thoracle's 17. This is the headline ordering fix: a
// flat WinLineCount would have called these equal.
func TestWinLineWeight_FourCardGrindyInfiniteLowerThanTwoCardInfinite(t *testing.T) {
	two := &WinLine{
		Pieces: []string{"A", "B"},
		Type:   "infinite",
		Class:  ComboClassInfiniteETB,
	}
	four := &WinLine{
		Pieces: []string{"A", "B", "C", "D"},
		Type:   "infinite",
		Class:  ComboClassInfiniteETB,
	}
	twoW := computeWinLineWeight(two)
	fourW := computeWinLineWeight(four)
	if !(twoW > fourW) {
		t.Fatalf("expected 2-card infinite weight > 4-card infinite weight; got 2-card=%d 4-card=%d", twoW, fourW)
	}
	if twoW != 12 {
		t.Fatalf("2-card infinite_etb weight = %d, want 12 (10 + 2)", twoW)
	}
	if fourW != 8 {
		t.Fatalf("4-card infinite_etb weight = %d, want 8 (round((10 + 2) * 0.70))", fourW)
	}
}

// Type ordering: infinite > determined > finisher > alt_wincon > combat >
// commander_damage, with all class bonuses zeroed so the ordering reflects
// the typeBase axis alone.
func TestWinLineWeight_TypeBaseOrdering(t *testing.T) {
	mk := func(t string) int {
		return computeWinLineWeight(&WinLine{Pieces: []string{"X", "Y"}, Type: t})
	}
	inf := mk("infinite")
	det := mk("determined")
	fin := mk("finisher")
	alt := mk("alt_wincon")
	cmb := mk("combat")
	cmd := mk("commander_damage")
	if !(inf > det && det > fin && fin > alt && alt > cmb && cmb > cmd) {
		t.Fatalf("type ordering broken: infinite=%d determined=%d finisher=%d alt_wincon=%d combat=%d commander_damage=%d",
			inf, det, fin, alt, cmb, cmd)
	}
	if inf != 10 || det != 7 || fin != 5 || alt != 3 || cmb != 2 || cmd != 1 {
		t.Fatalf("type base values drifted: got infinite=%d (want 10), determined=%d (want 7), finisher=%d (want 5), alt_wincon=%d (want 3), combat=%d (want 2), commander_damage=%d (want 1)",
			inf, det, fin, alt, cmb, cmd)
	}
}

// Class bonuses are tiered: categorical-win classes (library_exile_win /
// infinite_drain / infinite_damage) top the table at +5, prison /
// infinite-mana classes get +4, payoffs +2-3, blink/combat-finisher/
// graveyard-loop +1, and unknown / land-cycle / empty get +0. Pin a
// representative sample so a future class addition forces a conscious
// classification call rather than silently landing at +0.
func TestWinLineWeight_ClassBonusTable(t *testing.T) {
	tests := []struct {
		class string
		want  int
	}{
		{ComboClassLibraryExileWin, 5},
		{ComboClassInfiniteDrain, 5},
		{ComboClassInfiniteDamage, 5},
		{ComboClassInfiniteMana, 4},
		{ComboClassInfiniteMill, 4},
		{ComboClassLockdown, 4},
		{ComboClassStormFinisher, 3},
		{ComboClassManaSink, 3},
		{ComboClassInfiniteETB, 2},
		{ComboClassInfiniteTokens, 2},
		{ComboClassETBPayoff, 2},
		{ComboClassETBDoubler, 2},
		{ComboClassBlinkEngine, 1},
		{ComboClassCombatFinisher, 1},
		{ComboClassGraveyardLoop, 1},
		{ComboClassUnknown, 0},
		{ComboClassLandCycleSynergy, 0},
		{"", 0},
	}
	for _, tc := range tests {
		// 2-card infinite shape so typeBase + classBonus is plainly readable
		// and pieceMultiplier is 1.0 — total weight = 10 + classBonus.
		wl := &WinLine{Pieces: []string{"A", "B"}, Type: "infinite", Class: tc.class}
		got := computeWinLineWeight(wl)
		want := 10 + tc.want
		if got != want {
			t.Errorf("class %q: weight = %d, want %d (typeBase 10 + classBonus %d)",
				tc.class, got, want, tc.want)
		}
	}
}

// Piece-count multiplier: 1.00 / 0.85 / 0.70 / 0.55 for ≤2/3/4/≥5 pieces.
// Same infinite + infinite_damage shape (typeBase 10 + classBonus 5 = 15)
// across all piece counts so the multiplier is the only variable.
func TestWinLineWeight_PieceCountTaper(t *testing.T) {
	mk := func(n int) int {
		pieces := make([]string, n)
		for i := range pieces {
			pieces[i] = "P"
		}
		return computeWinLineWeight(&WinLine{Pieces: pieces, Type: "infinite", Class: ComboClassInfiniteDamage})
	}
	one := mk(1)
	two := mk(2)
	three := mk(3)
	four := mk(4)
	five := mk(5)
	six := mk(6)
	// 1-piece and 2-piece both use pieceMult 1.0 (≤2 branch). 3 → 0.85, 4 → 0.70, 5+ → 0.55.
	if one != 15 {
		t.Errorf("1-piece infinite + infinite_damage weight = %d, want 15", one)
	}
	if two != 15 {
		t.Errorf("2-piece infinite + infinite_damage weight = %d, want 15", two)
	}
	if three != 13 { // round(15 * 0.85) = round(12.75) = 13
		t.Errorf("3-piece weight = %d, want 13 (round(15 * 0.85))", three)
	}
	if four != 11 { // round(15 * 0.70) = round(10.5) = 11
		t.Errorf("4-piece weight = %d, want 11 (round(15 * 0.70))", four)
	}
	if five != 8 { // round(15 * 0.55) = round(8.25) = 8
		t.Errorf("5-piece weight = %d, want 8 (round(15 * 0.55))", five)
	}
	if six != 8 { // 5+ branch — same as 5-piece
		t.Errorf("6-piece weight = %d, want 8 (same 5+ branch as 5-piece)", six)
	}
	if !(two > three && three > four && four > five) {
		t.Fatalf("piece-count taper broken: 2=%d 3=%d 4=%d 5=%d", two, three, four, five)
	}
}

// Tutor bonus caps at +2 and only counts when at least one piece is
// tutorable. A line with no TutorPaths gets +0; a line with every piece
// tutorable gets +2; partial coverage rounds half-up.
func TestWinLineWeight_TutorBonus(t *testing.T) {
	base := &WinLine{Pieces: []string{"A", "B", "C", "D"}, Type: "infinite", Class: ComboClassInfiniteMana}
	// 0 of 4 → +0
	noPaths := *base
	if got := computeWinLineWeight(&noPaths); got != 10 { // round((10 + 4) * 0.70) = 10
		t.Errorf("0/4 tutored weight = %d, want 10 (+0 tutor bonus)", got)
	}
	// 1 of 4 → 1/4 * 2 = 0.5 → rounds to 1
	one := *base
	one.TutorPaths = []TutorChain{{Tutor: "T1", Finds: "A"}}
	if got := computeWinLineWeight(&one); got != 11 { // 10 + 1
		t.Errorf("1/4 tutored weight = %d, want 11 (+1 tutor bonus)", got)
	}
	// 4 of 4 → 1.0 * 2 = 2
	full := *base
	full.TutorPaths = []TutorChain{
		{Tutor: "T", Finds: "A"}, {Tutor: "T", Finds: "B"},
		{Tutor: "T", Finds: "C"}, {Tutor: "T", Finds: "D"},
	}
	if got := computeWinLineWeight(&full); got != 12 { // 10 + 2
		t.Errorf("4/4 tutored weight = %d, want 12 (+2 tutor bonus)", got)
	}
	// Duplicate paths for the same piece don't double-count — coverage is
	// deduped by piece name, so two tutors hitting "A" plus one hitting "B"
	// is still 2/4 coverage.
	dup := *base
	dup.TutorPaths = []TutorChain{
		{Tutor: "T1", Finds: "A"}, {Tutor: "T2", Finds: "A"}, {Tutor: "T3", Finds: "B"},
	}
	if got := computeWinLineWeight(&dup); got != 11 { // 2/4 * 2 = 1, 10 + 1
		t.Errorf("duplicate-tutor 2/4 piece coverage weight = %d, want 11 (+1 tutor bonus, deduped)", got)
	}
}

// Nil safety: a nil pointer should return 0 rather than panic.
func TestWinLineWeight_NilSafe(t *testing.T) {
	if got := computeWinLineWeight(nil); got != 0 {
		t.Fatalf("nil weight = %d, want 0", got)
	}
}

// TotalWeightedScore = sum of per-line Weight values across the analysis.
// Pin a small synthetic analysis with three lines so the sum invariant
// is verifiable without running the full ComputeWinLines pipeline.
func TestWinLineAnalysis_TotalWeightedScoreSumInvariant(t *testing.T) {
	wla := &WinLineAnalysis{
		WinLines: []WinLine{
			{Pieces: []string{"A", "B"}, Type: "infinite", Class: ComboClassLibraryExileWin},
			{Pieces: []string{"C", "D", "E"}, Type: "determined", Class: ComboClassInfiniteMana},
			{Pieces: []string{"F"}, Type: "commander_damage"},
		},
	}
	// Mirror the post-sort pass in ComputeWinLines.
	for i := range wla.WinLines {
		wla.WinLines[i].Weight = computeWinLineWeight(&wla.WinLines[i])
		wla.TotalWeightedScore += wla.WinLines[i].Weight
	}
	w0 := wla.WinLines[0].Weight // 10 + 5 = 15 (2-card Thoracle-shape)
	w1 := wla.WinLines[1].Weight // round((7 + 4) * 0.85) = round(9.35) = 9
	w2 := wla.WinLines[2].Weight // commander_damage typeBase 1, no class, 1-piece (≤2 → 1.0) = 1
	if w0 != 15 {
		t.Errorf("line0 (2-card lib-exile infinite) weight = %d, want 15", w0)
	}
	if w1 != 9 {
		t.Errorf("line1 (3-card determined infinite-mana) weight = %d, want 9", w1)
	}
	if w2 != 1 {
		t.Errorf("line2 (commander_damage) weight = %d, want 1", w2)
	}
	if wla.TotalWeightedScore != w0+w1+w2 {
		t.Fatalf("TotalWeightedScore = %d, want %d (sum invariant broken)", wla.TotalWeightedScore, w0+w1+w2)
	}
}

// Sum-vs-count divergence: a 3-line analysis can have a HIGHER
// TotalWeightedScore than a 1-line analysis (more lines, more weight)
// even when the 1-line analysis carries the BEST individual line. This
// is intentional — the fix isn't "always rank single elite above many
// mediocre"; the fix is to expose BOTH WinLineCount (breadth) and
// TotalWeightedScore (depth × quality) so downstream consumers can
// rank on either axis. Pin the concrete numbers so any silent drift in
// the weight formula surfaces here.
func TestWinLineAnalysis_SumExposesBreadthSeparateFromCount(t *testing.T) {
	elite := &WinLineAnalysis{
		WinLines: []WinLine{
			{
				Pieces: []string{"Thassa's Oracle", "Demonic Consultation"},
				Type:   "infinite",
				Class:  ComboClassLibraryExileWin,
				TutorPaths: []TutorChain{
					{Tutor: "Demonic Tutor", Finds: "Thassa's Oracle"},
					{Tutor: "Demonic Tutor", Finds: "Demonic Consultation"},
				},
			},
		},
	}
	mediocre := &WinLineAnalysis{
		WinLines: []WinLine{
			{Pieces: []string{"A", "B", "C", "D", "E"}, Type: "infinite", Class: ComboClassInfiniteETB},
			{Pieces: []string{"F", "G", "H", "I", "J"}, Type: "infinite", Class: ComboClassInfiniteETB},
			{Pieces: []string{"K", "L", "M", "N", "O"}, Type: "infinite", Class: ComboClassInfiniteETB},
		},
	}
	for _, wla := range []*WinLineAnalysis{elite, mediocre} {
		for i := range wla.WinLines {
			wla.WinLines[i].Weight = computeWinLineWeight(&wla.WinLines[i])
			wla.TotalWeightedScore += wla.WinLines[i].Weight
		}
	}
	if elite.TotalWeightedScore != 17 {
		t.Errorf("elite weighted score = %d, want 17 (single 2-card Thoracle line: 10 + 5 + 2)", elite.TotalWeightedScore)
	}
	// Each 5-card untutored infinite_etb line: round((10 + 2) * 0.55) =
	// round(6.6) = 7. Three of them sum to 21.
	if mediocre.TotalWeightedScore != 21 {
		t.Errorf("mediocre weighted score = %d, want 21 (3 × round((10+2)*0.55))", mediocre.TotalWeightedScore)
	}
	// The two analyses correctly report different signals on each axis:
	// elite is BETTER per-line (17 > 7), mediocre is BIGGER in sum (21 > 17).
	// The pre-fix WinLineCount said mediocre=3 > elite=1, conflating the
	// two — now both signals are surfaced separately.
	if elite.WinLines[0].Weight <= mediocre.WinLines[0].Weight {
		t.Fatalf("elite per-line weight (%d) should exceed mediocre per-line weight (%d) — count-vs-quality axes must diverge", elite.WinLines[0].Weight, mediocre.WinLines[0].Weight)
	}
}

// Per-line ordering invariant: the elite Thoracle line individually
// outweighs each mediocre 5-card line by a wide margin. This is the
// signal downstream consumers (power-percentile, hat MCTS bridge) should
// rank on, alongside the count.
func TestWinLineWeight_PerLineEliteBeatsMediocre(t *testing.T) {
	elite := computeWinLineWeight(&WinLine{
		Pieces: []string{"Thassa's Oracle", "Demonic Consultation"},
		Type:   "infinite",
		Class:  ComboClassLibraryExileWin,
		TutorPaths: []TutorChain{
			{Tutor: "Demonic Tutor", Finds: "Thassa's Oracle"},
			{Tutor: "Demonic Tutor", Finds: "Demonic Consultation"},
		},
	})
	mediocre := computeWinLineWeight(&WinLine{
		Pieces: []string{"A", "B", "C", "D", "E"},
		Type:   "infinite",
		Class:  ComboClassInfiniteETB,
	})
	if !(elite > mediocre*2) {
		t.Fatalf("elite per-line weight (%d) should be >2x mediocre per-line weight (%d) — the per-line signal is the real fix even when sum across many mediocre lines is comparable", elite, mediocre)
	}
}
