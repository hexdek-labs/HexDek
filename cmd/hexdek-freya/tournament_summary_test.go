package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/tournament"
)

// fixtureSyntheticTournament builds an end-to-end TournamentResult
// covering every section the summary report computes:
//
//   - 6 commanders with varied winrate spreads (top/mid/bottom tiers)
//   - ELO entries reflecting performance (so upsets register against
//     ELO baseline)
//   - One genuine upset: Krenko (low ELO) beat Atraxa Praetors (high
//     ELO) 4-1 head-to-head
//   - SwissStandings with a clear OMW% leader
//   - KillRecords spread across commanders + methods so the win-
//     condition analyzer has data to roll up
//   - CardRankings with a top-contribution leader
//
// This is the canonical fixture used by every summary-report test;
// each test asserts one specific section is wired right.
func fixtureSyntheticTournament() *tournament.TournamentResult {
	commanders := []string{
		"Atraxa, Praetors' Voice", // top tier — high winrate
		"Thrasios, Triton Hero",   // top tier
		"Yawgmoth, Thran Physician",
		"Uril, the Miststalker",
		"Meren of Clan Nel Toth",
		"Krenko, Mob Boss", // bottom tier overall, but pulls off one upset
	}
	wins := map[string]int{
		"Atraxa, Praetors' Voice":   18,
		"Thrasios, Triton Hero":     16,
		"Yawgmoth, Thran Physician": 12,
		"Uril, the Miststalker":     10,
		"Meren of Clan Nel Toth":    8,
		"Krenko, Mob Boss":          4,
	}
	gamesByCommander := map[string]int{
		"Atraxa, Praetors' Voice":   30,
		"Thrasios, Triton Hero":     30,
		"Yawgmoth, Thran Physician": 30,
		"Uril, the Miststalker":     30,
		"Meren of Clan Nel Toth":    30,
		"Krenko, Mob Boss":          30,
	}

	elo := []tournament.ELOEntry{
		{Commander: "Atraxa, Praetors' Voice", Rating: 1750, Games: 30},
		{Commander: "Thrasios, Triton Hero", Rating: 1720, Games: 30},
		{Commander: "Yawgmoth, Thran Physician", Rating: 1600, Games: 30},
		{Commander: "Uril, the Miststalker", Rating: 1500, Games: 30},
		{Commander: "Meren of Clan Nel Toth", Rating: 1450, Games: 30},
		{Commander: "Krenko, Mob Boss", Rating: 1380, Games: 30},
	}

	// MatchupMatrix: Krenko beats Atraxa 4-1 head-to-head despite the
	// 370-ELO gap → that's the upset the test will assert.
	matchupMatrix := map[string]map[string]int{
		"Krenko, Mob Boss": {
			"Atraxa, Praetors' Voice": 4, // Krenko wins
		},
		"Atraxa, Praetors' Voice": {
			"Krenko, Mob Boss": 1, // Atraxa loses
		},
		// Other matchups roughly track ELO — no upsets there.
		"Thrasios, Triton Hero": {
			"Meren of Clan Nel Toth": 4,
		},
		"Meren of Clan Nel Toth": {
			"Thrasios, Triton Hero": 1,
		},
	}
	matchupGames := map[string]map[string]int{
		"Krenko, Mob Boss": {
			"Atraxa, Praetors' Voice": 5,
		},
		"Atraxa, Praetors' Voice": {
			"Krenko, Mob Boss": 5,
		},
		"Thrasios, Triton Hero": {
			"Meren of Clan Nel Toth": 5,
		},
		"Meren of Clan Nel Toth": {
			"Thrasios, Triton Hero": 5,
		},
	}

	swiss := []tournament.SwissStanding{
		{CommanderName: "Atraxa, Praetors' Voice", Points: 18, Games: 9, Wins: 6, Losses: 3, WinRate: 0.667, OpponentMatchWinPct: 0.580},
		{CommanderName: "Thrasios, Triton Hero", Points: 15, Games: 9, Wins: 5, Losses: 4, WinRate: 0.556, OpponentMatchWinPct: 0.620}, // OMW leader
		{CommanderName: "Yawgmoth, Thran Physician", Points: 12, Games: 9, Wins: 4, Losses: 5, WinRate: 0.444, OpponentMatchWinPct: 0.510},
		{CommanderName: "Uril, the Miststalker", Points: 9, Games: 9, Wins: 3, Losses: 6, WinRate: 0.333, OpponentMatchWinPct: 0.490},
		{CommanderName: "Meren of Clan Nel Toth", Points: 9, Games: 9, Wins: 3, Losses: 6, WinRate: 0.333, OpponentMatchWinPct: 0.470},
		{CommanderName: "Krenko, Mob Boss", Points: 3, Games: 9, Wins: 1, Losses: 8, WinRate: 0.111, OpponentMatchWinPct: 0.450},
	}

	killRecords := []analytics.KillRecord{
		// Atraxa wins via planeswalker ultimates 5 times.
		{KillerCommander: "Atraxa, Praetors' Voice", VictimCommander: "Uril, the Miststalker", Method: "planeswalker_ultimate", LethalCard: "Elspeth, Sun's Champion", Turn: 10},
		{KillerCommander: "Atraxa, Praetors' Voice", VictimCommander: "Uril, the Miststalker", Method: "planeswalker_ultimate", LethalCard: "Elspeth, Sun's Champion", Turn: 11},
		{KillerCommander: "Atraxa, Praetors' Voice", VictimCommander: "Meren of Clan Nel Toth", Method: "planeswalker_ultimate", LethalCard: "Karn Liberated", Turn: 12},
		{KillerCommander: "Atraxa, Praetors' Voice", VictimCommander: "Krenko, Mob Boss", Method: "combat_damage", LethalCard: "", Turn: 9},
		{KillerCommander: "Atraxa, Praetors' Voice", VictimCommander: "Yawgmoth, Thran Physician", Method: "planeswalker_ultimate", LethalCard: "Elspeth, Sun's Champion", Turn: 13},
		// Thrasios wins via Thoracle.
		{KillerCommander: "Thrasios, Triton Hero", VictimCommander: "Meren of Clan Nel Toth", Method: "alt_wincon", LethalCard: "Thassa's Oracle", Turn: 7},
		{KillerCommander: "Thrasios, Triton Hero", VictimCommander: "Yawgmoth, Thran Physician", Method: "alt_wincon", LethalCard: "Thassa's Oracle", Turn: 6},
		{KillerCommander: "Thrasios, Triton Hero", VictimCommander: "Atraxa, Praetors' Voice", Method: "alt_wincon", LethalCard: "Thassa's Oracle", Turn: 8},
		// Yawgmoth wins via drain.
		{KillerCommander: "Yawgmoth, Thran Physician", VictimCommander: "Krenko, Mob Boss", Method: "life_drain", LethalCard: "Blood Artist", Turn: 11},
		{KillerCommander: "Yawgmoth, Thran Physician", VictimCommander: "Uril, the Miststalker", Method: "life_drain", LethalCard: "Blood Artist", Turn: 12},
		// Krenko's one upset: combat damage vs Atraxa.
		{KillerCommander: "Krenko, Mob Boss", VictimCommander: "Atraxa, Praetors' Voice", Method: "combat_damage", LethalCard: "Goblin Piledriver", Turn: 6},
	}

	cardRankings := []analytics.CardRanking{
		{Name: "Thassa's Oracle", GamesPlayed: 30, GamesWon: 16, WinContribution: 0.53, KillShotRate: 0.40},
		{Name: "Doubling Season", GamesPlayed: 30, GamesWon: 18, WinContribution: 0.51, KillShotRate: 0.10},
		{Name: "Elspeth, Sun's Champion", GamesPlayed: 30, GamesWon: 12, WinContribution: 0.40, KillShotRate: 0.20},
		{Name: "Sol Ring", GamesPlayed: 180, GamesWon: 64, WinContribution: 0.35, KillShotRate: 0.0},
		{Name: "Blood Artist", GamesPlayed: 30, GamesWon: 10, WinContribution: 0.30, KillShotRate: 0.15},
		{Name: "Goblin Piledriver", GamesPlayed: 30, GamesWon: 4, WinContribution: 0.10, KillShotRate: 0.25},
	}

	return &tournament.TournamentResult{
		SchemaVersion:          tournament.SchemaVersion,
		Mode:                   "swiss",
		Games:                  60,
		NSeats:                 4,
		AvgTurns:               9.2,
		CommanderNames:         commanders,
		WinsByCommander:        wins,
		GamesPlayedByCommander: gamesByCommander,
		ELO:                    elo,
		MatchupMatrix:          matchupMatrix,
		MatchupGames:           matchupGames,
		SwissStandings:         swiss,
		KillRecords:            killRecords,
		CardRankings:           cardRankings,
		Duration:               time.Hour,
	}
}

