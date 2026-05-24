package heimdall

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func newSummaryGS(t *testing.T, seats int) *gameengine.GameState {
	t.Helper()
	gs := gameengine.NewGameState(seats, rand.New(rand.NewSource(1)), nil)
	gs.Turn = 12
	gs.CardFirstPlayed = map[string]int{}
	return gs
}

// BuildGameSummary copies obs's three R60 signals through verbatim
// and stamps data_source = "rich".
func TestBuildGameSummary_RichDataSourceWithAllSignals(t *testing.T) {
	gs := newSummaryGS(t, 4)
	obs := Observation{
		Seed: GameSeed{
			RNGSeed:  4242,
			Winner:   1,
			Turns:    12,
			DeckKeys: [4]string{"a", "b", "c", "d"},
		},
		CommanderZoneVisits: []CommanderZoneVisit{
			{Seat: 1, CommanderName: "Edgar Markov", CastCount: 2, InZoneAtEnd: true, Visits: 3},
		},
		RegretCards: []RegretCard{
			{Seat: 0, CardName: "Blightsteel Colossus", CMC: 12, Reason: "stranded_in_hand"},
		},
		MVPCards: []MVPCard{
			{Seat: 1, CardName: "Edgar Markov", CMC: 6, Score: 10, IsCommander: true},
		},
	}

	s := BuildGameSummary(obs, gs, "combat")

	if s.GameID != "4242" {
		t.Errorf("GameID: want 4242, got %s", s.GameID)
	}
	if s.Winner != 1 || s.Turns != 12 || s.EndReason != "combat" {
		t.Errorf("metadata mismatch: winner=%d turns=%d end=%q", s.Winner, s.Turns, s.EndReason)
	}
	if s.DataSource != "rich" {
		t.Errorf("DataSource: want rich, got %q", s.DataSource)
	}
	if len(s.CommanderZoneVisits) != 1 || len(s.RegretCards) != 1 || len(s.MVPCards) != 1 {
		t.Errorf("signal arrays not pass-through: %+v", s)
	}
	if got := s.DeckKeys; len(got) != 4 || got[0] != "a" || got[3] != "d" {
		t.Errorf("DeckKeys flatten failed: %v", got)
	}
}

// Turning points: a commander cast in turn 4 + the game-end record
// for the winner on turn 12, sorted chronologically.
func TestExtractTurningPoints_CommanderFirstCastAndGameEnd(t *testing.T) {
	gs := newSummaryGS(t, 4)
	gs.Seats[1].CommanderNames = []string{"Edgar Markov"}
	gs.CardFirstPlayed["Edgar Markov"] = 4
	obs := Observation{Seed: GameSeed{Winner: 1, Turns: 12}}

	tps := ExtractTurningPoints(obs, gs)
	if len(tps) != 2 {
		t.Fatalf("want 2 turning points, got %d", len(tps))
	}
	// First record is the commander cast (turn 4).
	if tps[0].Turn != 4 || tps[0].Kind != "commander_first_cast" || tps[0].Seat != 1 {
		t.Errorf("position 0 should be commander_first_cast on turn 4 seat 1; got %+v", tps[0])
	}
	if !strings.Contains(tps[0].Description, "Edgar Markov") {
		t.Errorf("position 0 description should name the commander; got %q", tps[0].Description)
	}
	// Second record is game_end on turn 12.
	if tps[1].Turn != 12 || tps[1].Kind != "game_end" || tps[1].Seat != 1 {
		t.Errorf("position 1 should be game_end on turn 12 seat 1; got %+v", tps[1])
	}
}

// Partner pair: each cast commander surfaces independently.
func TestExtractTurningPoints_PartnerPairBothSurface(t *testing.T) {
	gs := newSummaryGS(t, 2)
	gs.Turn = 8
	gs.Seats[0].CommanderNames = []string{"Tymna the Weaver", "Thrasios, Triton Hero"}
	gs.CardFirstPlayed["Tymna the Weaver"] = 3
	gs.CardFirstPlayed["Thrasios, Triton Hero"] = 4
	obs := Observation{Seed: GameSeed{Winner: 0, Turns: 8}}

	tps := ExtractTurningPoints(obs, gs)
	if len(tps) != 3 {
		t.Fatalf("want 3 turning points (2 commander casts + game_end), got %d", len(tps))
	}
	want := []struct {
		turn int
		kind string
	}{
		{3, "commander_first_cast"},
		{4, "commander_first_cast"},
		{8, "game_end"},
	}
	for i, w := range want {
		if tps[i].Turn != w.turn || tps[i].Kind != w.kind {
			t.Errorf("position %d: want turn=%d kind=%s; got %+v", i, w.turn, w.kind, tps[i])
		}
	}
}

