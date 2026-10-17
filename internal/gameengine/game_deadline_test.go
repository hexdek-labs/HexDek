package gameengine

import (
	"testing"
	"time"
)

// game_deadline_test.go — the wall-clock backstop for runaway loops that
// no counter-based guard happens to watch (2026-10-08: one grinder game
// held a core for sixteen days, zero loop guards fired).
//
// The two properties that matter most are the negative ones: a game with
// no deadline armed must behave exactly as before, and an armed deadline
// that has not passed must be inert. Those protect every deterministic
// caller (tests, parity, the correctness harness) from the nondeterminism
// that got the previous wall-clock budget removed.

func deadlineTestState() *GameState {
	return &GameState{
		Flags: map[string]int{},
		Seats: []*Seat{{Life: 40}, {Life: 40}},
		Turn:  7,
	}
}

// TestGameDeadline_UnarmedIsInert — the default. No flag, no deadline, no
// behaviour change for any caller that never opts in.
func TestGameDeadline_UnarmedIsInert(t *testing.T) {
	gs := deadlineTestState()
	if _, ok := GameDeadline(gs); ok {
		t.Fatal("a fresh state reports a deadline")
	}
	if GameDeadlineExceeded(gs) {
		t.Fatal("unarmed state reports its deadline exceeded")
	}
	if CheckGameDeadline(gs, "test") {
		t.Fatal("unarmed state tripped the guard")
	}
	if gs.Flags["ended"] == 1 {
		t.Fatal("unarmed state ended the game")
	}
}

// TestGameDeadline_ArmedButNotPassedIsInert — time remaining must not trip.
func TestGameDeadline_ArmedButNotPassedIsInert(t *testing.T) {
	gs := deadlineTestState()
	SetGameDeadline(gs, time.Now().Add(time.Hour))
	if !hasDeadline(gs) {
		t.Fatal("deadline did not arm")
	}
	if GameDeadlineExceeded(gs) {
		t.Fatal("future deadline reported as exceeded")
	}
	if CheckGameDeadline(gs, "test") {
		t.Fatal("future deadline tripped the guard")
	}
	if gs.Flags["ended"] == 1 {
		t.Fatal("future deadline ended the game")
	}
}

// TestGameDeadline_PassedEndsGameAsDraw — the whole point. CR §104.4b: a
// mandatory loop no player can break is a draw.
func TestGameDeadline_PassedEndsGameAsDraw(t *testing.T) {
	gs := deadlineTestState()
	SetGameDeadline(gs, time.Now().Add(-time.Second))
	if !GameDeadlineExceeded(gs) {
		t.Fatal("past deadline not reported as exceeded")
	}
	if !CheckGameDeadline(gs, "stack_drain") {
		t.Fatal("past deadline did not trip the guard")
	}
	if gs.Flags["ended"] != 1 {
		t.Error("game did not end")
	}
	if gs.Flags["game_draw"] != 1 {
		t.Error("game did not end as a DRAW")
	}
	for i, s := range gs.Seats {
		if s.Won {
			t.Errorf("seat %d was marked a winner on a loop draw", i)
		}
	}
}

// TestGameDeadline_AlreadyEndedIsInert — once a game is over the guard
// must not re-fire and re-log on every subsequent loop iteration.
//
// r161 corrected this test's ASSERTION, not its intent. It used to
// require CheckGameDeadline to return false once ended, conflating two
// different things: whether the guard fires (it must not, twice) and
// what the call site is told (it must still bail, or nothing unwinds
// the in-flight turn). Production proved the difference — a game ran
// 1094s past its deadline with the counter reading exactly 1. The
// no-re-fire intent written above is unchanged and is what this test
// now checks; TestR161_ExpiredDeadlineStillReportsAfterTheGameEnded
// covers the return value.
func TestGameDeadline_AlreadyEndedIsInert(t *testing.T) {
	ResetLoopGuardCountsForTest()
	t.Cleanup(ResetLoopGuardCountsForTest)

	gs := deadlineTestState()
	SetGameDeadline(gs, time.Now().Add(-time.Second))
	gs.Flags["ended"] = 1
	before := len(gs.EventLog)
	CheckGameDeadline(gs, "test")
	if n := LoopGuardCounts()["game_deadline"]; n != 0 {
		t.Fatalf("guard re-fired on an already-ended game: counted %d", n)
	}
	if len(gs.EventLog) != before {
		t.Fatalf("guard re-logged on an already-ended game: %d new events",
			len(gs.EventLog)-before)
	}
}

// TestGameDeadline_Clear disarms.
func TestGameDeadline_Clear(t *testing.T) {
	gs := deadlineTestState()
	SetGameDeadline(gs, time.Now().Add(-time.Second))
	ClearGameDeadline(gs)
	if GameDeadlineExceeded(gs) {
		t.Fatal("cleared deadline still reports exceeded")
	}
	if CheckGameDeadline(gs, "test") {
		t.Fatal("cleared deadline still trips")
	}
}

// TestGameDeadline_NilSafe — these are called from hot loops; a nil state
// must never panic.
func TestGameDeadline_NilSafe(t *testing.T) {
	SetGameDeadline(nil, time.Now())
	ClearGameDeadline(nil)
	if _, ok := GameDeadline(nil); ok {
		t.Error("nil state reported a deadline")
	}
	if GameDeadlineExceeded(nil) {
		t.Error("nil state reported exceeded")
	}
	if CheckGameDeadline(nil, "test") {
		t.Error("nil state tripped")
	}
}

// TestGameDeadline_SurvivesRolloutClone is the reason the deadline lives
// in Flags rather than as a GameState field: CloneForRollout builds its
// result from an explicit field list, so a new struct field would be
// dropped from every clone without anything failing to compile.
func TestGameDeadline_SurvivesRolloutClone(t *testing.T) {
	gs := deadlineTestState()
	at := time.Now().Add(time.Hour)
	SetGameDeadline(gs, at)

	clone := gs.CloneForRollout(nil)
	if clone == nil {
		t.Fatal("CloneForRollout returned nil")
	}
	got, ok := GameDeadline(clone)
	if !ok {
		t.Fatal("clone lost the deadline — is it stored outside Flags?")
	}
	if got.Unix() != at.Unix() {
		t.Errorf("clone deadline = %v, want %v", got.Unix(), at.Unix())
	}
}

// TestGameDeadline_LogsTheSiteThatNoticed — the diagnostic the 2026-10-08
// stall never produced. Without the site name a future hang tells us only
// that something spun, not where.
func TestGameDeadline_LogsTheSiteThatNoticed(t *testing.T) {
	gs := deadlineTestState()
	gs.EventPolicy = EventLogFull
	SetGameDeadline(gs, time.Now().Add(-time.Second))

	if !CheckGameDeadline(gs, "trigger_drain") {
		t.Fatal("guard did not trip")
	}
	var found bool
	for _, ev := range gs.EventLog {
		if ev.Kind != EventLoopGuardFired {
			continue
		}
		if ev.Details["guard"] != "game_deadline" {
			continue
		}
		found = true
		if ev.Details["site"] != "trigger_drain" {
			t.Errorf("site = %v, want trigger_drain", ev.Details["site"])
		}
		if ev.Details["turn"] != 7 {
			t.Errorf("turn = %v, want 7", ev.Details["turn"])
		}
	}
	if !found {
		t.Fatal("no loop_guard_fired event naming game_deadline")
	}
}

func hasDeadline(gs *GameState) bool {
	_, ok := GameDeadline(gs)
	return ok
}
