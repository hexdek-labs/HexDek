package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/achievements"
)

func TestOwnerFromDeckPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"data/decks/wiedeman/burn.txt", "wiedeman"},
		{"/abs/data/decks/alice/control.txt", "alice"},
		{"alice/control.txt", "alice"},
		{"", ""},
	}
	for _, c := range cases {
		if got := ownerFromDeckPath(c.in); got != c.want {
			t.Errorf("ownerFromDeckPath(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestAwardAchievementsResolvesOwnerByCommanderIdx(t *testing.T) {
	tr, err := achievements.NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	// owners[1] = alice (the winning seat references CommanderIdx=1).
	owners := []string{"bob", "alice", "carol", "dan"}
	o := GameOutcome{
		Turns: 7,
		PostGameStats: []SeatStats{
			{CommanderIdx: 0, Won: false, FinalLife: 0},
			{CommanderIdx: 1, Won: true, FinalLife: 24},
			{CommanderIdx: 2, Won: false, FinalLife: 0},
			{CommanderIdx: 3, Won: false, FinalLife: 0},
		},
	}
	awardAchievements(tr, o, owners)

	snap := tr.Snapshot("alice")
	if snap.TotalGames != 1 || snap.TotalWins != 1 {
		t.Fatalf("alice expected 1g/1w, got games=%d wins=%d", snap.TotalGames, snap.TotalWins)
	}
	hasFirstBlood := false
	for _, b := range snap.Badges {
		if b.ID == "first_blood" {
			hasFirstBlood = true
		}
	}
	if !hasFirstBlood {
		t.Errorf("expected first_blood after first win via tournament path, got %+v", snap.Badges)
	}
}

func TestAwardAchievementsNilTrackerIsNoOp(t *testing.T) {
	// Explicit doesn't-panic assertion via recover() — Phase 2B audit.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("awardAchievements(nil tracker, ...) must early-return; got panic %v", r)
		}
	}()
	awardAchievements(nil, GameOutcome{}, nil)
	awardAchievements(nil, GameOutcome{Turns: 5, PostGameStats: []SeatStats{{Won: true}}}, []string{"alice"})
}

func TestAwardAchievementsSkipsOutOfRangeIdx(t *testing.T) {
	tr, _ := achievements.NewTracker(t.TempDir())
	owners := []string{"alice"}
	o := GameOutcome{
		Turns: 5,
		PostGameStats: []SeatStats{
			{CommanderIdx: 0, Won: true, FinalLife: 30},
			{CommanderIdx: 99, Won: false, FinalLife: 0},
			{CommanderIdx: -1, Won: false, FinalLife: 0},
		},
	}
	awardAchievements(tr, o, owners)
	if got := tr.Snapshot("alice").TotalGames; got != 1 {
		t.Errorf("alice should have 1 game, got %d", got)
	}
}
