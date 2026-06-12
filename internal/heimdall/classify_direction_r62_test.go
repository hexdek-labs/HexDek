package heimdall

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/seedcontract"
)

// classify_direction_r62_test.go — fleet review reports 06 + 08.
//
// ClassifyKill inferred the winner's kill method by scanning eliminated
// seats in SEAT order and returning the first classifiable LossReason —
// so in multi-elimination games the stored win_reason usually described
// an EARLIER elimination (often another player's kill), not how the
// winner closed the game. The fix keys the method off the opponent with
// the highest Seat.LostOrder (stamped by HandleSeatElimination in
// elimination order) and routes the vocabulary through the canonical
// seedcontract mapper.

func classifyGame(t *testing.T, seats int) *gameengine.GameState {
	t.Helper()
	return gameengine.NewGameState(seats, rand.New(rand.NewSource(1)), nil)
}

// eliminate marks the seat lost with the given reason and runs the real
// §800.4a pipeline (CheckEnd → HandleSeatElimination), which stamps
// LostOrder and emits the seat_eliminated event — the production path.
func eliminate(t *testing.T, gs *gameengine.GameState, seatIdx int, reason string) {
	t.Helper()
	gs.Seats[seatIdx].Lost = true
	gs.Seats[seatIdx].LossReason = reason
	gameengine.HandleSeatElimination(gs, seatIdx)
}

// TestClassifyKill_DirectionUsesFinalElimination is the headline
// regression: an early poison elimination (someone ELSE's kill) followed
// by the winner's game-closing commander-damage kill must classify as
// "commander". The pre-r62 seat-order scan returned "poison".
func TestClassifyKill_DirectionUsesFinalElimination(t *testing.T) {
	gs := classifyGame(t, 4)
	winner := 3

	// Turn 5: seat 0 dies to poison (seat 1's infect deck, not the winner).
	eliminate(t, gs, 0, "ten or more poison counters (CR 704.5c)")
	// Turn 9: seat 1 dies to combat.
	eliminate(t, gs, 1, "life total 0 or less (CR 704.5a)")
	// Turn 14: the winner closes the game with commander damage.
	eliminate(t, gs, 2, "21+ commander damage from Test Commander (CR 704.6c)")

	if got := ClassifyKill(gs, winner); got != "commander" {
		t.Fatalf("win method must come from the FINAL elimination (commander); got %q (pre-r62 seat-order bug returns the first eliminated seat's method)", got)
	}
}

// TestClassifyKill_PerKillType drives every kill type through the real
// elimination pipeline as the FINAL elimination — preceded by a decoy
// first elimination of a DIFFERENT type so any direction regression
// flips the answer.
func TestClassifyKill_PerKillType(t *testing.T) {
	cases := []struct {
		name         string
		decoyReason  string // first elimination (must NOT win classification)
		finalReason  string // game-closing elimination
		finalState   func(s *gameengine.Seat)
		want         string
	}{
		{
			name:        "combat",
			decoyReason: "ten or more poison counters (CR 704.5c)",
			finalReason: "life total 0 or less (CR 704.5a)",
			finalState:  func(s *gameengine.Seat) { s.Life = 0 },
			want:        "combat",
		},
		{
			name:        "commander_damage",
			decoyReason: "drew from empty library (CR 704.5b)",
			finalReason: "21+ commander damage from Atraxa (CR 704.6c)",
			want:        "commander",
		},
		{
			name:        "poison",
			decoyReason: "life total 0 or less (CR 704.5a)",
			finalReason: "ten or more poison counters (CR 704.5c)",
			finalState:  func(s *gameengine.Seat) { s.PoisonCounters = 10 },
			want:        "poison",
		},
		{
			name:        "mill",
			decoyReason: "life total 0 or less (CR 704.5a)",
			finalReason: "drew from empty library (CR 704.5b)",
			want:        "mill",
		},
		{
			name:        "combo",
			decoyReason: "life total 0 or less (CR 704.5a)",
			finalReason: "lost to infinite combo loop",
			want:        "combo",
		},
		{
			name:        "heuristic_poison_no_reason",
			decoyReason: "life total 0 or less (CR 704.5a)",
			finalReason: "", // no LossReason — state heuristic path
			finalState:  func(s *gameengine.Seat) { s.PoisonCounters = 12; s.Life = 20 },
			want:        "poison",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := classifyGame(t, 3)
			winner := 2
			eliminate(t, gs, 0, tc.decoyReason)
			if tc.finalState != nil {
				tc.finalState(gs.Seats[1])
			}
			eliminate(t, gs, 1, tc.finalReason)

			if got := ClassifyKill(gs, winner); got != tc.want {
				t.Fatalf("final elimination %q: got %q want %q", tc.finalReason, got, tc.want)
			}
		})
	}
}

