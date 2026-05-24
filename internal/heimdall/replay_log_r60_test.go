package heimdall

import (
	"testing"
)

// Simple snapshot with one cast + game_end. Sort puts the cast
// first, game_end second.
func TestBuildReplayLog_MinimalSnapshot(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed: GameSeed{RNGSeed: 1, Winner: 0, Turns: 5},
		CardFirstPlayed: map[string]int{
			"Sol Ring": 1,
		},
		FinalTurn: 5,
	}
	events := BuildReplayLog(snap)

	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	if events[0].Kind != "card_first_played" || events[0].Turn != 1 || events[0].CardName != "Sol Ring" {
		t.Errorf("position 0: %+v", events[0])
	}
	if events[0].Seat != -1 || events[0].IsCommander {
		t.Errorf("Sol Ring is not a commander; seat should be -1: %+v", events[0])
	}
	if events[1].Kind != "game_end" || events[1].Turn != 5 || events[1].Seat != 0 {
		t.Errorf("position 1: %+v", events[1])
	}
}

// Commander match: card_first_played for a commander name resolves
// to the seat that owns it.
func TestBuildReplayLog_CommanderAttributedToSeat(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed: GameSeed{Winner: 1, Turns: 8},
		CardFirstPlayed: map[string]int{
			"Atraxa, Praetors' Voice": 4,
			"Sol Ring":                1,
		},
		CommanderNamesBySeat: [][]string{nil, {"Atraxa, Praetors' Voice"}, nil, nil},
		FinalTurn:            8,
	}
	events := BuildReplayLog(snap)

	byName := map[string]ReplayEvent{}
	for _, e := range events {
		if e.Kind == "card_first_played" {
			byName[e.CardName] = e
		}
	}
	if ev := byName["Atraxa, Praetors' Voice"]; ev.Seat != 1 || !ev.IsCommander {
		t.Errorf("Atraxa should attribute to seat 1 as commander; got %+v", ev)
	}
	if ev := byName["Sol Ring"]; ev.Seat != -1 || ev.IsCommander {
		t.Errorf("Sol Ring should stay seat=-1; got %+v", ev)
	}
}

// MVP emergence emits its own seat-attributed event at TurnPlayed
// with the score carried through.
func TestBuildReplayLog_MVPEmergedEvent(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed: GameSeed{Winner: 2, Turns: 9},
		MVPCards: []MVPCard{
			{Seat: 2, CardName: "Craterhoof Behemoth", CMC: 8, TurnPlayed: 6, Score: 8},
		},
		FinalTurn: 9,
	}
	events := BuildReplayLog(snap)

	var mvp *ReplayEvent
	for i := range events {
		if events[i].Kind == "mvp_emerged" {
			mvp = &events[i]
			break
		}
	}
	if mvp == nil {
		t.Fatal("no mvp_emerged event")
	}
	if mvp.Turn != 6 || mvp.Seat != 2 || mvp.Score != 8 || mvp.CardName != "Craterhoof Behemoth" {
		t.Errorf("MVP event: %+v", mvp)
	}
}

// MVP with TurnPlayed=0 (e.g., a token that never had a
// CardFirstPlayed entry) is silently skipped — no spurious turn-0
// event in the log.
func TestBuildReplayLog_MVPWithUnknownTurnPlayedSkipped(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed: GameSeed{Winner: 0, Turns: 3},
		MVPCards: []MVPCard{
			{Seat: 0, CardName: "Soldier Token", CMC: 0, TurnPlayed: 0, Score: 0},
		},
		FinalTurn: 3,
	}
	events := BuildReplayLog(snap)

	for _, e := range events {
		if e.Kind == "mvp_emerged" {
			t.Errorf("unknown-TurnPlayed MVP should be skipped; got %+v", e)
		}
	}
	if len(events) != 1 || events[0].Kind != "game_end" {
		t.Errorf("only game_end should remain; got %+v", events)
	}
}

