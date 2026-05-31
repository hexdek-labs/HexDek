package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestExpectedWinPctForMatchup_StrengthBands pins the ELO-derived
// win-rate output against the strength-band comments on matchupEntry.
// "strong" lands in 70-80, "moderate" in 60-70, "slight" in 52-58,
// "neutral" exactly 50. The unfavored mirror is 100-x for each.
// Drop these bands and the entire downstream tilt/tech pipeline
// uses different priors than the existing documentation claims.
func TestExpectedWinPctForMatchup_StrengthBands(t *testing.T) {
	cases := []struct {
		rating   string
		strength string
		min      int
		max      int
	}{
		{"favored", "strong", 70, 80},
		{"favored", "moderate", 60, 70},
		{"favored", "slight", 52, 58},
		{"favored", "", 60, 70}, // empty strength normalizes to moderate
		{"neutral", "", 50, 50},
		{"unfavored", "strong", 20, 30},
		{"unfavored", "moderate", 30, 40},
		{"unfavored", "slight", 42, 48},
	}
	for _, c := range cases {
		got := expectedWinPctForMatchup(c.rating, c.strength)
		if got < c.min || got > c.max {
			t.Errorf("rating=%q strength=%q: got %d%%, want %d-%d%%", c.rating, c.strength, got, c.min, c.max)
		}
	}
}

// TestExpectedWinPctForMatchup_ReciprocityMirror pins that favored and
// unfavored at the same strength sum to ~100. ELO math guarantees this
// because expectedScore(d) + expectedScore(-d) == 1. The integer
// round-trip can lose 1pp on the boundary so we accept |sum-100| <= 1.
func TestExpectedWinPctForMatchup_ReciprocityMirror(t *testing.T) {
	for _, strength := range []string{"strong", "moderate", "slight"} {
		fav := expectedWinPctForMatchup("favored", strength)
		unf := expectedWinPctForMatchup("unfavored", strength)
		sum := fav + unf
		if sum < 99 || sum > 101 {
			t.Errorf("strength=%q: favored %d%% + unfavored %d%% = %d (want ~100)", strength, fav, unf, sum)
		}
	}
}

// TestEmpiricalWeight_RampShape verifies the linear-ramp empirical
// blend weight is 0 below low, climbs to emMaxWeight at high, and
// stays clamped at emMaxWeight beyond. Pinning the shape protects
// against accidental ramp parameter changes silently shifting the
// blend semantics.
func TestEmpiricalWeight_RampShape(t *testing.T) {
	cases := []struct {
		games  int
		expect float64 // approximate
	}{
		{0, 0.0},
		{5, 0.0},  // below emShrinkLow
		{9, 0.0},  // still below
		{10, 0.0}, // exactly at floor — weight starts at 0 (progress=0)
		{30, 0.4}, // halfway between 10 and 50 → 0.4 weight
		{50, 0.8}, // at ceiling
		{100, 0.8},
		{10000, 0.8},
	}
	for _, c := range cases {
		w := empiricalWeight(c.games)
		// Allow 0.01 slack for float arithmetic.
		if w < c.expect-0.01 || w > c.expect+0.01 {
			t.Errorf("games=%d: weight=%.3f, want ~%.2f", c.games, w, c.expect)
		}
	}
}

// TestBlendEmpirical_LowSampleKeepsBaseline verifies low-sample
// matchups don't perturb the baseline at all. With 5 games the
// blend weight is 0, so blended must equal baseline exactly.
func TestBlendEmpirical_LowSampleKeepsBaseline(t *testing.T) {
	history := &MetaMatchupHistory{
		Pairs: map[string]MatchupRecord{
			"combo|stax": {Games: 5, WinsForFirst: 4},
		},
	}
	blended, games, emp := blendEmpirical(60, "Combo", "Stax", history)
	if blended != 60 {
		t.Errorf("low-sample baseline should be unchanged: got %d, want 60", blended)
	}
	if games != 5 {
		t.Errorf("games=%d, want 5", games)
	}
	// Empirical win % is computed regardless of weight — it's surfaced
	// in the report even when it doesn't move the headline number.
	if emp != 80 {
		t.Errorf("empirical %% should be 4/5=80, got %d", emp)
	}
}