// TestBuildTournamentSummary_PerformanceTiers verifies decks split
// into top/mid/bottom by winrate quartile, with the top tier
// containing the highest-winrate commanders.
func TestBuildTournamentSummary_PerformanceTiers(t *testing.T) {
	r := fixtureSyntheticTournament()
	s := BuildTournamentSummary(r)

	if len(s.Tiers) != 3 {
		t.Fatalf("expected 3 tiers (top/mid/bottom), got %d", len(s.Tiers))
	}
	if s.Tiers[0].Tier != "top" {
		t.Errorf("first tier label = %q, want top", s.Tiers[0].Tier)
	}
	if len(s.Tiers[0].Commanders) == 0 {
		t.Fatal("top tier is empty")
	}
	top := s.Tiers[0].Commanders[0]
	if top.Commander != "Atraxa, Praetors' Voice" {
		t.Errorf("top of leaderboard = %q, want Atraxa (18/30 wins)", top.Commander)
	}
	if top.Winrate < 0.59 {
		t.Errorf("top winrate = %.3f, want >0.59 (18/30 = 0.6)", top.Winrate)
	}
}

// TestBuildTournamentSummary_ArchetypeBreakdown verifies the
// commander → archetype heuristic produces a sensible meta
// distribution + names a best archetype.
func TestBuildTournamentSummary_ArchetypeBreakdown(t *testing.T) {
	r := fixtureSyntheticTournament()
	s := BuildTournamentSummary(r)

	if len(s.ArchetypeBreakdown) == 0 {
		t.Fatal("ArchetypeBreakdown empty")
	}
	// Krenko + Edgar Markov would both map to Aggro in the hint
	// table; the fixture only has Krenko so Aggro has 1 commander.
	// Atraxa Praetors' Voice = Superfriends; Thrasios = Combo; etc.
	if s.ArchetypeBreakdown["Superfriends"] != 1 {
		t.Errorf("Superfriends count = %d, want 1 (Atraxa Praetors)", s.ArchetypeBreakdown["Superfriends"])
	}
	if _, ok := s.ArchetypeWinrate["Superfriends"]; !ok {
		t.Errorf("ArchetypeWinrate missing Superfriends")
	}
}

