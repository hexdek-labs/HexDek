package heimdall

import (
	"encoding/json"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// mulligan_stats_r60_test.go — pins ExtractMulliganStats + the
// GameSummary / snapshot surface that carries it.

func newMulliganGameState(seats int) *gameengine.GameState {
	gs := &gameengine.GameState{
		Seats: make([]*gameengine.Seat, seats),
	}
	for i := range gs.Seats {
		gs.Seats[i] = &gameengine.Seat{}
	}
	return gs
}

func mhe(name string, types ...string) gameengine.MulliganHandEntry {
	return gameengine.MulliganHandEntry{Name: name, Types: types}
}

// -----------------------------------------------------------------------------
// Keepable rule: 2-5 lands AND at least one action card.
// -----------------------------------------------------------------------------

func TestExtractMulliganStats_KeepableHand(t *testing.T) {
	gs := newMulliganGameState(2)
	gs.MulliganHistory = []gameengine.SeatMulliganStats{
		{
			MulligansTaken: 0,
			OpeningHand: []gameengine.MulliganHandEntry{
				mhe("Forest", "land"),
				mhe("Mountain", "land"),
				mhe("Plains", "land"),
				mhe("Lightning Bolt", "instant"),
				mhe("Grizzly Bears", "creature"),
				mhe("Wrath of God", "sorcery"),
				mhe("Sol Ring", "artifact"),
			},
		},
		{},
	}
	stats := ExtractMulliganStats(gs)
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat (seat 1 empty), got %d", len(stats))
	}
	s := stats[0]
	if s.Seat != 0 {
		t.Errorf("Seat = %d, want 0", s.Seat)
	}
	if s.MulligansTaken != 0 {
		t.Errorf("MulligansTaken = %d, want 0", s.MulligansTaken)
	}
	if s.OpeningHandSize != 7 {
		t.Errorf("OpeningHandSize = %d, want 7", s.OpeningHandSize)
	}
	if s.Lands != 3 {
		t.Errorf("Lands = %d, want 3", s.Lands)
	}
	if s.ActionCards != 3 {
		t.Errorf("ActionCards = %d, want 3 (bolt+bears+wrath; Sol Ring is artifact-only)", s.ActionCards)
	}
	if !s.Keepable {
		t.Errorf("hand should be keepable (3 lands + 3 actions)")
	}
}

func TestExtractMulliganStats_LandFloodNotKeepable(t *testing.T) {
	gs := newMulliganGameState(1)
	gs.MulliganHistory = []gameengine.SeatMulliganStats{
		{
			MulligansTaken: 1,
			OpeningHand: []gameengine.MulliganHandEntry{
				mhe("Forest", "land"),
				mhe("Mountain", "land"),
				mhe("Plains", "land"),
				mhe("Island", "land"),
				mhe("Swamp", "land"),
				mhe("Wastes", "land"),
			},
		},
	}
	stats := ExtractMulliganStats(gs)
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].Lands != 6 {
		t.Errorf("Lands = %d, want 6", stats[0].Lands)
	}
	if stats[0].Keepable {
		t.Errorf("6-land hand should NOT be keepable (>5 lands)")
	}
}

func TestExtractMulliganStats_NoLandsNotKeepable(t *testing.T) {
	gs := newMulliganGameState(1)
	gs.MulliganHistory = []gameengine.SeatMulliganStats{
		{
			MulligansTaken: 0,
			OpeningHand: []gameengine.MulliganHandEntry{
				mhe("Lightning Bolt", "instant"),
				mhe("Grizzly Bears", "creature"),
				mhe("Counterspell", "instant"),
				mhe("Wrath of God", "sorcery"),
				mhe("Jace, the Mind Sculptor", "planeswalker"),
				mhe("Day of Judgment", "sorcery"),
				mhe("Snapcaster Mage", "creature"),
			},
		},
	}
	stats := ExtractMulliganStats(gs)
	if stats[0].Keepable {
		t.Errorf("0-land hand should NOT be keepable (<2 lands)")
	}
	if stats[0].ActionCards != 7 {
		t.Errorf("ActionCards = %d, want 7", stats[0].ActionCards)
	}
}

func TestExtractMulliganStats_NoActionNotKeepable(t *testing.T) {
	gs := newMulliganGameState(1)
	gs.MulliganHistory = []gameengine.SeatMulliganStats{
		{
			MulligansTaken: 0,
			OpeningHand: []gameengine.MulliganHandEntry{
				mhe("Forest", "land"),
				mhe("Mountain", "land"),
				mhe("Plains", "land"),
				mhe("Sol Ring", "artifact"),
				mhe("Mana Vault", "artifact"),
				mhe("Mox Diamond", "artifact"),
				mhe("Chrome Mox", "artifact"),
			},
		},
	}
	stats := ExtractMulliganStats(gs)
	if stats[0].Keepable {
		t.Errorf("artifact-only hand (no creatures/spells) should NOT be keepable per Freya rule")
	}
	if stats[0].Lands != 3 {
		t.Errorf("Lands = %d, want 3", stats[0].Lands)
	}
	if stats[0].ActionCards != 0 {
		t.Errorf("ActionCards = %d, want 0 (artifacts don't count)", stats[0].ActionCards)
	}
}

