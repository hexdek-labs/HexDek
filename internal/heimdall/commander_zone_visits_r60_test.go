package heimdall

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60: ExtractCommanderZoneVisits surfaces per-commander zone activity
// at game end so post-game analytics can distinguish answer-magnet
// commanders (high visit counts) from sticky-board commanders (visits
// = 1, sitting on the battlefield).

func newCmdGS(t *testing.T, seats int) *gameengine.GameState {
	t.Helper()
	return gameengine.NewGameState(seats, rand.New(rand.NewSource(1)), nil)
}

func addCmd(gs *gameengine.GameState, seat int, name string) *gameengine.Card {
	c := &gameengine.Card{Name: name, Owner: seat}
	gs.Seats[seat].CommanderNames = append(gs.Seats[seat].CommanderNames, name)
	if gs.Seats[seat].CommanderCastCounts == nil {
		gs.Seats[seat].CommanderCastCounts = map[string]int{}
	}
	return c
}

// Commander sitting in the command zone all game (never cast):
// CastCount=0, InZoneAtEnd=true, Visits=1.
func TestExtractCommanderZoneVisits_NeverCastStaysInZone(t *testing.T) {
	gs := newCmdGS(t, 4)
	c := addCmd(gs, 0, "Krenko, Mob Boss")
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, c)

	visits := ExtractCommanderZoneVisits(gs)
	if len(visits) != 1 {
		t.Fatalf("want 1 visit record, got %d", len(visits))
	}
	v := visits[0]
	if v.Seat != 0 || v.CommanderName != "Krenko, Mob Boss" {
		t.Errorf("unexpected (seat=%d, name=%q)", v.Seat, v.CommanderName)
	}
	if v.CastCount != 0 || !v.InZoneAtEnd || v.Visits != 1 {
		t.Errorf("want cast=0 in_zone=true visits=1; got cast=%d in_zone=%v visits=%d",
			v.CastCount, v.InZoneAtEnd, v.Visits)
	}
}

// Sticky-board commander: cast once, currently on the battlefield
// (not in command zone). CastCount=1, InZoneAtEnd=false, Visits=1.
func TestExtractCommanderZoneVisits_CastOnceOnBattlefield(t *testing.T) {
	gs := newCmdGS(t, 4)
	addCmd(gs, 1, "Atraxa, Praetors' Voice")
	gs.Seats[1].CommanderCastCounts["Atraxa, Praetors' Voice"] = 1
	// Not in command zone.

	visits := ExtractCommanderZoneVisits(gs)
	if len(visits) != 1 {
		t.Fatalf("want 1, got %d", len(visits))
	}
	v := visits[0]
	if v.CastCount != 1 || v.InZoneAtEnd || v.Visits != 1 {
		t.Errorf("want cast=1 in_zone=false visits=1; got cast=%d in_zone=%v visits=%d",
			v.CastCount, v.InZoneAtEnd, v.Visits)
	}
}

// Answer-magnet commander: cast 3 times, currently back in the
// command zone (died+redirected after the 3rd cast).
// CastCount=3, InZoneAtEnd=true, Visits=4 (initial + 3 redirects).
func TestExtractCommanderZoneVisits_AnswerMagnet(t *testing.T) {
	gs := newCmdGS(t, 4)
	c := addCmd(gs, 2, "Edgar Markov")
	gs.Seats[2].CommanderCastCounts["Edgar Markov"] = 3
	gs.Seats[2].CommandZone = append(gs.Seats[2].CommandZone, c)

	visits := ExtractCommanderZoneVisits(gs)
	if len(visits) != 1 {
		t.Fatalf("want 1, got %d", len(visits))
	}
	v := visits[0]
	if v.CastCount != 3 || !v.InZoneAtEnd || v.Visits != 4 {
		t.Errorf("want cast=3 in_zone=true visits=4; got cast=%d in_zone=%v visits=%d",
			v.CastCount, v.InZoneAtEnd, v.Visits)
	}
}