// TestBuildTournamentSummary_UpsetDetection verifies the H2H upset
// scan picks up the seeded Krenko-over-Atraxa case (370 ELO gap,
// 4-1 H2H series in Krenko's favor).
func TestBuildTournamentSummary_UpsetDetection(t *testing.T) {
	r := fixtureSyntheticTournament()
	s := BuildTournamentSummary(r)

	if len(s.Upsets) == 0 {
		t.Fatal("no upsets detected — Krenko 4-1 over Atraxa should fire")
	}
	found := false
	for _, u := range s.Upsets {
		if u.Winner == "Krenko, Mob Boss" && u.Loser == "Atraxa, Praetors' Voice" {
			found = true
			if u.ELOGap < 300 {
				t.Errorf("ELO gap = %.0f, want >=300 (1750-1380=370)", u.ELOGap)
			}
			if u.WinnerWins != 4 || u.Games != 5 {
				t.Errorf("H2H = %d/%d, want 4/5", u.WinnerWins, u.Games)
			}
		}
	}
	if !found {
		t.Errorf("Krenko→Atraxa upset missing from %v", s.Upsets)
	}
}

// TestBuildTournamentSummary_OMWLeaders verifies Thrasios (0.620 OMW)
// leads despite Atraxa (0.580) having more points. OMW% is the
// strength-of-schedule signal — independent of raw winrate.
func TestBuildTournamentSummary_OMWLeaders(t *testing.T) {
	r := fixtureSyntheticTournament()
	s := BuildTournamentSummary(r)

	if len(s.OMWLeaders) == 0 {
		t.Fatal("OMWLeaders empty — fixture has SwissStandings populated")
	}
	if s.OMWLeaders[0].Commander != "Thrasios, Triton Hero" {
		t.Errorf("OMW leader = %q, want Thrasios (0.620)", s.OMWLeaders[0].Commander)
	}
	if s.OMWLeaders[0].OMW < 0.61 {
		t.Errorf("OMW = %f, want 0.620", s.OMWLeaders[0].OMW)
	}
}

