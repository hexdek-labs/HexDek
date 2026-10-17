package gameengine

import (
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r166 — the wall-clock backstop inside resolveCreateToken's per-token
// loop. See the comment at the call site for the production evidence.
//
// The load-bearing property is NOT "the guard fires". r161 shipped a
// guard that fired and did nothing. It is "a game whose deadline has
// passed stops minting tokens", and the number asserted on is the one
// production actually prints: LoopGuardCounts()["game_deadline"].

const r166Count = 64

// r166Game builds a game with one source permanent on seat 0, under the
// EventLogNone policy the grinder really runs, and resets the process-
// wide guard counters around the test.
func r166Game(t *testing.T) (*GameState, *Permanent) {
	t.Helper()
	ResetLoopGuardCountsForTest()
	t.Cleanup(ResetLoopGuardCountsForTest)
	gs := newFixtureGame(t)
	gs.EventPolicy = EventLogNone
	src := addBattlefield(gs, 0, "Krenko, Mob Boss", 3, 3, "creature")
	return gs, src
}

func r166Effect(n int) *gameast.CreateToken {
	return &gameast.CreateToken{
		Count: gameast.NumberOrRef{IsInt: true, Int: n},
		PT:    &[2]int{1, 1},
		Types: []string{"creature"},
	}
}

// r166Minted returns how many permanents resolveCreateToken added.
func r166Minted(gs *GameState, before int) int {
	return len(gs.Seats[0].Battlefield) - before
}

func TestR166_ExpiredDeadlineStopsTokenCreation(t *testing.T) {
	gs, src := r166Game(t)
	SetGameDeadline(gs, time.Now().Add(-time.Second))
	before := len(gs.Seats[0].Battlefield)

	resolveCreateToken(gs, src, r166Effect(r166Count))

	if got := r166Minted(gs, before); got >= r166Count {
		t.Fatalf("expired deadline minted %d of %d tokens — the loop did not bail", got, r166Count)
	}
	// The number production prints.
	if n := LoopGuardCounts()["game_deadline"]; n != 1 {
		t.Fatalf("game_deadline counter = %d, want exactly 1 (once per game)", n)
	}
	if gs.Flags["ended"] != 1 {
		t.Fatalf("breaching the deadline must end the game; ended=%d", gs.Flags["ended"])
	}
}

func TestR166_UnarmedGameMintsEveryToken(t *testing.T) {
	// The opt-in property. Tests, paritycheck and the correctness
	// harness never arm a deadline, so they must be bit-for-bit
	// unaffected by this round.
	gs, src := r166Game(t)
	before := len(gs.Seats[0].Battlefield)

	resolveCreateToken(gs, src, r166Effect(r166Count))

	if got := r166Minted(gs, before); got != r166Count {
		t.Fatalf("unarmed game minted %d tokens, want all %d", got, r166Count)
	}
	if n := LoopGuardCounts()["game_deadline"]; n != 0 {
		t.Fatalf("unarmed game fired the deadline guard %d times, want 0", n)
	}
	if gs.Flags["ended"] == 1 {
		t.Fatalf("unarmed game must not be ended by token creation")
	}
}

func TestR166_UnexpiredDeadlineMintsEveryToken(t *testing.T) {
	gs, src := r166Game(t)
	SetGameDeadline(gs, time.Now().Add(time.Hour))
	before := len(gs.Seats[0].Battlefield)

	resolveCreateToken(gs, src, r166Effect(r166Count))

	if got := r166Minted(gs, before); got != r166Count {
		t.Fatalf("unexpired deadline minted %d tokens, want all %d", got, r166Count)
	}
	if n := LoopGuardCounts()["game_deadline"]; n != 0 {
		t.Fatalf("unexpired deadline fired the guard %d times, want 0", n)
	}
}

func TestR166_AlreadyEndedGameStillStopsMinting(t *testing.T) {
	// r161's shape: an already-ended game must still be stopped here.
	// CheckGameDeadline returns true without re-ending or re-counting,
	// so the loop must bail on that true and NOT keep minting into a
	// game that is already recorded as a draw.
	gs, src := r166Game(t)
	SetGameDeadline(gs, time.Now().Add(-time.Second))
	EndGameAsLoopDraw(gs, "test_precondition")
	before := len(gs.Seats[0].Battlefield)

	resolveCreateToken(gs, src, r166Effect(r166Count))

	if got := r166Minted(gs, before); got >= r166Count {
		t.Fatalf("already-ended game minted %d of %d tokens", got, r166Count)
	}
	if n := LoopGuardCounts()["game_deadline"]; n != 0 {
		t.Fatalf("an already-ended game must not re-count the guard; got %d", n)
	}
}

func TestR166_SingleTokenIsNotSpecialCased(t *testing.T) {
	// The overwhelmingly common production shape is count==1. It must
	// behave exactly like any other count: minted when live, refused
	// when the clock has gone.
	gs, src := r166Game(t)
	before := len(gs.Seats[0].Battlefield)
	resolveCreateToken(gs, src, r166Effect(1))
	if got := r166Minted(gs, before); got != 1 {
		t.Fatalf("live game minted %d tokens for count=1, want 1", got)
	}

	gs2, src2 := r166Game(t)
	SetGameDeadline(gs2, time.Now().Add(-time.Second))
	before2 := len(gs2.Seats[0].Battlefield)
	resolveCreateToken(gs2, src2, r166Effect(1))
	if got := r166Minted(gs2, before2); got != 0 {
		t.Fatalf("expired game minted %d tokens for count=1, want 0", got)
	}
}
