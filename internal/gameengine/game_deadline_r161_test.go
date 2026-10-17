package gameengine

import (
	"math/rand"
	"testing"
	"time"
)

// r161Game builds a four-seat game with an armed, already-expired
// wall-clock deadline. `over` is how long ago the deadline passed.
func r161Game(t *testing.T, over time.Duration) *GameState {
	t.Helper()
	gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
	SetGameDeadline(gs, time.Now().Add(-over))
	return gs
}

// The defect, pinned as its own test: before r161 this returned false
// once the game was marked ended, so every call site after the first
// trip believed there was no deadline problem and kept spinning. This
// is the shape that let a production game run 1094 seconds past a
// 600-second cap while the guard counter read exactly 1.
func TestR161_ExpiredDeadlineStillReportsAfterTheGameEnded(t *testing.T) {
	gs := r161Game(t, 30*time.Second)

	if !CheckGameDeadline(gs, "first_site") {
		t.Fatalf("first trip must report the breach")
	}
	if gs.Flags["ended"] != 1 {
		t.Fatalf("first trip must end the game, ended=%d", gs.Flags["ended"])
	}

	// The in-flight turn keeps running and reaches another check site.
	if !CheckGameDeadline(gs, "second_site") {
		t.Fatalf("an expired deadline must keep reporting so callers unwind; " +
			"returning false here is the r161 defect")
	}
}

// ...but it must end the game exactly once. The guard counter is the
// production-visible number (r160), so assert on that rather than on a
// side effect nobody can see.
func TestR161_TheGameIsEndedAndCountedExactlyOnce(t *testing.T) {
	ResetLoopGuardCountsForTest()
	t.Cleanup(ResetLoopGuardCountsForTest)

	gs := r161Game(t, 30*time.Second)
	for i := 0; i < 5; i++ {
		if !CheckGameDeadline(gs, "site") {
			t.Fatalf("call %d: expected the breach to be reported", i)
		}
	}
	if got := LoopGuardCounts()["game_deadline"]; got != 1 {
		t.Fatalf("game_deadline counted %d times, want exactly 1 "+
			"(production read :1 and :2 — one per long game)", got)
	}
}

// An ended game with NO deadline armed must stay false. Tests, parity
// and the deterministic harnesses never arm one and must be untouched.
func TestR161_UnarmedGameIsUnaffectedByTheEndedFlag(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
	gs.Flags["ended"] = 1
	if CheckGameDeadline(gs, "site") {
		t.Fatalf("no deadline armed: must report nothing")
	}
}

// An armed deadline that has NOT passed stays false even once ended —
// the new early return must key on the breach, not on the flag.
func TestR161_UnexpiredDeadlineReportsNothingWhenEnded(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
	SetGameDeadline(gs, time.Now().Add(time.Hour))
	gs.Flags["ended"] = 1
	if CheckGameDeadline(gs, "site") {
		t.Fatalf("deadline in the future: must report nothing")
	}
}

func TestR161_NilGameStateIsSafe(t *testing.T) {
	if CheckGameDeadline(nil, "site") {
		t.Fatalf("nil GameState must report nothing")
	}
}