// Cast twice, did NOT come back after the 2nd cast (sitting in
// graveyard, exile, or elsewhere). 1 redirect happened, plus initial.
// CastCount=2, InZoneAtEnd=false, Visits=2.
func TestExtractCommanderZoneVisits_CastTwiceNotInZone(t *testing.T) {
	gs := newCmdGS(t, 4)
	addCmd(gs, 0, "Yuriko, the Tiger's Shadow")
	gs.Seats[0].CommanderCastCounts["Yuriko, the Tiger's Shadow"] = 2

	visits := ExtractCommanderZoneVisits(gs)
	v := visits[0]
	if v.CastCount != 2 || v.InZoneAtEnd || v.Visits != 2 {
		t.Errorf("want cast=2 in_zone=false visits=2; got cast=%d in_zone=%v visits=%d",
			v.CastCount, v.InZoneAtEnd, v.Visits)
	}
}

// Partner pair: two commanders on the same seat, surfaced
// independently.
func TestExtractCommanderZoneVisits_PartnerPair(t *testing.T) {
	gs := newCmdGS(t, 4)
	a := addCmd(gs, 3, "Tymna the Weaver")
	addCmd(gs, 3, "Thrasios, Triton Hero")
	gs.Seats[3].CommanderCastCounts["Tymna the Weaver"] = 2
	gs.Seats[3].CommanderCastCounts["Thrasios, Triton Hero"] = 1
	// Tymna currently sitting in command zone; Thrasios is not.
	gs.Seats[3].CommandZone = append(gs.Seats[3].CommandZone, a)

	visits := ExtractCommanderZoneVisits(gs)
	if len(visits) != 2 {
		t.Fatalf("want 2 records for partner pair, got %d", len(visits))
	}
	byName := map[string]CommanderZoneVisit{}
	for _, v := range visits {
		byName[v.CommanderName] = v
	}
	if v := byName["Tymna the Weaver"]; v.CastCount != 2 || !v.InZoneAtEnd || v.Visits != 3 {
		t.Errorf("Tymna: want cast=2 in_zone=true visits=3; got %+v", v)
	}
	if v := byName["Thrasios, Triton Hero"]; v.CastCount != 1 || v.InZoneAtEnd || v.Visits != 1 {
		t.Errorf("Thrasios: want cast=1 in_zone=false visits=1; got %+v", v)
	}
}

// Multi-seat: every seat with a commander contributes a record,
// seats without commanders contribute nothing.
func TestExtractCommanderZoneVisits_MultiSeatOrdering(t *testing.T) {
	gs := newCmdGS(t, 4)
	c0 := addCmd(gs, 0, "Krenko, Mob Boss")
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, c0)
	// Seat 1: no commander assigned (CommanderNames empty).
	c2 := addCmd(gs, 2, "Edgar Markov")
	gs.Seats[2].CommanderCastCounts["Edgar Markov"] = 2
	gs.Seats[2].CommandZone = append(gs.Seats[2].CommandZone, c2)
	addCmd(gs, 3, "Atraxa, Praetors' Voice")
	gs.Seats[3].CommanderCastCounts["Atraxa, Praetors' Voice"] = 1

	visits := ExtractCommanderZoneVisits(gs)
	if len(visits) != 3 {
		t.Fatalf("want 3 records (seats 0/2/3), got %d", len(visits))
	}
	seats := []int{}
	for _, v := range visits {
		seats = append(seats, v.Seat)
	}
	sort.Ints(seats)
	want := []int{0, 2, 3}
	for i, s := range seats {
		if s != want[i] {
			t.Errorf("seat ordering: got %v, want %v", seats, want)
			break
		}
	}
}

// Defensive: nil game state returns nil, doesn't panic.
func TestExtractCommanderZoneVisits_NilGameState(t *testing.T) {
	if got := ExtractCommanderZoneVisits(nil); got != nil {
		t.Errorf("nil gs should return nil, got %v", got)
	}
}

// Defensive: a seat with CommanderNames populated but a nil
// CommanderCastCounts map reports cast=0 without panicking.
func TestExtractCommanderZoneVisits_NilCastCountsMap(t *testing.T) {
	gs := newCmdGS(t, 2)
	c := &gameengine.Card{Name: "Niv-Mizzet Reborn", Owner: 0}
	gs.Seats[0].CommanderNames = []string{"Niv-Mizzet Reborn"}
	gs.Seats[0].CommanderCastCounts = nil
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, c)

	visits := ExtractCommanderZoneVisits(gs)
	if len(visits) != 1 || visits[0].CastCount != 0 || !visits[0].InZoneAtEnd || visits[0].Visits != 1 {
		t.Errorf("nil map fallback: got %+v", visits)
	}
}
