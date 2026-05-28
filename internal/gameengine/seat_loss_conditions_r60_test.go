package gameengine

import (
	"testing"
)

// R60 — pins the two-level loss/win architecture port from
// internal/game/ (#680) to the real rules engine.
//
// Level 1: Seat.CheckLossConditions — pure read-only classifier across
// §704.5a / §704.5b / §704.5c / §704.6c. Tested in isolation; no SBA
// machinery involved.
//
// Level 2: GameState.CheckEnd — master aggregator. Already handles
// §104.2a (last-seat) and §104.3b (simultaneous-elim-draw); this wave
// adds §104.2c "you win the game" + §104.3b's "simultaneous wins ⇒
// draw" branch via the Seat.Won flag, with §104.3a (loss beats win
// when simultaneous on the same player) enforced by the `!s.Lost`
// gate on the winner-scan.

// -----------------------------------------------------------------------
// Per-clause Seat tests — pure CheckLossConditions.
// -----------------------------------------------------------------------

func TestSeat_CheckLossConditions_HealthyNotLost(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	cause, lost := gs.Seats[0].CheckLossConditions(gs)
	if lost {
		t.Fatalf("healthy seat should not be lost; cause=%q", cause)
	}
	if cause != LossNone {
		t.Errorf("expected LossNone; got %q", cause)
	}
}

func TestSeat_CheckLossConditions_Life_704_5a(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	gs.Seats[0].Life = 0
	cause, lost := gs.Seats[0].CheckLossConditions(gs)
	if !lost || cause != LossLife {
		t.Errorf("Life=0 should be LossLife; got cause=%q lost=%v", cause, lost)
	}
	gs.Seats[0].Life = -5
	cause, lost = gs.Seats[0].CheckLossConditions(gs)
	if !lost || cause != LossLife {
		t.Errorf("Life=-5 should be LossLife; got cause=%q lost=%v", cause, lost)
	}
}

func TestSeat_CheckLossConditions_EmptyLibrary_704_5b(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].AttemptedEmptyDraw = true
	cause, lost := gs.Seats[0].CheckLossConditions(gs)
	if !lost || cause != LossEmptyLibrary {
		t.Errorf("AttemptedEmptyDraw should be LossEmptyLibrary; got cause=%q lost=%v", cause, lost)
	}
}

func TestSeat_CheckLossConditions_Poison_704_5c(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].PoisonCounters = 10
	cause, lost := gs.Seats[0].CheckLossConditions(gs)
	if !lost || cause != LossPoison {
		t.Errorf("poison=10 should be LossPoison; got cause=%q lost=%v", cause, lost)
	}
	gs.Seats[0].PoisonCounters = 25
	if _, lost := gs.Seats[0].CheckLossConditions(gs); !lost {
		t.Error("poison=25 should still be lost")
	}
}

func TestSeat_CheckLossConditions_CommanderDamage_704_6c(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	gs.CommanderFormat = true
	gs.Seats[0].Life = 40
	gs.Seats[0].CommanderDamage = map[int]map[string]int{
		1: {"Edgar Markov": 21},
	}
	cause, lost := gs.Seats[0].CheckLossConditions(gs)
	if !lost || cause != LossCommanderDamage {
		t.Errorf("21 commander damage should be LossCommanderDamage; got cause=%q lost=%v", cause, lost)
	}
}

func TestSeat_CheckLossConditions_CommanderDamage_PartnerSplitNotLost(t *testing.T) {
	// Partner pair: 15 from Kraum + 15 from Tymna → neither bucket
	// crosses 21 → seat not lost. Defends against an over-broad
	// classifier that sums across commanders.
	gs := newMultiplayerGame(t, 2)
	gs.CommanderFormat = true
	gs.Seats[0].Life = 40
	gs.Seats[0].CommanderDamage = map[int]map[string]int{
		1: {"Kraum, Ludevic's Opus": 15, "Tymna the Weaver": 15},
	}
	if _, lost := gs.Seats[0].CheckLossConditions(gs); lost {
		t.Error("partner-split damage should NOT trigger §704.6c (no single bucket ≥ 21)")
	}
}

func TestSeat_CheckLossConditions_CommanderDamage_FormatGated(t *testing.T) {
	// Non-Commander format → §704.6c silently skipped even with 21+
	// damage on record.
	gs := newMultiplayerGame(t, 2)
	gs.CommanderFormat = false
	gs.Seats[0].Life = 40
	gs.Seats[0].CommanderDamage = map[int]map[string]int{
		1: {"Edgar Markov": 30},
	}
	if _, lost := gs.Seats[0].CheckLossConditions(gs); lost {
		t.Error("commander damage clause must be format-gated; got lost=true in non-Commander game")
	}
}

func TestSeat_CheckLossConditions_NilGameStateOKForLifeAndPoison(t *testing.T) {
	// Passing nil gs is documented as safe — commander-damage clause
	// silently skipped. Life / EmptyLibrary / Poison clauses still
	// evaluate.
	s := &Seat{Life: 0}
	if cause, lost := s.CheckLossConditions(nil); !lost || cause != LossLife {
		t.Errorf("Life check should fire with nil gs; cause=%q lost=%v", cause, lost)
	}
}

func TestSeat_CheckLossConditions_LifeFirstWhenMultipleClauses(t *testing.T) {
	// Multi-cause: Life=-5 AND poison=12 AND empty-draw flag. Method
	// returns first match per documented ordering (Life → EmptyLibrary
	// → Poison → CommanderDamage). Pinning order keeps action-log
	// payloads stable.
	gs := newMultiplayerGame(t, 2)
	gs.Seats[0].Life = -5
	gs.Seats[0].AttemptedEmptyDraw = true
	gs.Seats[0].PoisonCounters = 12
	cause, lost := gs.Seats[0].CheckLossConditions(gs)
	if !lost {
		t.Fatal("multi-cause seat must be lost")
	}
	if cause != LossLife {
		t.Errorf("ordering: Life first; got %q", cause)
	}
}