// game_end always sorts last within its turn, even when other
// events share that turn.
func TestBuildReplayLog_GameEndSortsLastWithinTurn(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed: GameSeed{Winner: 0, Turns: 5},
		CardFirstPlayed: map[string]int{
			"Cyclonic Rift": 5,
			"Counterspell":  5,
		},
		MVPCards: []MVPCard{
			{Seat: 0, CardName: "Cyclonic Rift", CMC: 7, TurnPlayed: 5, Score: 7},
		},
		FinalTurn: 5,
	}
	events := BuildReplayLog(snap)

	// Last event on turn 5 must be game_end.
	last := events[len(events)-1]
	if last.Kind != "game_end" {
		t.Errorf("game_end should sort last on its turn; got last event %+v", last)
	}
	// Earlier turn-5 events should be card_first_played + mvp_emerged.
	kinds := []string{}
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	if kinds[len(kinds)-1] != "game_end" {
		t.Errorf("kinds order: %v", kinds)
	}
}

// Turn-zero entries in CardFirstPlayed are skipped (placeholder
// sentinel values, not actual casts).
func TestBuildReplayLog_TurnZeroEntriesSkipped(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed: GameSeed{Winner: 0, Turns: 4},
		CardFirstPlayed: map[string]int{
			"Sol Ring":    1,
			"Mana Vault":  0, // sentinel — should be skipped
			"Mana Crypt": -1, // negative — should be skipped
		},
		FinalTurn: 4,
	}
	events := BuildReplayLog(snap)

	for _, e := range events {
		if e.CardName == "Mana Vault" || e.CardName == "Mana Crypt" {
			t.Errorf("skipped entry leaked: %+v", e)
		}
	}
}

// Draw / no-winner: game_end still emits with seat=-1.
func TestBuildReplayLog_DrawEmitsMinusOneWinner(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed:      GameSeed{Winner: -1, Turns: 80},
		FinalTurn: 80,
	}
	events := BuildReplayLog(snap)
	if len(events) != 1 || events[0].Seat != -1 {
		t.Errorf("draw should emit single game_end seat=-1; got %+v", events)
	}
}

// Empty snapshot: still emits the game_end event so streaming
// callers always have at least one record.
func TestBuildReplayLog_EmptySnapshotEmitsGameEnd(t *testing.T) {
	events := BuildReplayLog(GameObservationSnapshot{})
	if len(events) != 1 || events[0].Kind != "game_end" {
		t.Errorf("empty snapshot should still emit game_end; got %+v", events)
	}
}

// Multi-turn timeline: events sort by turn ascending then by kind
// within a turn (card_first_played < mvp_emerged < game_end).
func TestBuildReplayLog_ChronologicalOrdering(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed: GameSeed{Winner: 1, Turns: 9},
		CardFirstPlayed: map[string]int{
			"Sol Ring":   1,
			"Counterspell": 3,
			"Atraxa":     5,
		},
		MVPCards: []MVPCard{
			{Seat: 1, CardName: "Atraxa", CMC: 4, TurnPlayed: 5, Score: 8, IsCommander: true},
		},
		CommanderNamesBySeat: [][]string{nil, {"Atraxa"}, nil, nil},
		FinalTurn:            9,
	}
	events := BuildReplayLog(snap)

	// Expected order:
	// turn 1: card_first_played Sol Ring
	// turn 3: card_first_played Counterspell
	// turn 5: card_first_played Atraxa
	// turn 5: mvp_emerged Atraxa
	// turn 9: game_end
	if len(events) != 5 {
		t.Fatalf("want 5 events, got %d (%+v)", len(events), events)
	}
	wantTurns := []int{1, 3, 5, 5, 9}
	wantKinds := []string{"card_first_played", "card_first_played", "card_first_played", "mvp_emerged", "game_end"}
	for i, e := range events {
		if e.Turn != wantTurns[i] || e.Kind != wantKinds[i] {
			t.Errorf("position %d: want (turn=%d kind=%s), got (turn=%d kind=%s name=%q)",
				i, wantTurns[i], wantKinds[i], e.Turn, e.Kind, e.CardName)
		}
	}
}

// Same-turn tie-break: when two card_first_played entries share a
// turn, sort by card name ascending so output is byte-stable.
func TestBuildReplayLog_SameTurnSortsByNameAscending(t *testing.T) {
	snap := GameObservationSnapshot{
		Seed: GameSeed{Winner: 0, Turns: 2},
		CardFirstPlayed: map[string]int{
			"Zur":      1,
			"Atraxa":   1,
			"Krenko":   1,
		},
		FinalTurn: 2,
	}
	events := BuildReplayLog(snap)

	wantOrder := []string{"Atraxa", "Krenko", "Zur"}
	for i, name := range wantOrder {
		if events[i].CardName != name {
			t.Errorf("position %d: want %q, got %q", i, name, events[i].CardName)
		}
	}
}