// TestBlendEmpirical_HighSampleShiftsToward verifies high-sample
// matchups shift the headline toward empirical. With 100 games at
// 30% win rate and a 60% baseline, the blend at 0.8 weight should
// land at 0.8*30 + 0.2*60 = 36.
func TestBlendEmpirical_HighSampleShiftsToward(t *testing.T) {
	history := &MetaMatchupHistory{
		Pairs: map[string]MatchupRecord{
			"combo|stax": {Games: 100, WinsForFirst: 30},
		},
	}
	blended, _, emp := blendEmpirical(60, "Combo", "Stax", history)
	if emp != 30 {
		t.Errorf("empirical %% = %d, want 30", emp)
	}
	// Expected: 0.8*30 + 0.2*60 = 36
	if blended < 35 || blended > 37 {
		t.Errorf("blended %% = %d, want ~36", blended)
	}
}

// TestBlendEmpirical_NoHistoryReturnsBaseline confirms the
// nil-history fast path: when LoadedMatchupHistory is nil, blend is
// the identity function on baseline.
func TestBlendEmpirical_NoHistoryReturnsBaseline(t *testing.T) {
	blended, games, emp := blendEmpirical(72, "Combo", "Stax", nil)
	if blended != 72 || games != 0 || emp != 0 {
		t.Errorf("no-history blend should pass through: got (%d, %d, %d), want (72, 0, 0)",
			blended, games, emp)
	}
}

// TestDetectTilt_BelowMinSampleNeverFires guards against tilt
// false-positives on low-sample matchups. Even a 50pp swing on 8
// games is noise, not signal.
func TestDetectTilt_BelowMinSampleNeverFires(t *testing.T) {
	tilted, dir, delta := detectTilt(60, 20, 8)
	if tilted {
		t.Errorf("low-sample tilt should not fire: tilted=%v dir=%q delta=%d", tilted, dir, delta)
	}
}

// TestDetectTilt_OverPerforming verifies a positive empirical-baseline
// gap above the threshold triggers "over" direction with the right
// signed delta.
func TestDetectTilt_OverPerforming(t *testing.T) {
	tilted, dir, delta := detectTilt(50, 75, 30)
	if !tilted {
		t.Errorf("over-perform should be tilted: got tilted=%v", tilted)
	}
	if dir != "over" {
		t.Errorf("direction=%q, want over", dir)
	}
	if delta != 25 {
		t.Errorf("delta=%d, want 25", delta)
	}
}

// TestDetectTilt_UnderPerforming verifies the negative direction.
func TestDetectTilt_UnderPerforming(t *testing.T) {
	tilted, dir, delta := detectTilt(70, 40, 30)
	if !tilted {
		t.Errorf("under-perform should be tilted")
	}
	if dir != "under" {
		t.Errorf("direction=%q, want under", dir)
	}
	if delta != -30 {
		t.Errorf("delta=%d, want -30", delta)
	}
}

// TestDetectTilt_WithinThresholdQuiet verifies a 15pp gap doesn't fire
// tilt — only deltas at or above tiltThresholdPP (20) qualify as
// coaching signal.
func TestDetectTilt_WithinThresholdQuiet(t *testing.T) {
	tilted, _, _ := detectTilt(60, 75, 30)
	if tilted {
		t.Errorf("15pp gap should not fire tilt at 20pp threshold")
	}
}