// TestBuildTournamentSummary_WinConditions verifies KillRecords
// roll up to per-commander dominant method + lethal card.
// Atraxa wins 4/5 via planeswalker ultimate, Elspeth being the
// most common lethal card (3 of 4 PW-ultimate wins).
func TestBuildTournamentSummary_WinConditions(t *testing.T) {
	r := fixtureSyntheticTournament()
	s := BuildTournamentSummary(r)

	if len(s.WinConditions) == 0 {
		t.Fatal("WinConditions empty")
	}
	byCommander := map[string]CommanderWinPattern{}
	for _, w := range s.WinConditions {
		byCommander[w.Commander] = w
	}

	atraxa, ok := byCommander["Atraxa, Praetors' Voice"]
	if !ok {
		t.Fatal("Atraxa missing from win conditions")
	}
	if atraxa.TopMethod != "planeswalker_ultimate" {
		t.Errorf("Atraxa top method = %q, want planeswalker_ultimate", atraxa.TopMethod)
	}
	if atraxa.TopLethalCard != "Elspeth, Sun's Champion" {
		t.Errorf("Atraxa top lethal card = %q, want Elspeth, Sun's Champion", atraxa.TopLethalCard)
	}
	if atraxa.TopMethodPct < 70 {
		t.Errorf("Atraxa top method pct = %d, want >=70 (4/5)", atraxa.TopMethodPct)
	}

	thrasios, ok := byCommander["Thrasios, Triton Hero"]
	if !ok {
		t.Fatal("Thrasios missing from win conditions")
	}
	if thrasios.TopLethalCard != "Thassa's Oracle" {
		t.Errorf("Thrasios top lethal card = %q, want Thassa's Oracle", thrasios.TopLethalCard)
	}
}

// TestBuildTournamentSummary_TopCards verifies the win-contribution
// leaderboard sorts correctly.
func TestBuildTournamentSummary_TopCards(t *testing.T) {
	r := fixtureSyntheticTournament()
	s := BuildTournamentSummary(r)

	if len(s.TopCards) == 0 {
		t.Fatal("TopCards empty")
	}
	if s.TopCards[0].Card != "Thassa's Oracle" {
		t.Errorf("top card = %q, want Thassa's Oracle (0.53 contribution)", s.TopCards[0].Card)
	}
	// Descending invariant.
	for i := 1; i < len(s.TopCards); i++ {
		if s.TopCards[i].WinContribution > s.TopCards[i-1].WinContribution {
			t.Errorf("TopCards not sorted desc at index %d", i)
		}
	}
}

// TestLoadTournamentResult_RoundTrip verifies the JSON loader
// produces a valid TournamentResult from a marshaled fixture, and
// that BuildTournamentSummary handles the loaded shape identically
// to the in-memory version.
func TestLoadTournamentResult_RoundTrip(t *testing.T) {
	r := fixtureSyntheticTournament()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "tournament.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := LoadTournamentResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	s := BuildTournamentSummary(loaded)
	if s == nil {
		t.Fatal("BuildTournamentSummary returned nil on loaded fixture")
	}
	if s.Games != r.Games {
		t.Errorf("Games = %d, want %d", s.Games, r.Games)
	}
}