func TestSeat_CheckLossConditions_NilSeatSafe(t *testing.T) {
	var s *Seat
	if cause, lost := s.CheckLossConditions(nil); lost || cause != LossNone {
		t.Errorf("nil seat should report (LossNone, false); got (%q, %v)", cause, lost)
	}
}

// -----------------------------------------------------------------------
// Aggregation tests — CheckEnd with §104.2c via Seat.Won.
// -----------------------------------------------------------------------

func TestCheckEnd_WinEffectEndsGameWithHealthyOpponents_104_2c(t *testing.T) {
	// Approach of the Second Sun / Felidar Sovereign style: Won flips
	// on a healthy seat while opponents are also healthy. Before this
	// R60 wave, the engine left the game running because no opponent
	// was Lost. The §104.2c short-circuit now ends the game on the
	// next CheckEnd call.
	gs := newMultiplayerGame(t, 4)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.Seats[2].Won = true
	if !gs.CheckEnd() {
		t.Fatal("CheckEnd must return true when a seat has Won=true")
	}
	if gs.Flags["winner"] != 2 {
		t.Errorf("winner should be seat 2; got %d", gs.Flags["winner"])
	}
	if gs.Flags["ended"] != 1 {
		t.Error("Flags[\"ended\"] should be 1")
	}
}

func TestCheckEnd_104_3a_LossOverridesWinOnSameSeat(t *testing.T) {
	// CR §104.3a: "If a player would both win and lose the game
	// simultaneously, that player loses." Approach controller dropped
	// to life=0 the same turn the win effect resolved → §704.5a SBA
	// marks Lost → §104.3a says the win doesn't apply → game falls
	// through to §104.2a alive-counting.
	gs := newMultiplayerGame(t, 2)
	gs.Seats[0].Life = 0
	gs.Seats[0].Won = true  // Approach resolved
	gs.Seats[0].Lost = true // sba704_5a fired in same window
	gs.Seats[0].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Seats[1].Life = 40

	if !gs.CheckEnd() {
		t.Fatal("CheckEnd should still end the game (seat 0 lost → seat 1 last alive)")
	}
	if gs.Flags["winner"] != 1 {
		t.Errorf("seat 1 should win via §104.2a (loss beat win on seat 0); got winner=%d", gs.Flags["winner"])
	}
}

func TestCheckEnd_104_3b_MultipleSimultaneousWinsDraw(t *testing.T) {
	// Mirrored Approach decks: two seats trigger a win effect in the
	// same window. §104.3b: simultaneous wins by multiple players ⇒
	// game is a draw.
	gs := newMultiplayerGame(t, 4)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.Seats[1].Won = true
	gs.Seats[2].Won = true

	if !gs.CheckEnd() {
		t.Fatal("CheckEnd should end the game on simultaneous wins")
	}
	if _, hasWinner := gs.Flags["winner"]; hasWinner {
		t.Errorf("simultaneous win effects must end as a draw; got winner=%d", gs.Flags["winner"])
	}
	if gs.Flags["ended"] != 1 {
		t.Error("Flags[\"ended\"] should be 1 even on draw")
	}
}

func TestCheckEnd_104_2a_LastSeatStandingUnchanged(t *testing.T) {
	// Pin the pre-existing §104.2a path — last seat alive wins. The
	// §104.2c branch must not perturb this path.
	gs := newMultiplayerGame(t, 4)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	gs.Seats[0].Lost = true
	gs.Seats[0].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Seats[1].Lost = true
	gs.Seats[1].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Seats[2].Lost = true
	gs.Seats[2].LossReason = "ten or more poison counters (CR 704.5c)"
	// Seat 3 alone alive.

	if !gs.CheckEnd() {
		t.Fatal("CheckEnd should fire on 1-alive")
	}
	if gs.Flags["winner"] != 3 {
		t.Errorf("seat 3 should win via §104.2a; got winner=%d", gs.Flags["winner"])
	}
	if !gs.Seats[3].Won {
		t.Error("CheckEnd should flip Won on the last-seat-standing winner")
	}
}

func TestCheckEnd_104_3b_ZeroAliveDraw(t *testing.T) {
	// Pin the pre-existing §104.3b path — simultaneous elimination
	// draw. Must remain unchanged.
	gs := newMultiplayerGame(t, 2)
	gs.Seats[0].Lost = true
	gs.Seats[0].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Seats[1].Lost = true
	gs.Seats[1].LossReason = "life total 0 or less (CR 704.5a)"

	if !gs.CheckEnd() {
		t.Fatal("CheckEnd should fire on 0-alive")
	}
	if _, hasWinner := gs.Flags["winner"]; hasWinner {
		t.Error("simultaneous elimination should end as a draw")
	}
}

func TestCheckEnd_GameContinuesWithNoEndCondition(t *testing.T) {
	// Multi-alive + no Won flags → game continues. Pins the negative
	// path (CheckEnd doesn't false-positive after the §104.2c add).
	gs := newMultiplayerGame(t, 4)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	if gs.CheckEnd() {
		t.Error("CheckEnd should return false when 4 seats are alive and none have Won")
	}
	if _, ended := gs.Flags["ended"]; ended {
		t.Error("Flags[\"ended\"] should not be set when game is still in progress")
	}
}