// A commander that never resolved (no CardFirstPlayed entry) does
// not produce a turning point.
func TestExtractTurningPoints_UncastCommanderSkipped(t *testing.T) {
	gs := newSummaryGS(t, 2)
	gs.Seats[0].CommanderNames = []string{"Yuriko, the Tiger's Shadow"}
	// No CardFirstPlayed entry — never cast.
	obs := Observation{Seed: GameSeed{Winner: 0, Turns: 8}}

	tps := ExtractTurningPoints(obs, gs)
	if len(tps) != 1 || tps[0].Kind != "game_end" {
		t.Errorf("uncast commander should only produce game_end; got %+v", tps)
	}
}

// Draw / no-winner game: game_end still emits with seat=-1 and a
// description that names the no-winner condition.
func TestExtractTurningPoints_DrawEmitsGameEndWithMinusOne(t *testing.T) {
	gs := newSummaryGS(t, 2)
	obs := Observation{Seed: GameSeed{Winner: -1, Turns: 80}}

	tps := ExtractTurningPoints(obs, gs)
	if len(tps) != 1 {
		t.Fatalf("want exactly 1 turning point (game_end), got %d", len(tps))
	}
	if tps[0].Seat != -1 || !strings.Contains(tps[0].Description, "without a winner") {
		t.Errorf("draw description should flag no-winner; got %+v", tps[0])
	}
}

// gs nil: BuildGameSummary tolerates a nil GameState; turning points
// degrade to game_end only, and the signal arrays still pass through.
func TestBuildGameSummary_NilGameStateDegradesGracefully(t *testing.T) {
	obs := Observation{
		Seed: GameSeed{RNGSeed: 9, Winner: 2, Turns: 5},
		MVPCards: []MVPCard{
			{Seat: 2, CardName: "Sol Ring", CMC: 1, Score: 1},
		},
	}

	s := BuildGameSummary(obs, nil, "")

	if s.GameID != "9" || s.Winner != 2 || s.Turns != 5 {
		t.Errorf("metadata pass-through failed: %+v", s)
	}
	if len(s.MVPCards) != 1 {
		t.Errorf("MVPCards should pass through even with nil gs; got %+v", s.MVPCards)
	}
	if len(s.TurningPoints) != 1 || s.TurningPoints[0].Kind != "game_end" {
		t.Errorf("nil gs should still emit game_end turning point; got %+v", s.TurningPoints)
	}
}

// Empty observation: builds a minimal summary with the game_end
// turning point and empty signal arrays.
func TestBuildGameSummary_EmptyObservation(t *testing.T) {
	gs := newSummaryGS(t, 2)
	gs.Turn = 1
	obs := Observation{Seed: GameSeed{Winner: 0, Turns: 1}}

	s := BuildGameSummary(obs, gs, "timeout")

	if s.EndReason != "timeout" || s.Turns != 1 {
		t.Errorf("metadata mismatch: %+v", s)
	}
	if len(s.CommanderZoneVisits) != 0 || len(s.RegretCards) != 0 || len(s.MVPCards) != 0 {
		t.Errorf("empty observation should keep signal arrays empty; got %+v", s)
	}
	if len(s.TurningPoints) != 1 {
		t.Errorf("empty observation should still emit game_end; got %+v", s.TurningPoints)
	}
}

// Sort stability: when two turning points share a turn, sort falls
// back to seat ascending then kind ascending.
func TestExtractTurningPoints_TieBreakSeatThenKind(t *testing.T) {
	gs := newSummaryGS(t, 4)
	gs.Seats[0].CommanderNames = []string{"Krenko, Mob Boss"}
	gs.Seats[1].CommanderNames = []string{"Atraxa, Praetors' Voice"}
	gs.CardFirstPlayed["Krenko, Mob Boss"] = 4
	gs.CardFirstPlayed["Atraxa, Praetors' Voice"] = 4
	obs := Observation{Seed: GameSeed{Winner: 0, Turns: 10}}

	tps := ExtractTurningPoints(obs, gs)
	if len(tps) != 3 {
		t.Fatalf("want 3 turning points, got %d", len(tps))
	}
	// Both commander casts on turn 4 sort by seat ascending.
	if tps[0].Seat != 0 || tps[1].Seat != 1 {
		t.Errorf("turn-4 ties should sort by seat asc; got seats [%d,%d]", tps[0].Seat, tps[1].Seat)
	}
}