// TestClassifyKillWithMaxTurns_TimeoutViaCanonicalMapper pins that the
// turn-cap shape routes through seedcontract.KillMethodFromEndReason —
// the single source for the "timeout" spelling.
func TestClassifyKillWithMaxTurns_TimeoutViaCanonicalMapper(t *testing.T) {
	gs := classifyGame(t, 4)
	gs.Turn = 60
	if got := ClassifyKillWithMaxTurns(gs, 0, 60); got != "timeout" {
		t.Fatalf("turn-cap game: got %q want timeout", got)
	}
	if got := seedcontract.KillMethodFromEndReason("turn_cap"); got != "timeout" {
		t.Fatalf("canonical mapper drifted: turn_cap -> %q", got)
	}
}

// TestClassifyKill_EventLogFallback covers states predating the
// LostOrder stamp (e.g. deserialized legacy games): the last
// seat_eliminated event names the final victim.
func TestClassifyKill_EventLogFallback(t *testing.T) {
	gs := classifyGame(t, 4)
	winner := 3
	// Mark seats lost WITHOUT the pipeline (no LostOrder stamp), then
	// append the events in elimination order.
	gs.Seats[0].Lost = true
	gs.Seats[0].LossReason = "ten or more poison counters (CR 704.5c)"
	gs.Seats[1].Lost = true
	gs.Seats[1].LossReason = "21+ commander damage from Edgar (CR 704.6c)"
	gs.LogEvent(gameengine.Event{Kind: "seat_eliminated", Seat: 0})
	gs.LogEvent(gameengine.Event{Kind: "seat_eliminated", Seat: 1})

	if got := ClassifyKill(gs, winner); got != "commander" {
		t.Fatalf("event-log fallback must use the LAST seat_eliminated; got %q want commander", got)
	}
}

// TestClassifyKill_AgreesWithAnalyticsKillRecord pins the cross-system
// contract the fleet review asked for: for each kill type, heimdall's
// win_reason method and the analytics threat-graph KillRecord for the
// FINAL victim normalize to the same canonical method via
// seedcontract.CanonicalKillMethod.
func TestClassifyKill_AgreesWithAnalyticsKillRecord(t *testing.T) {
	type killCase struct {
		name        string
		finalReason string
		finalState  func(s *gameengine.Seat)
		// events the analytics inferKiller needs to attribute the kill
		// (killer seat 2 → victim seat 1).
		killEvents func(gs *gameengine.GameState)
	}
	cases := []killCase{
		{
			name:        "combat",
			finalReason: "life total 0 or less (CR 704.5a)",
			finalState:  func(s *gameengine.Seat) { s.Life = 0 },
			killEvents: func(gs *gameengine.GameState) {
				gs.LogEvent(gameengine.Event{Kind: "damage", Seat: 2, Target: 1, Amount: 40,
					Source: "Big Dragon", Details: map[string]interface{}{"combat": true}})
			},
		},
		{
			name:        "commander_damage",
			finalReason: "21+ commander damage from Big Dragon (CR 704.6c)",
			killEvents: func(gs *gameengine.GameState) {
				gs.LogEvent(gameengine.Event{Kind: "damage", Seat: 2, Target: 1, Amount: 21,
					Source: "Big Dragon", Details: map[string]interface{}{"commander": true, "combat": true}})
			},
		},
		{
			name:        "poison",
			finalReason: "ten or more poison counters (CR 704.5c)",
			finalState:  func(s *gameengine.Seat) { s.PoisonCounters = 10 },
			killEvents: func(gs *gameengine.GameState) {
				gs.LogEvent(gameengine.Event{Kind: "damage", Seat: 2, Target: 1, Amount: 10,
					Source: "Blighted Agent", Details: map[string]interface{}{"infect": true}})
			},
		},
		{
			name:        "mill",
			finalReason: "drew from empty library (CR 704.5b)",
			killEvents: func(gs *gameengine.GameState) {
				gs.LogEvent(gameengine.Event{Kind: "mill", Seat: 2, Target: 1, Amount: 30,
					Source: "Maddening Cacophony"})
			},
		},
	}
	commanders := []string{"Seat0 Cmdr", "Victim Cmdr", "Winner Cmdr"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := classifyGame(t, 3)
			gs.EventPolicy = gameengine.EventLogFull
			winner := 2

			// Decoy early elimination of a different type.
			eliminate(t, gs, 0, "lost to infinite combo loop")

			if tc.finalState != nil {
				tc.finalState(gs.Seats[1])
			}
			tc.killEvents(gs)
			eliminate(t, gs, 1, tc.finalReason)

			heimdallMethod := ClassifyKill(gs, winner)

			records := analytics.ExtractKillRecords(gs.EventLog, 3, commanders, winner, "test-game")
			var finalRec *analytics.KillRecord
			for i := range records {
				if records[i].VictimSeat == 1 {
					finalRec = &records[i]
				}
			}
			if finalRec == nil {
				t.Fatalf("analytics produced no KillRecord for the final victim (records=%d)", len(records))
			}

			h := seedcontract.CanonicalKillMethod(heimdallMethod)
			a := seedcontract.CanonicalKillMethod(finalRec.Method)
			if h != a {
				t.Fatalf("heimdall and analytics disagree on the final kill: heimdall %q (canonical %q) vs analytics %q (canonical %q)",
					heimdallMethod, h, finalRec.Method, a)
			}
		})
	}
}