func TestExtractMulliganStats_CreatureLandCountsAsLand(t *testing.T) {
	// A creature-land (Dryad Arbor, animated state) still produces
	// mana on the turn it ETBs — count it as a land for the keepable
	// rule, not as a play.
	gs := newMulliganGameState(1)
	gs.MulliganHistory = []gameengine.SeatMulliganStats{
		{
			MulligansTaken: 0,
			OpeningHand: []gameengine.MulliganHandEntry{
				mhe("Dryad Arbor", "land", "creature"),
				mhe("Forest", "land"),
				mhe("Plains", "land"),
				mhe("Sol Ring", "artifact"),
				mhe("Mana Crypt", "artifact"),
				mhe("Mox Opal", "artifact"),
				mhe("Mox Amber", "artifact"),
			},
		},
	}
	stats := ExtractMulliganStats(gs)
	if stats[0].Lands != 3 {
		t.Errorf("Lands = %d, want 3 (Dryad Arbor + Forest + Plains)", stats[0].Lands)
	}
	if stats[0].ActionCards != 0 {
		t.Errorf("ActionCards = %d, want 0 (Dryad Arbor must not double-count)", stats[0].ActionCards)
	}
}

// -----------------------------------------------------------------------------
// Multi-seat / sorting / skip-empty.
// -----------------------------------------------------------------------------

func TestExtractMulliganStats_PerSeatSortedAscending(t *testing.T) {
	gs := newMulliganGameState(4)
	gs.MulliganHistory = make([]gameengine.SeatMulliganStats, 4)
	for i := 0; i < 4; i++ {
		gs.MulliganHistory[i] = gameengine.SeatMulliganStats{
			MulligansTaken: i, // 0, 1, 2, 3
			OpeningHand: []gameengine.MulliganHandEntry{
				mhe("Forest", "land"),
				mhe("Mountain", "land"),
				mhe("Bolt", "instant"),
			},
		}
	}
	stats := ExtractMulliganStats(gs)
	if len(stats) != 4 {
		t.Fatalf("expected 4 stats, got %d", len(stats))
	}
	for i, s := range stats {
		if s.Seat != i {
			t.Errorf("stats[%d].Seat = %d, want %d", i, s.Seat, i)
		}
		if s.MulligansTaken != i {
			t.Errorf("stats[%d].MulligansTaken = %d, want %d", i, s.MulligansTaken, i)
		}
	}
}

func TestExtractMulliganStats_NilGameStateReturnsNil(t *testing.T) {
	if stats := ExtractMulliganStats(nil); stats != nil {
		t.Errorf("nil gs should return nil, got %v", stats)
	}
}

func TestExtractMulliganStats_EmptyHistoryReturnsNil(t *testing.T) {
	gs := newMulliganGameState(2)
	if stats := ExtractMulliganStats(gs); stats != nil {
		t.Errorf("empty MulliganHistory should return nil, got %v", stats)
	}
}

// -----------------------------------------------------------------------------
// GameSummary + snapshot wiring.
// -----------------------------------------------------------------------------

func TestBuildGameSummary_IncludesMulliganStats(t *testing.T) {
	gs := newMulliganGameState(2)
	gs.Turn = 5
	obs := Observation{
		Seed: GameSeed{RNGSeed: 7, Winner: 1, Turns: 5},
		MulliganStats: []MulliganStat{
			{Seat: 0, MulligansTaken: 2, OpeningHandSize: 5, Lands: 2, ActionCards: 2, Keepable: true},
			{Seat: 1, MulligansTaken: 0, OpeningHandSize: 7, Lands: 4, ActionCards: 3, Keepable: true},
		},
	}
	summary := BuildGameSummary(obs, gs, "combat")
	if len(summary.MulliganStats) != 2 {
		t.Fatalf("summary should carry 2 mulligan stats, got %d", len(summary.MulliganStats))
	}
	if summary.MulliganStats[0].MulligansTaken != 2 {
		t.Errorf("seat 0 mulligans should be 2, got %d", summary.MulliganStats[0].MulligansTaken)
	}
}

func TestSnapshotRoundTrip_PreservesMulliganStats(t *testing.T) {
	obs := Observation{
		Seed: GameSeed{RNGSeed: 42, Winner: 0, Turns: 9},
		MulliganStats: []MulliganStat{
			{Seat: 2, MulligansTaken: 1, OpeningHandSize: 6, Lands: 3, ActionCards: 2, Keepable: true},
		},
	}
	snap := SnapshotFromObservation(obs, nil)
	payload, err := MarshalSnapshot(snap)
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}
	// Sanity-check that the JSON field name is the documented one
	// (we promise this in the schema and want a regression alarm if
	// it ever drifts).
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["mulligan_stats"]; !ok {
		t.Errorf("snapshot JSON missing 'mulligan_stats' key; got keys %v", keysOf(raw))
	}
	restored, err := UnmarshalSnapshot(payload)
	if err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}
	if len(restored.MulliganStats) != 1 {
		t.Fatalf("round-tripped snapshot lost MulliganStats: got %d", len(restored.MulliganStats))
	}
	if restored.MulliganStats[0].MulligansTaken != 1 {
		t.Errorf("MulligansTaken lost across round-trip: got %d", restored.MulliganStats[0].MulligansTaken)
	}
	if restored.MulliganStats[0].Seat != 2 {
		t.Errorf("Seat lost across round-trip: got %d", restored.MulliganStats[0].Seat)
	}
	summary := BuildGameSummaryFromSnapshot(restored, "combat")
	if len(summary.MulliganStats) != 1 {
		t.Errorf("summary from snapshot missing mulligan stats")
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
