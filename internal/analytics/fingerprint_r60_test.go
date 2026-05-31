package analytics

import (
	"math"
	"strings"
	"testing"
)

// fingerprint_r60_test.go — pins DeckFingerprint aggregation,
// ClassifyArchetype heuristic, LookupFingerprint, and the
// `## Deck Fingerprints` report renderer.

func fpApprox(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

func findFingerprint(fps []DeckFingerprint, name string) *DeckFingerprint {
	for i := range fps {
		if fps[i].CommanderName == name {
			return &fps[i]
		}
	}
	return nil
}

// mkFpGame builds a minimal GameAnalysis suitable for fingerprint
// aggregation: one seat with the given commander, with the listed
// cards in CardsPlayed at the given turns, and the winner flag.
// FirstBlood and ComboAssembled can be 0 to leave them unobserved.
func mkFpGame(commander string, won bool, winCondition string, firstBlood, comboAssembled, totalTurns int, cards ...CardPerformance) *GameAnalysis {
	return &GameAnalysis{
		WinCondition:   winCondition,
		FirstBlood:     firstBlood,
		ComboAssembled: comboAssembled,
		TotalTurns:     totalTurns,
		Players: []PlayerAnalysis{{
			CommanderName: commander,
			Won:           won,
			CardsPlayed:   cards,
		}},
	}
}

// TestBuildDeckFingerprints_CardFrequencyAggregation pins per-game
// dedupe + frequency math: 4 games where commander played; "Rhystic"
// cast in 3 of them, "Sol Ring" in all 4 → 0.75 and 1.00 frequency.
func TestBuildDeckFingerprints_CardFrequencyAggregation(t *testing.T) {
	games := []*GameAnalysis{
		mkFpGame("Cmdr A", true, "combo", 5, 7, 10,
			CardPerformance{Name: "Rhystic Study", TurnCast: 3},
			CardPerformance{Name: "Sol Ring", TurnCast: 1},
		),
		mkFpGame("Cmdr A", false, "", 0, 0, 8,
			CardPerformance{Name: "Rhystic Study", TurnCast: 4},
			CardPerformance{Name: "Sol Ring", TurnCast: 1},
		),
		mkFpGame("Cmdr A", false, "", 0, 0, 12,
			CardPerformance{Name: "Sol Ring", TurnCast: 1},
		),
		mkFpGame("Cmdr A", true, "combo", 0, 6, 9,
			CardPerformance{Name: "Rhystic Study", TurnCast: 5},
			CardPerformance{Name: "Sol Ring", TurnCast: 1},
		),
	}
	fps := BuildDeckFingerprints(games)
	fp := findFingerprint(fps, "Cmdr A")
	if fp == nil {
		t.Fatal("Cmdr A missing")
	}
	if fp.GamesObserved != 4 {
		t.Errorf("GamesObserved = %d, want 4", fp.GamesObserved)
	}
	if got := findCardFreq(fp, "Sol Ring"); got == nil || got.GamesCast != 4 || !fpApprox(got.Frequency, 1.0) {
		t.Errorf("Sol Ring = %+v, want games=4 freq=1.0", got)
	}
	if got := findCardFreq(fp, "Rhystic Study"); got == nil || got.GamesCast != 3 || !fpApprox(got.Frequency, 0.75) {
		t.Errorf("Rhystic Study = %+v, want games=3 freq=0.75", got)
	}
	// Sort: Sol Ring (freq 1.0) ranks above Rhystic Study (freq 0.75).
	if len(fp.CardFrequency) < 2 || fp.CardFrequency[0].Name != "Sol Ring" {
		t.Errorf("rank 1 = %q, want Sol Ring", fp.CardFrequency[0].Name)
	}
}

func findCardFreq(fp *DeckFingerprint, name string) *CardFrequencyEntry {
	for i := range fp.CardFrequency {
		if fp.CardFrequency[i].Name == name {
			return &fp.CardFrequency[i]
		}
	}
	return nil
}

// TestBuildDeckFingerprints_ExcludesLandsTokensCommander pins the
// three filters: lands, tokens, and the commander's own row never
// appear in CardFrequency.
func TestBuildDeckFingerprints_ExcludesLandsTokensCommander(t *testing.T) {
	games := []*GameAnalysis{
		mkFpGame("Cmdr A", true, "combat_damage", 3, 0, 8,
			CardPerformance{Name: "Forest", TurnCast: 1, IsLand: true},
			CardPerformance{Name: "Treasure Token", TurnCast: 2, IsToken: true},
			CardPerformance{Name: "Cmdr A", TurnCast: 4},
			CardPerformance{Name: "Real Spell", TurnCast: 3},
		),
	}
	fps := BuildDeckFingerprints(games)
	fp := findFingerprint(fps, "Cmdr A")
	if fp == nil {
		t.Fatal("missing")
	}
	for _, banned := range []string{"Forest", "Treasure Token", "Cmdr A"} {
		if findCardFreq(fp, banned) != nil {
			t.Errorf("%q should be excluded from CardFrequency", banned)
		}
	}
	if findCardFreq(fp, "Real Spell") == nil {
		t.Errorf("Real Spell should appear")
	}
}

// TestBuildDeckFingerprints_AvgTimingBenchmarks pins the conditional
// means for AvgTurnToFirstResolution / AvgTurnToFirstCombo /
// AvgFirstBlood — only games where the event fired contribute.
func TestBuildDeckFingerprints_AvgTimingBenchmarks(t *testing.T) {
	games := []*GameAnalysis{
		mkFpGame("Cmdr A", true, "combo", 4, 6, 7,
			CardPerformance{Name: "Cmdr A", TurnCast: 3},
		),
		// Commander never resolved + no combo: doesn't lower the avgs.
		mkFpGame("Cmdr A", false, "", 0, 0, 12),
		mkFpGame("Cmdr A", true, "combo", 5, 8, 9,
			CardPerformance{Name: "Cmdr A", TurnCast: 4},
		),
	}
	fps := BuildDeckFingerprints(games)
	fp := findFingerprint(fps, "Cmdr A")
	if fp == nil {
		t.Fatal("missing")
	}
	// First resolution: turns 3 and 4 → avg 3.5.
	if !fpApprox(fp.AvgTurnToFirstResolution, 3.5) {
		t.Errorf("AvgTurnToFirstResolution = %f, want 3.5", fp.AvgTurnToFirstResolution)
	}
	// First combo: turns 6 and 8 → avg 7.0.
	if !fpApprox(fp.AvgTurnToFirstCombo, 7.0) {
		t.Errorf("AvgTurnToFirstCombo = %f, want 7.0", fp.AvgTurnToFirstCombo)
	}
	// First blood: turns 4 and 5 → avg 4.5.
	if !fpApprox(fp.AvgFirstBlood, 4.5) {
		t.Errorf("AvgFirstBlood = %f, want 4.5", fp.AvgFirstBlood)
	}
	// Avg game length: (7 + 12 + 9) / 3 = 28/3.
	if !fpApprox(fp.AvgGameLength, 28.0/3.0) {
		t.Errorf("AvgGameLength = %f, want %f", fp.AvgGameLength, 28.0/3.0)
	}
}

// TestBuildDeckFingerprints_WinConditionMix pins the per-commander
// fraction of wins by WinCondition. Across 4 wins: 2 combo, 1
// combat_damage, 1 commander_damage → 0.5 / 0.25 / 0.25.
func TestBuildDeckFingerprints_WinConditionMix(t *testing.T) {
	games := []*GameAnalysis{
		mkFpGame("Cmdr A", true, "combo", 0, 5, 6),
		mkFpGame("Cmdr A", true, "combo", 0, 7, 8),
		mkFpGame("Cmdr A", true, "combat_damage", 4, 0, 10),
		mkFpGame("Cmdr A", true, "commander_damage", 3, 0, 9),
		mkFpGame("Cmdr A", false, "", 0, 0, 12),
	}
	fps := BuildDeckFingerprints(games)
	fp := findFingerprint(fps, "Cmdr A")
	if fp == nil {
		t.Fatal("missing")
	}
	if !fpApprox(fp.WinConditionMix["combo"], 0.5) {
		t.Errorf("combo = %f, want 0.5", fp.WinConditionMix["combo"])
	}
	if !fpApprox(fp.WinConditionMix["combat_damage"], 0.25) {
		t.Errorf("combat_damage = %f, want 0.25", fp.WinConditionMix["combat_damage"])
	}
	if !fpApprox(fp.WinConditionMix["commander_damage"], 0.25) {
		t.Errorf("commander_damage = %f, want 0.25", fp.WinConditionMix["commander_damage"])
	}
}

// TestBuildDeckFingerprints_EmptyAndNilGuards pins the nil/empty
// guards.
func TestBuildDeckFingerprints_EmptyAndNilGuards(t *testing.T) {
	if got := BuildDeckFingerprints(nil); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
	if got := BuildDeckFingerprints([]*GameAnalysis{}); got != nil {
		t.Errorf("empty slice: got %v, want nil", got)
	}
	// nil game in slice should be skipped, not crash.
	got := BuildDeckFingerprints([]*GameAnalysis{nil, mkFpGame("Cmdr", true, "combo", 0, 5, 6)})
	if len(got) != 1 || got[0].CommanderName != "Cmdr" {
		t.Errorf("nil entry should be skipped; got %v", got)
	}
}

// TestClassifyArchetype_StaxSignatureHits pins the Stax detection:
// 3+ signature cards at ≥20% frequency lands "stax".
func TestClassifyArchetype_StaxSignatureHits(t *testing.T) {
	staxGames := []*GameAnalysis{}
	for i := 0; i < 5; i++ {
		staxGames = append(staxGames, mkFpGame("Stax Lock", true, "turn_cap", 0, 0, 60,
			CardPerformance{Name: "Smothering Tithe", TurnCast: 3},
			CardPerformance{Name: "Rhystic Study", TurnCast: 4},
			CardPerformance{Name: "Winter Orb", TurnCast: 5},
			CardPerformance{Name: "Trinisphere", TurnCast: 6},
		))
	}
	fps := BuildDeckFingerprints(staxGames)
	fp := findFingerprint(fps, "Stax Lock")
	if fp == nil {
		t.Fatal("missing")
	}
	if fp.Archetype != "stax" {
		t.Errorf("Archetype = %q, want stax", fp.Archetype)
	}
	if fp.ArchetypeConfidence < 0.3 {
		t.Errorf("ArchetypeConfidence = %f, want >= 0.3", fp.ArchetypeConfidence)
	}
}

// TestClassifyArchetype_ComboSignatureAndWinMix pins combo detection
// via a mix of signature cards + win mix tilt.
func TestClassifyArchetype_ComboSignatureAndWinMix(t *testing.T) {
	games := []*GameAnalysis{}
	for i := 0; i < 5; i++ {
		games = append(games, mkFpGame("Combo Pile", true, "combo", 0, 5, 7,
			CardPerformance{Name: "Thassa's Oracle", TurnCast: 4},
			CardPerformance{Name: "Demonic Consultation", TurnCast: 4},
			CardPerformance{Name: "Tainted Pact", TurnCast: 5},
		))
	}
	fps := BuildDeckFingerprints(games)
	fp := findFingerprint(fps, "Combo Pile")
	if fp == nil {
		t.Fatal("missing")
	}
	if fp.Archetype != "combo" {
		t.Errorf("Archetype = %q, want combo", fp.Archetype)
	}
}

// TestClassifyArchetype_AggroFirstBlood pins the aggro signal:
// combat_damage wins with low first-blood turn (≤6) flag aggro, not
// midrange.
func TestClassifyArchetype_AggroFirstBlood(t *testing.T) {
	games := []*GameAnalysis{}
	for i := 0; i < 5; i++ {
		games = append(games, mkFpGame("Aggro Goblins", true, "combat_damage", 3, 0, 6,
			CardPerformance{Name: "Lightning Bolt", TurnCast: 1},
			CardPerformance{Name: "Goblin Guide", TurnCast: 1},
			CardPerformance{Name: "Embercleave", TurnCast: 4},
		))
	}
	fps := BuildDeckFingerprints(games)
	fp := findFingerprint(fps, "Aggro Goblins")
	if fp == nil {
		t.Fatal("missing")
	}
	if fp.Archetype != "aggro" {
		t.Errorf("Archetype = %q, want aggro (early first blood + combat_damage wins + signature)", fp.Archetype)
	}
}

// TestClassifyArchetype_UnknownWhenSampleSmall pins the small-sample
// guard: <3 games observed → unknown regardless of signal strength.
func TestClassifyArchetype_UnknownWhenSampleSmall(t *testing.T) {
	fp := &DeckFingerprint{
		CommanderName: "Tiny",
		GamesObserved: 2,
		CardFrequency: []CardFrequencyEntry{
			{Name: "Smothering Tithe", Frequency: 1.0},
			{Name: "Rhystic Study", Frequency: 1.0},
			{Name: "Winter Orb", Frequency: 1.0},
		},
	}
	arch, conf := ClassifyArchetype(fp)
	if arch != "unknown" || conf != 0 {
		t.Errorf("got (%s, %f), want (unknown, 0)", arch, conf)
	}
}

// TestClassifyArchetype_UnknownWhenNoSignatureMatch pins the
// no-signal guard: signature hits <2 AND winTilt for the leader <0.5
// → unknown.
func TestClassifyArchetype_UnknownWhenNoSignatureMatch(t *testing.T) {
	games := []*GameAnalysis{}
	for i := 0; i < 5; i++ {
		games = append(games, mkFpGame("Mystery", false, "", 0, 0, 10,
			CardPerformance{Name: "Obscure Card", TurnCast: 3},
		))
	}
	fps := BuildDeckFingerprints(games)
	fp := findFingerprint(fps, "Mystery")
	if fp == nil {
		t.Fatal("missing")
	}
	if fp.Archetype != "unknown" {
		t.Errorf("Archetype = %q, want unknown", fp.Archetype)
	}
}

// TestLookupFingerprint_CaseInsensitive pins the case-insensitive
// commander-name lookup. Hat 3rd Eye may pass the seat's exact
// CommanderName which can differ in casing across surfaces.
func TestLookupFingerprint_CaseInsensitive(t *testing.T) {
	fps := []DeckFingerprint{
		{CommanderName: "Atraxa, Praetors' Voice"},
		{CommanderName: "Korvold, Fae-Cursed King"},
	}
	if got := LookupFingerprint("atraxa, praetors' voice", fps); got == nil || got.CommanderName != "Atraxa, Praetors' Voice" {
		t.Errorf("lowercase lookup failed: got %v", got)
	}
	if got := LookupFingerprint("  Korvold, Fae-Cursed King  ", fps); got == nil {
		t.Errorf("whitespace-padded lookup failed")
	}
	if got := LookupFingerprint("Unknown Cmdr", fps); got != nil {
		t.Errorf("unknown name should return nil; got %+v", got)
	}
	if got := LookupFingerprint("", fps); got != nil {
		t.Errorf("empty name should return nil")
	}
}

// TestBuildDeckFingerprints_SortByGamesObserved pins the output sort:
// fingerprints emit by GamesObserved desc, then by CommanderName asc.
func TestBuildDeckFingerprints_SortByGamesObserved(t *testing.T) {
	games := []*GameAnalysis{}
	for i := 0; i < 10; i++ {
		games = append(games, mkFpGame("Popular", false, "", 0, 0, 5))
	}
	for i := 0; i < 5; i++ {
		games = append(games, mkFpGame("Mid", false, "", 0, 0, 5))
	}
	for i := 0; i < 3; i++ {
		games = append(games, mkFpGame("Niche", false, "", 0, 0, 5))
	}
	fps := BuildDeckFingerprints(games)
	if len(fps) < 3 {
		t.Fatalf("expected 3 fingerprints; got %d", len(fps))
	}
	wantOrder := []string{"Popular", "Mid", "Niche"}
	for i, want := range wantOrder {
		if fps[i].CommanderName != want {
			t.Errorf("rank %d = %q, want %q", i+1, fps[i].CommanderName, want)
		}
	}
}

// TestWriteDeckFingerprints_RenderShape pins the markdown surface:
// per-commander heading, archetype summary, win-mix line, card
// frequency table.
func TestWriteDeckFingerprints_RenderShape(t *testing.T) {
	games := []*GameAnalysis{}
	for i := 0; i < 5; i++ {
		games = append(games, mkFpGame("Stax Lock", true, "turn_cap", 0, 0, 60,
			CardPerformance{Name: "Smothering Tithe", TurnCast: 3},
			CardPerformance{Name: "Rhystic Study", TurnCast: 4},
			CardPerformance{Name: "Winter Orb", TurnCast: 5},
			CardPerformance{Name: "Trinisphere", TurnCast: 6},
		))
	}
	fps := BuildDeckFingerprints(games)
	r := &AnalyticsReport{
		Analyses:         games,
		DeckFingerprints: fps,
		TotalGames:       5,
	}
	var b strings.Builder
	r.writeDeckFingerprints(&b, 10)
	out := b.String()
	if !strings.Contains(out, "## Deck Fingerprints") {
		t.Errorf("missing section header; got:\n%s", out)
	}
	if !strings.Contains(out, "### Stax Lock") {
		t.Errorf("missing commander heading; got:\n%s", out)
	}
	if !strings.Contains(out, "**stax**") {
		t.Errorf("expected archetype label '**stax**' in summary; got:\n%s", out)
	}
	if !strings.Contains(out, "Win mix:") {
		t.Errorf("missing win-mix line; got:\n%s", out)
	}
	if !strings.Contains(out, "turn_cap 100%") {
		t.Errorf("expected turn_cap 100%% in win mix; got:\n%s", out)
	}
	if !strings.Contains(out, "| Card | Frequency | Games Cast |") {
		t.Errorf("missing card-frequency table header; got:\n%s", out)
	}
	if !strings.Contains(out, "Smothering Tithe") {
		t.Errorf("Smothering Tithe should appear in card freq; got:\n%s", out)
	}
}

// TestWriteDeckFingerprints_NoData pins the nil-data guard.
func TestWriteDeckFingerprints_NoData(t *testing.T) {
	var b strings.Builder
	(&AnalyticsReport{}).writeDeckFingerprints(&b, 10)
	out := b.String()
	if !strings.Contains(out, "## Deck Fingerprints") {
		t.Errorf("missing header; got:\n%s", out)
	}
	if !strings.Contains(out, "No fingerprint data") {
		t.Errorf("expected nil-data message; got:\n%s", out)
	}
}

// TestWriteDeckFingerprints_TruncatesAtCap pins the top-N render cap
// with `+N more commanders` footnote.
func TestWriteDeckFingerprints_TruncatesAtCap(t *testing.T) {
	fps := make([]DeckFingerprint, 0, 12)
	for i := 0; i < 12; i++ {
		fps = append(fps, DeckFingerprint{
			CommanderName: string(rune('A'+i)) + " Commander",
			GamesObserved: 5 - i%3,
		})
	}
	r := &AnalyticsReport{DeckFingerprints: fps}
	var b strings.Builder
	r.writeDeckFingerprints(&b, 5)
	out := b.String()
	if !strings.Contains(out, "+7 more commanders") {
		t.Errorf("missing tail footnote; got:\n%s", out)
	}
}
