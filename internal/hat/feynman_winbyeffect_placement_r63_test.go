package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// feynman_winbyeffect_placement_r63_test.go — firespot r63: a §104.2c
// "you win the game" end (Approach, Thassa's Oracle, Maze's End, …) must
// produce a COMPLETE final standing — winner + every opponent marked Lost
// (CR §104.2c) — so a 4-player game always has a full 1st→4th placement and
// the feynman/seat-outcome post-game checks see no "zombie outcome" /
// "N of 4 lost (expected 3)" mismatch. Pre-fix: resolveWinGame set only the
// winner's Won flag; CheckEnd left the 3 opponents neither-Won-nor-Lost.
func TestWinByEffect_CompletePlacement_NoFeynmanGameEnd(t *testing.T) {
	gs := gameengine.NewGameState(4, nil, nil)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 12
	gs.Seats[2].Life = 8
	gs.Seats[3].Life = 30 // highest life among losers, but did NOT win

	// §104.2c — seat 0 wins via an effect (e.g. Approach of the Second Sun).
	gs.Seats[0].Won = true
	if !gs.CheckEnd() {
		t.Fatal("CheckEnd should report the game ended on a §104.2c win")
	}

	// (a) Complete outcome: winner Won, every opponent Lost.
	if !gs.Seats[0].Won || gs.Seats[0].Lost {
		t.Fatalf("seat 0 must be the sole winner (Won=%v Lost=%v)", gs.Seats[0].Won, gs.Seats[0].Lost)
	}
	for i := 1; i < 4; i++ {
		if !gs.Seats[i].Lost {
			t.Fatalf("CR §104.2c: opponent seat %d must be marked Lost after a win-by-effect end", i)
		}
		if gs.Seats[i].Won {
			t.Fatalf("opponent seat %d must not be Won", i)
		}
	}

	// (a) Full deterministic 1st→4th standing, winner = 1st, all distinct.
	pl := gs.FinalPlacements()
	if len(pl) != 4 {
		t.Fatalf("FinalPlacements len = %d, want 4", len(pl))
	}
	if pl[0] != 1 {
		t.Fatalf("winner must place 1st; got %d", pl[0])
	}
	seen := map[int]bool{}
	for i, p := range pl {
		if p < 1 || p > 4 {
			t.Fatalf("seat %d placement %d out of range 1..4", i, p)
		}
		if seen[p] {
			t.Fatalf("duplicate placement %d — standing is not a complete 1..4 permutation: %v", p, pl)
		}
		seen[p] = true
	}

	// (b) Feynman's post-game oracle sees no game_end mismatch.
	res := CheckGame(gs)
	for _, v := range res.Violations {
		if v.Name == "game_end" {
			t.Fatalf("feynman game_end violation after a complete win-by-effect end: %s", v.Message)
		}
	}
}

// Last-seat-standing (§104.2a) is unchanged: opponents already Lost, winner
// 1st, complete placement — guards against the §104.2c marking regressing
// the normal path.
func TestLastSeatStanding_CompletePlacement(t *testing.T) {
	gs := gameengine.NewGameState(4, nil, nil)
	gs.Seats[0].Life = 15
	gs.Seats[1].Life = 0
	gs.Seats[1].Lost = true
	gs.Seats[1].LostOrder = 1
	gs.Seats[2].Life = 0
	gs.Seats[2].Lost = true
	gs.Seats[2].LostOrder = 2
	gs.Seats[3].Life = 0
	gs.Seats[3].Lost = true
	gs.Seats[3].LostOrder = 3

	if !gs.CheckEnd() {
		t.Fatal("CheckEnd should end the game with one seat standing")
	}
	if !gs.Seats[0].Won {
		t.Fatal("last seat standing must be Won")
	}
	pl := gs.FinalPlacements()
	// winner 1st; later-eliminated seats finish better than earlier ones.
	if pl[0] != 1 {
		t.Fatalf("winner placement = %d, want 1", pl[0])
	}
	if !(pl[3] < pl[2] && pl[2] < pl[1]) {
		t.Fatalf("later-eliminated seats must place better: got seat1=%d seat2=%d seat3=%d", pl[1], pl[2], pl[3])
	}
	res := CheckGame(gs)
	for _, v := range res.Violations {
		if v.Name == "game_end" {
			t.Fatalf("feynman game_end violation on clean last-seat-standing: %s", v.Message)
		}
	}
}
