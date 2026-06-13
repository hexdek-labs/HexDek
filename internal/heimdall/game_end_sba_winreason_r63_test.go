package heimdall

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
)

// game_end_sba_winreason_r63_test.go — end-to-end audit of the
// state-based loss → game-end → win_reason chain (r63).
//
// The existing classify tests hand-stamp Seat.LossReason and call
// HandleSeatElimination directly. These instead drive the REAL engine:
// set the lethal STATE, run gameengine.StateBasedActions so the §704.5/
// §704.6 SBA itself marks the seat Lost (and stamps LossDetail), run
// CheckEnd to crown the survivor, then assert ClassifyKillFinal reports
// the right CAUSE + DIRECTION. This pins that each SBA loss condition
// fires as an SBA (every check) and maps consistently to the canonical
// kill vocabulary.

// commanderGame builds a fresh 2-seat Commander game with both seats at
// 40 life and alive. Seat 0 is the intended winner, seat 1 the victim.
func commanderGame(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.CommanderFormat = true
	for _, s := range gs.Seats {
		s.Life = 40
		s.Lost = false
	}
	return gs
}

// TestGameEnd_SBA_LossCauseAndDirection drives each state-based loss
// condition to the point where the engine's SBA pass ends the game, then
// asserts (a) the SBA marked the victim Lost with the correct structured
// LossDetail, (b) CheckEnd crowned seat 0, and (c) ClassifyKillFinal
// reports the canonical method + victim direction.
func TestGameEnd_SBA_LossCauseAndDirection(t *testing.T) {
	cases := []struct {
		name        string
		setup       func(gs *gameengine.GameState) // make seat 1 lethal
		wantCat     string                         // judge.LossCategory*
		wantRule    string
		wantMethod  string // canonical kill vocabulary
		wantKiller  int    // expected ClassifyKillFinal.KillerSeat
	}{
		{
			name: "commander_damage_704_6c",
			setup: func(gs *gameengine.GameState) {
				gs.Seats[0].CommanderNames = []string{"Winner Cmdr"}
				gs.Seats[1].CommanderDamage = map[int]map[string]int{0: {"Winner Cmdr": 21}}
			},
			wantCat:    judge.LossCategoryCommanderDamage,
			wantRule:   "704.6c",
			wantMethod: "commander",
			wantKiller: 0, // commander's owner
		},
		{
			name: "poison_704_5c",
			setup: func(gs *gameengine.GameState) {
				gs.Seats[1].PoisonCounters = 10
			},
			wantCat:    judge.LossCategoryPoison,
			wantRule:   "704.5c",
			wantMethod: "poison",
			wantKiller: -1, // non-combat shape: not attributed from final state
		},
		{
			name: "deckout_704_5b",
			setup: func(gs *gameengine.GameState) {
				gs.Seats[1].AttemptedEmptyDraw = true
			},
			wantCat:    judge.LossCategoryEmptyLibrary,
			wantRule:   "704.5b",
			wantMethod: "mill",
			wantKiller: -1,
		},
		{
			name: "life_zero_704_5a",
			setup: func(gs *gameengine.GameState) {
				gs.Seats[1].Life = 0
			},
			wantCat:    judge.LossCategoryLife,
			wantRule:   "704.5a",
			wantMethod: "combat", // life-zero with no poison/commander → combat
			wantKiller: 0,        // combat closes are credited to the winner
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := commanderGame(t)
			tc.setup(gs)

			// The SBA itself must mark the seat lost — not a triggered event.
			if !gameengine.StateBasedActions(gs) {
				t.Fatalf("StateBasedActions reported no change; the §704 loss SBA did not fire")
			}
			victim := gs.Seats[1]
			if !victim.Lost {
				t.Fatalf("victim seat 1 not marked Lost by the SBA pass")
			}
			if gs.Seats[0].Lost {
				t.Fatalf("winner seat 0 wrongly marked Lost")
			}
			if victim.LossDetail == nil {
				t.Fatalf("victim has no structured LossDetail — classifier would fall back to string heuristics")
			}
			if victim.LossDetail.Category != tc.wantCat {
				t.Fatalf("LossDetail.Category = %q, want %q", victim.LossDetail.Category, tc.wantCat)
			}
			if victim.LossDetail.Rule != tc.wantRule {
				t.Fatalf("LossDetail.Rule = %q, want %q", victim.LossDetail.Rule, tc.wantRule)
			}

			// CheckEnd crowns the survivor (and runs §800.4a, stamping LostOrder).
			if !gs.CheckEnd() {
				t.Fatalf("CheckEnd did not end the game with one seat remaining")
			}
			if gs.Flags["winner"] != 0 {
				t.Fatalf("winner flag = %d, want 0", gs.Flags["winner"])
			}

			// win_reason cause + direction.
			kc := ClassifyKillFinal(gs, 0, 60)
			if kc.Method != tc.wantMethod {
				t.Fatalf("ClassifyKillFinal.Method = %q, want %q", kc.Method, tc.wantMethod)
			}
			if kc.VictimSeat != 1 {
				t.Fatalf("ClassifyKillFinal.VictimSeat = %d, want 1 (the final eliminated opponent)", kc.VictimSeat)
			}
			if kc.KillerSeat != tc.wantKiller {
				t.Fatalf("ClassifyKillFinal.KillerSeat = %d, want %d", kc.KillerSeat, tc.wantKiller)
			}
		})
	}
}