// TestSuggestTechCardsForArchetype_ReanimatorReturnsExileCards
// verifies the recommender pulls graveyard-exile pieces from
// hoserDB when targeting a Reanimator opponent. The "reanimator"
// hoser condition exists; this test confirms it round-trips into
// suggestions.
func TestSuggestTechCardsForArchetype_ReanimatorReturnsExileCards(t *testing.T) {
	suggestions := suggestTechCardsForArchetype("Reanimator", nil)
	if len(suggestions) == 0 {
		t.Fatal("no tech suggestions for Reanimator opponent")
	}

	wantAny := []string{"Faerie Macabre", "Bojuka Bog", "Rest in Peace", "Leyline of the Void"}
	found := false
	for _, s := range suggestions {
		for _, w := range wantAny {
			if s.Card == w {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("Reanimator suggestions missing canonical graveyard hosers; got %v", suggestions)
	}
}

// TestSuggestTechCardsForArchetype_SortedBySeverityDesc verifies
// critical hosers come before major before minor. Without this the
// report's first listed pick might not be the most impactful one.
func TestSuggestTechCardsForArchetype_SortedBySeverityDesc(t *testing.T) {
	suggestions := suggestTechCardsForArchetype("Combo", nil)
	if len(suggestions) < 2 {
		t.Skip("need >=2 Combo hosers to verify sort")
	}
	if !sort.SliceIsSorted(suggestions, func(i, j int) bool {
		if suggestions[i].Severity != suggestions[j].Severity {
			return suggestions[i].Severity > suggestions[j].Severity
		}
		return suggestions[i].Card < suggestions[j].Card
	}) {
		t.Errorf("suggestions not severity-desc sorted: %v", suggestions)
	}
}

// TestSuggestTechCardsForArchetype_CappedAtLimit verifies the
// recommender never returns more than techSuggestionCap entries —
// downstream callers depend on this for layout budgeting.
func TestSuggestTechCardsForArchetype_CappedAtLimit(t *testing.T) {
	for _, arch := range []string{"Combo", "Reanimator", "Aristocrats", "Storm"} {
		s := suggestTechCardsForArchetype(arch, nil)
		if len(s) > techSuggestionCap {
			t.Errorf("%s: %d suggestions, exceeds cap %d", arch, len(s), techSuggestionCap)
		}
	}
}

// TestSuggestTechCardsForArchetype_AlreadyInDeckFlag verifies the
// AlreadyInDeck flag fires when the deck's profiles include the
// hoser card. Builders see the suggestion is already covered rather
// than the recommender silently dropping it (which would obscure the
// deck's actual coverage).
func TestSuggestTechCardsForArchetype_AlreadyInDeckFlag(t *testing.T) {
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Bojuka Bog"},
		},
	}
	suggestions := suggestTechCardsForArchetype("Reanimator", report)
	foundFlagged := false
	for _, s := range suggestions {
		if s.Card == "Bojuka Bog" {
			if !s.AlreadyInDeck {
				t.Errorf("Bojuka Bog should be flagged AlreadyInDeck=true")
			}
			foundFlagged = true
		}
	}
	if !foundFlagged {
		t.Errorf("Bojuka Bog missing from Reanimator suggestions entirely")
	}
}

// TestWorstMatchupArchetype_PicksLowestExpected verifies the worst-
// matchup selector returns the archetype with the lowest expected
// win % among the matchup list.
func TestWorstMatchupArchetype_PicksLowestExpected(t *testing.T) {
	matchups := []MetaMatchup{
		{Archetype: "Combo", ExpectedWinPct: 65},
		{Archetype: "Stax", ExpectedWinPct: 24},
		{Archetype: "Control", ExpectedWinPct: 50},
	}
	worst := worstMatchupArchetype(matchups)
	if worst != "Stax" {
		t.Errorf("worst=%q, want Stax (lowest at 24%%)", worst)
	}
}

// TestWorstMatchupArchetype_SkipsWhenAllAboveCeiling verifies the
// recommender stays silent when the deck has no genuinely bad
// matchup — there's nothing to fix, so suggesting hate-pieces would
// be noise.
func TestWorstMatchupArchetype_SkipsWhenAllAboveCeiling(t *testing.T) {
	matchups := []MetaMatchup{
		{Archetype: "Combo", ExpectedWinPct: 65},
		{Archetype: "Stax", ExpectedWinPct: 55},
		{Archetype: "Control", ExpectedWinPct: 60},
	}
	worst := worstMatchupArchetype(matchups)
	if worst != "" {
		t.Errorf("worst=%q, want empty (all matchups above 50%% ceiling)", worst)
	}
}

// TestComputeMetaPositioning_EndToEndPopulatesAllFields exercises
// the full pipeline through BuildDeckProfile's metaPositioning call
// site: every MetaMatchup gets ExpectedWinPct populated, and a
// deck with at least one unfavored matchup ends up with TechSuggestions
// and a WorstMatchup label.
func TestComputeMetaPositioning_EndToEndPopulatesAllFields(t *testing.T) {
	saved := LoadedMatchupHistory
	t.Cleanup(func() { LoadedMatchupHistory = saved })
	LoadedMatchupHistory = nil

	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeMetaPositioningWithReport(dp, nil)

	if len(dp.MetaMatchups) == 0 {
		t.Fatal("Combo archetype should have matchup entries in metaMatchupDB")
	}

	for _, m := range dp.MetaMatchups {
		if m.ExpectedWinPct < 0 || m.ExpectedWinPct > 100 {
			t.Errorf("matchup vs %s: ExpectedWinPct=%d out of [0,100]", m.Archetype, m.ExpectedWinPct)
		}
		if m.BaselineWinPct != m.ExpectedWinPct {
			// With no history loaded, baseline should equal expected.
			t.Errorf("matchup vs %s: baseline %d != expected %d (no history loaded)",
				m.Archetype, m.BaselineWinPct, m.ExpectedWinPct)
		}
		if m.Rating == "neutral" && m.ExpectedWinPct != 50 {
			t.Errorf("neutral matchup vs %s should be 50%%, got %d", m.Archetype, m.ExpectedWinPct)
		}
	}

	// Combo's matchupDB has at least one unfavored entry (vs Stax or
	// similar), so worst-matchup detection should fire.
	hasUnfavored := false
	for _, m := range dp.MetaMatchups {
		if m.Rating == "unfavored" {
			hasUnfavored = true
			break
		}
	}
	if hasUnfavored && dp.WorstMatchup == "" {
		t.Errorf("Combo has unfavored matchups but WorstMatchup is empty")
	}
}

