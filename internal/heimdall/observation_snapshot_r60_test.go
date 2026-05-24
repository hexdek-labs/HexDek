package heimdall

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func newSnapshotGS(t *testing.T, seats int) *gameengine.GameState {
	t.Helper()
	gs := gameengine.NewGameState(seats, rand.New(rand.NewSource(1)), nil)
	gs.Turn = 12
	gs.CardFirstPlayed = map[string]int{}
	return gs
}

// SnapshotFromObservation copies the three R60 signals from obs +
// extracts CardFirstPlayed + per-seat commander names from gs.
func TestSnapshotFromObservation_CopiesSignalsAndCommanderState(t *testing.T) {
	gs := newSnapshotGS(t, 4)
	gs.Seats[0].CommanderNames = []string{"Krenko, Mob Boss"}
	gs.Seats[2].CommanderNames = []string{"Tymna the Weaver", "Thrasios, Triton Hero"}
	gs.CardFirstPlayed["Krenko, Mob Boss"] = 3
	gs.CardFirstPlayed["Tymna the Weaver"] = 4

	obs := Observation{
		Seed: GameSeed{
			RNGSeed:  7,
			Winner:   2,
			Turns:    12,
			DeckKeys: [4]string{"a", "b", "c", "d"},
		},
		CommanderZoneVisits: []CommanderZoneVisit{
			{Seat: 0, CommanderName: "Krenko, Mob Boss", CastCount: 2, InZoneAtEnd: true, Visits: 3},
		},
		RegretCards: []RegretCard{
			{Seat: 1, CardName: "Cyclonic Rift", CMC: 7, Reason: "stranded_in_hand"},
		},
		MVPCards: []MVPCard{
			{Seat: 2, CardName: "Tymna the Weaver", CMC: 3, Score: 7, IsCommander: true},
		},
	}

	snap := SnapshotFromObservation(obs, gs)

	if snap.Seed.RNGSeed != 7 || snap.Seed.Winner != 2 || snap.FinalTurn != 12 {
		t.Errorf("seed/final-turn mismatch: %+v", snap)
	}
	if len(snap.CommanderZoneVisits) != 1 || len(snap.RegretCards) != 1 || len(snap.MVPCards) != 1 {
		t.Errorf("signal slices not copied through: %+v", snap)
	}
	if snap.CardFirstPlayed["Krenko, Mob Boss"] != 3 || snap.CardFirstPlayed["Tymna the Weaver"] != 4 {
		t.Errorf("CardFirstPlayed not copied: %+v", snap.CardFirstPlayed)
	}
	if len(snap.CommanderNamesBySeat) != 4 {
		t.Fatalf("CommanderNamesBySeat should size to seats; got %d", len(snap.CommanderNamesBySeat))
	}
	if got := snap.CommanderNamesBySeat[0]; len(got) != 1 || got[0] != "Krenko, Mob Boss" {
		t.Errorf("seat 0 commanders: %v", got)
	}
	if got := snap.CommanderNamesBySeat[2]; len(got) != 2 {
		t.Errorf("seat 2 partner pair: %v", got)
	}
}

// CardFirstPlayed is deep-copied so later gs mutations don't leak
// into the persisted snapshot.
func TestSnapshotFromObservation_CardFirstPlayedDeepCopied(t *testing.T) {
	gs := newSnapshotGS(t, 2)
	gs.Seats[0].CommanderNames = []string{"Krenko, Mob Boss"}
	gs.CardFirstPlayed["Krenko, Mob Boss"] = 3

	snap := SnapshotFromObservation(Observation{}, gs)
	gs.CardFirstPlayed["Krenko, Mob Boss"] = 999 // mutate after snapshot

	if snap.CardFirstPlayed["Krenko, Mob Boss"] != 3 {
		t.Errorf("snapshot leaked map reference; got %d (want 3)",
			snap.CardFirstPlayed["Krenko, Mob Boss"])
	}
}