// TestLoadTournamentResult_MissingFile verifies the loader returns a
// useful error on missing path rather than panicking.
func TestLoadTournamentResult_MissingFile(t *testing.T) {
	_, err := LoadTournamentResult("/nonexistent/path/tournament.json")
	if err == nil {
		t.Error("expected error on missing file")
	}
}

// TestFormatTournamentSummary_RendersAllSections is the end-to-end
// smoke test that every section header surfaces in the rendered
// output.
func TestFormatTournamentSummary_RendersAllSections(t *testing.T) {
	r := fixtureSyntheticTournament()
	s := BuildTournamentSummary(r)
	out := FormatTournamentSummary(s)

	required := []string{
		"FREYA -- Tournament Summary",
		"[TOP tier]",
		"[MID tier]",
		"[BOTTOM tier]",
		"[Archetype meta]",
		"[Surprise upsets]",
		"[OMW% leaders]",
		"[Win conditions]",
		"[Top cards]",
		"Atraxa, Praetors' Voice",
		"Krenko, Mob Boss",
		"Thassa's Oracle",
	}
	for _, r := range required {
		if !strings.Contains(out, r) {
			t.Errorf("missing required substring %q\n--- output ---\n%s", r, out)
		}
	}
}

// TestArchetypeForCommander_KnownAndUnknown pins the heuristic table:
// known commanders return their canonical archetype; unknown
// commanders fall through to "Unclassified" so the rollup degrades
// gracefully on novel commanders.
func TestArchetypeForCommander_KnownAndUnknown(t *testing.T) {
	cases := []struct {
		commander string
		want      string
	}{
		{"Atraxa, Praetors' Voice", "Superfriends"},
		{"Krenko, Mob Boss", "Aggro"},
		{"Yawgmoth, Thran Physician", "Aristocrats"},
		{"Uril, the Miststalker", "Voltron"},
		{"Some Unknown Commander", "Unclassified"},
		{"", "Unclassified"},
	}
	for _, c := range cases {
		if got := archetypeForCommander(c.commander); got != c.want {
			t.Errorf("archetypeForCommander(%q) = %q, want %q", c.commander, got, c.want)
		}
	}
}

// TestBuildTournamentSummary_NilSafe defends against panics on
// degenerate input.
func TestBuildTournamentSummary_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on nil input: %v", r)
		}
	}()
	if s := BuildTournamentSummary(nil); s != nil {
		t.Errorf("expected nil on nil input, got %+v", s)
	}
}

// TestBuildTournamentSummary_NoSwissNoOMW verifies the OMW% section
// stays empty when SwissStandings are absent (non-Swiss tournament
// mode).
func TestBuildTournamentSummary_NoSwissNoOMW(t *testing.T) {
	r := fixtureSyntheticTournament()
	r.SwissStandings = nil
	s := BuildTournamentSummary(r)
	if len(s.OMWLeaders) != 0 {
		t.Errorf("OMWLeaders should be empty for non-Swiss tournament; got %v", s.OMWLeaders)
	}
}

// TestBuildTournamentSummary_UpsetThresholdGuards verifies an H2H
// reversal below the ELO-gap or games-count threshold doesn't
// register as an upset (filters out noise).
func TestBuildTournamentSummary_UpsetThresholdGuards(t *testing.T) {
	r := fixtureSyntheticTournament()
	// Reduce the gap below 100 by raising Krenko's ELO.
	for i, e := range r.ELO {
		if e.Commander == "Krenko, Mob Boss" {
			r.ELO[i].Rating = 1700 // 50pt gap vs Atraxa now
		}
	}
	s := BuildTournamentSummary(r)
	for _, u := range s.Upsets {
		if u.Winner == "Krenko, Mob Boss" && u.Loser == "Atraxa, Praetors' Voice" {
			t.Errorf("upset should be filtered out at 50-point gap; got %+v", u)
		}
	}
}