// TestLoadMetaMatchupHistory_MissingFileTolerated guards the
// "history is optional" contract — a missing file is not an error,
// just a no-op that leaves LoadedMatchupHistory nil.
func TestLoadMetaMatchupHistory_MissingFileTolerated(t *testing.T) {
	saved := LoadedMatchupHistory
	t.Cleanup(func() { LoadedMatchupHistory = saved })
	LoadedMatchupHistory = nil

	err := LoadMetaMatchupHistory("/nonexistent/path/elo-history.json")
	if err != nil {
		t.Errorf("missing file should not error: %v", err)
	}
	if LoadedMatchupHistory != nil {
		t.Errorf("missing file should leave LoadedMatchupHistory nil")
	}
}

// TestLoadMetaMatchupHistory_ParsesValidJSON verifies the JSON round-
// trip end-to-end via a small tmp file with two pair records.
func TestLoadMetaMatchupHistory_ParsesValidJSON(t *testing.T) {
	saved := LoadedMatchupHistory
	t.Cleanup(func() { LoadedMatchupHistory = saved })
	LoadedMatchupHistory = nil

	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	body := `{"archetype_pairs": {"combo|stax": {"games": 40, "wins_for_first": 12}, "combo|aggro": {"games": 25, "wins_for_first": 18}}}`
	if err := writeTempFile(path, body); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	if err := LoadMetaMatchupHistory(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if LoadedMatchupHistory == nil {
		t.Fatal("LoadedMatchupHistory still nil after successful load")
	}
	rec, ok := LoadedMatchupHistory.Pairs["combo|stax"]
	if !ok {
		t.Errorf("combo|stax key missing")
	}
	if rec.Games != 40 || rec.WinsForFirst != 12 {
		t.Errorf("combo|stax record: %+v want {Games:40 WinsForFirst:12}", rec)
	}
}

// TestComputeMetaPositioning_EmpiricalTiltSurfaces verifies the full
// pipeline detects tilt when history disagrees materially with the
// baseline. Combo decks vs Stax is canonically favored-to-strong in
// metaMatchupDB; if real history says we win only 10% of those, the
// tilt flag should fire as "under".
func TestComputeMetaPositioning_EmpiricalTiltSurfaces(t *testing.T) {
	saved := LoadedMatchupHistory
	t.Cleanup(func() { LoadedMatchupHistory = saved })

	// Combo vs Stax is an unfavored/strong matchup in metaMatchupDB
	// (baseline ~24%). Use 0 wins in 50 games so the gap is large
	// enough (-24pp) to exceed the 20pp tilt threshold regardless of
	// exact baseline calibration.
	LoadedMatchupHistory = &MetaMatchupHistory{
		Pairs: map[string]MatchupRecord{
			"combo|stax": {Games: 50, WinsForFirst: 0},
		},
	}

	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeMetaPositioningWithReport(dp, nil)

	foundTilt := false
	for _, m := range dp.MetaMatchups {
		if canonicaliseMetaArchetypeKey(m.Archetype) != "stax" {
			continue
		}
		if !m.Tilted {
			t.Errorf("Combo vs Stax should be tilted at 10%% empirical vs ~70%% baseline; got %+v", m)
		}
		if m.TiltDirection != "under" {
			t.Errorf("tilt direction=%q, want under", m.TiltDirection)
		}
		if m.EmpiricalGames != 50 {
			t.Errorf("EmpiricalGames=%d, want 50", m.EmpiricalGames)
		}
		foundTilt = true
	}
	if !foundTilt {
		t.Fatal("no Stax matchup found in Combo's matchup list — metaMatchupDB shape changed?")
	}
}

// writeTempFile is a small helper for the load test.
func writeTempFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