// Marshal/Unmarshal round-trip preserves every field including
// the per-seat commander names + signal arrays.
func TestMarshalSnapshot_RoundTrip(t *testing.T) {
	original := GameObservationSnapshot{
		Seed: GameSeed{RNGSeed: 42, Winner: 1, Turns: 8, DeckKeys: [4]string{"a", "b", "c", "d"}},
		CommanderZoneVisits: []CommanderZoneVisit{
			{Seat: 1, CommanderName: "Atraxa", CastCount: 1, InZoneAtEnd: false, Visits: 1},
		},
		RegretCards: []RegretCard{
			{Seat: 0, CardName: "Plague Wind", CMC: 9, Reason: "stranded_in_hand"},
		},
		MVPCards: []MVPCard{
			{Seat: 1, CardName: "Atraxa", CMC: 4, Score: 8, IsCommander: true},
		},
		CardFirstPlayed:      map[string]int{"Atraxa": 5},
		CommanderNamesBySeat: [][]string{nil, {"Atraxa"}, nil, nil},
		FinalTurn:            8,
	}

	payload, err := MarshalSnapshot(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if payload == "" {
		t.Fatal("payload should not be empty")
	}

	restored, err := UnmarshalSnapshot(payload)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if restored.Seed.RNGSeed != 42 || restored.Seed.Winner != 1 || restored.FinalTurn != 8 {
		t.Errorf("seed/final-turn round-trip lost: %+v", restored)
	}
	if len(restored.CommanderZoneVisits) != 1 || restored.CommanderZoneVisits[0].CommanderName != "Atraxa" {
		t.Errorf("CommanderZoneVisits round-trip lost: %+v", restored.CommanderZoneVisits)
	}
	if len(restored.RegretCards) != 1 || restored.RegretCards[0].CardName != "Plague Wind" {
		t.Errorf("RegretCards round-trip lost: %+v", restored.RegretCards)
	}
	if len(restored.MVPCards) != 1 || !restored.MVPCards[0].IsCommander {
		t.Errorf("MVPCards round-trip lost: %+v", restored.MVPCards)
	}
	if restored.CardFirstPlayed["Atraxa"] != 5 {
		t.Errorf("CardFirstPlayed round-trip lost: %+v", restored.CardFirstPlayed)
	}
	if len(restored.CommanderNamesBySeat) != 4 || len(restored.CommanderNamesBySeat[1]) != 1 {
		t.Errorf("CommanderNamesBySeat round-trip lost: %+v", restored.CommanderNamesBySeat)
	}
}

func TestUnmarshalSnapshot_RejectsMalformedPayload(t *testing.T) {
	_, err := UnmarshalSnapshot("not json{{")
	if err == nil {
		t.Fatal("expected error on malformed payload")
	}
}

// BuildGameSummaryFromSnapshot mirrors BuildGameSummary's output
// shape and stamps data_source = "rich".
func TestBuildGameSummaryFromSnapshot_ProducesRichSummary(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed: GameSeed{RNGSeed: 17, Winner: 1, Turns: 6, DeckKeys: [4]string{"w", "x", "y", "z"}},
		CommanderZoneVisits: []CommanderZoneVisit{
			{Seat: 1, CommanderName: "Edgar Markov", CastCount: 1, InZoneAtEnd: true, Visits: 2},
		},
		MVPCards: []MVPCard{
			{Seat: 1, CardName: "Edgar Markov", CMC: 6, Score: 10, IsCommander: true},
		},
		CardFirstPlayed:      map[string]int{"Edgar Markov": 4},
		CommanderNamesBySeat: [][]string{nil, {"Edgar Markov"}, nil, nil},
		FinalTurn:            6,
	}

	s := BuildGameSummaryFromSnapshot(snap, "combat")

	if s.GameID != "17" || s.Winner != 1 || s.Turns != 6 || s.EndReason != "combat" {
		t.Errorf("metadata mismatch: %+v", s)
	}
	if s.DataSource != "rich" {
		t.Errorf("DataSource: want rich, got %q", s.DataSource)
	}
	if len(s.CommanderZoneVisits) != 1 || len(s.MVPCards) != 1 {
		t.Errorf("signal pass-through failed: %+v", s)
	}
	if len(s.DeckKeys) != 4 {
		t.Errorf("DeckKeys flatten failed: %v", s.DeckKeys)
	}
	// Turning points: commander cast turn 4 + game_end turn 6.
	if len(s.TurningPoints) != 2 {
		t.Fatalf("want 2 turning points, got %d", len(s.TurningPoints))
	}
	if s.TurningPoints[0].Turn != 4 || s.TurningPoints[0].Kind != "commander_first_cast" {
		t.Errorf("position 0: %+v", s.TurningPoints[0])
	}
	if s.TurningPoints[1].Turn != 6 || s.TurningPoints[1].Kind != "game_end" {
		t.Errorf("position 1: %+v", s.TurningPoints[1])
	}
}

// Snapshot → marshal → unmarshal → BuildGameSummaryFromSnapshot
// recreates the same logical GameSummary across the cycle.
func TestPersistLoadCycle_PreservesSummary(t *testing.T) {
	gs := newSnapshotGS(t, 4)
	gs.Turn = 9
	gs.Seats[2].CommanderNames = []string{"Yuriko, the Tiger's Shadow"}
	gs.CardFirstPlayed["Yuriko, the Tiger's Shadow"] = 3

	obs := Observation{
		Seed: GameSeed{RNGSeed: 99, Winner: 2, Turns: 9, DeckKeys: [4]string{"a", "b", "c", "d"}},
		MVPCards: []MVPCard{
			{Seat: 2, CardName: "Yuriko, the Tiger's Shadow", CMC: 3, Score: 7, IsCommander: true},
		},
	}

	snap := SnapshotFromObservation(obs, gs)
	payload, err := MarshalSnapshot(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	restored, err := UnmarshalSnapshot(payload)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	summary := BuildGameSummaryFromSnapshot(restored, "commander_damage")

	if summary.GameID != "99" || summary.Winner != 2 || summary.Turns != 9 {
		t.Errorf("metadata round-trip: %+v", summary)
	}
	if summary.DataSource != "rich" {
		t.Errorf("DataSource: want rich, got %q", summary.DataSource)
	}
	if len(summary.MVPCards) != 1 || summary.MVPCards[0].CardName != "Yuriko, the Tiger's Shadow" {
		t.Errorf("MVPs lost across cycle: %+v", summary.MVPCards)
	}
	if len(summary.TurningPoints) != 2 {
		t.Errorf("turning points lost across cycle: %+v", summary.TurningPoints)
	}
}

// gs nil: snapshot still captures the obs signals + seed, but
// CardFirstPlayed / CommanderNamesBySeat / FinalTurn are zero values.
func TestSnapshotFromObservation_NilGameStateTolerated(t *testing.T) {
	obs := Observation{
		Seed: GameSeed{RNGSeed: 1, Winner: 0, Turns: 3},
		RegretCards: []RegretCard{
			{Seat: 0, CardName: "Bolt", CMC: 1, Reason: "stranded_in_hand"},
		},
	}
	snap := SnapshotFromObservation(obs, nil)

	if len(snap.RegretCards) != 1 {
		t.Errorf("obs signals should still copy with nil gs: %+v", snap)
	}
	if snap.CardFirstPlayed != nil || len(snap.CommanderNamesBySeat) != 0 || snap.FinalTurn != 0 {
		t.Errorf("gs-derived fields should stay zero: %+v", snap)
	}
}