// TestGameEnd_SimultaneousLifeLoss_IsDraw pins CR §104.3b / §704.7: when
// the last two seats hit 0 life, ONE StateBasedActions call marks BOTH
// Lost (simultaneity), and CheckEnd resolves a draw — no seat is crowned.
func TestGameEnd_SimultaneousLifeLoss_IsDraw(t *testing.T) {
	gs := commanderGame(t)
	gs.Seats[0].Life = 0
	gs.Seats[1].Life = 0

	if !gameengine.StateBasedActions(gs) {
		t.Fatalf("StateBasedActions reported no change; §704.5a did not fire")
	}
	// Both must be marked in the SAME SBA call — that is the simultaneity
	// §104.3b/§704.7 relies on to declare a draw rather than crowning
	// whoever was swept last.
	if !gs.Seats[0].Lost || !gs.Seats[1].Lost {
		t.Fatalf("both seats must be Lost in one SBA pass; got s0=%v s1=%v",
			gs.Seats[0].Lost, gs.Seats[1].Lost)
	}

	if !gs.CheckEnd() {
		t.Fatalf("CheckEnd did not end the game when all seats are Lost")
	}
	if _, ok := gs.Flags["winner"]; ok {
		t.Fatalf("simultaneous all-loss must be a DRAW; winner flag was set to %d", gs.Flags["winner"])
	}
	for i, s := range gs.Seats {
		if s.Won {
			t.Fatalf("seat %d wrongly marked Won in a simultaneous-loss draw", i)
		}
	}
}

// TestGameEnd_SimultaneousLoss_TwoOfFour confirms the non-draw branch: in
// a 4-seat game two seats dying simultaneously to DIFFERENT causes are
// both marked in one SBA pass while the other two survive — the game
// continues (CheckEnd false), and both losers carry the right LossDetail.
func TestGameEnd_SimultaneousLoss_TwoOfFour(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(2)), nil)
	gs.CommanderFormat = true
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.Seats[1].Life = 0          // §704.5a
	gs.Seats[2].PoisonCounters = 10 // §704.5c

	if !gameengine.StateBasedActions(gs) {
		t.Fatalf("StateBasedActions reported no change")
	}
	if !gs.Seats[1].Lost || !gs.Seats[2].Lost {
		t.Fatalf("both lethal seats must be Lost in one pass; s1=%v s2=%v", gs.Seats[1].Lost, gs.Seats[2].Lost)
	}
	if gs.Seats[1].LossDetail == nil || gs.Seats[1].LossDetail.Category != judge.LossCategoryLife {
		t.Fatalf("seat 1 LossDetail mismatch: %+v", gs.Seats[1].LossDetail)
	}
	if gs.Seats[2].LossDetail == nil || gs.Seats[2].LossDetail.Category != judge.LossCategoryPoison {
		t.Fatalf("seat 2 LossDetail mismatch: %+v", gs.Seats[2].LossDetail)
	}
	if gs.Seats[0].Lost || gs.Seats[3].Lost {
		t.Fatalf("survivors wrongly marked Lost: s0=%v s3=%v", gs.Seats[0].Lost, gs.Seats[3].Lost)
	}
	if gs.CheckEnd() {
		t.Fatalf("game must continue with 2 of 4 seats alive")
	}
}
