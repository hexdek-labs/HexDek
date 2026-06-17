package per_card

// the_reaper_steal_slice_r63_test.go — r63 CardIdentity cross-seat aliasing
// root-producer fix. The Reaper King's reanimate-and-steal used MoveCard to
// drop the dying creature on its OWNER's battlefield slice, then flipped
// Permanent.Controller to the thief — leaving the *Permanent in the owner's
// slice with Controller=thief (a slice/Controller mismatch). A later control
// op then aliased the *Permanent onto two battlefields (loki seed 99 game
// 222: "The Scarab God appears in both seat 0 and seat 1 battlefield").
//
// Fix: the steal performs a real control change — the reanimated *Permanent
// is re-homed to the thief's battlefield slice, so slice-seat == Controller.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestTheReaperDies_StolenPermanentLandsInThiefSlice(t *testing.T) {
	Reset()

	gs := gameengine.NewGameState(3, nil, nil)

	// Seat 0 controls The Reaper.
	reaperCard := &gameengine.Card{Name: "The Reaper, King No More", Owner: 0, Types: []string{"creature", "legendary"}}
	reaper := &gameengine.Permanent{Card: reaperCard, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, reaper)

	// Seat 1's creature died (with a -1/-1 counter) and is in seat 1's graveyard.
	victim := &gameengine.Card{Name: "Stolen Beast", Owner: 1, Types: []string{"creature"}}
	gameengine.MintOGInstanceID(gs, victim)
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, victim)

	dyingPerm := &gameengine.Permanent{Card: victim, Controller: 1, Owner: 1, Counters: map[string]int{"-1/-1": 1}}
	ctx := map[string]interface{}{"perm": dyingPerm, "card": victim, "controller_seat": 1}

	theReaperDies(gs, reaper, ctx)

	// The stolen permanent must be on the THIEF's (seat 0) slice with
	// Controller==0 — not stranded in the owner's (seat 1) slice.
	var stolen *gameengine.Permanent
	sliceSeat := -1
	for si, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card == victim {
				if stolen != nil {
					t.Fatalf("stolen *Card appears on multiple battlefields (seat %d and seat %d) — alias", sliceSeat, si)
				}
				stolen = p
				sliceSeat = si
			}
		}
	}
	if stolen == nil {
		t.Fatal("steal did not place the creature on any battlefield")
	}
	if sliceSeat != 0 {
		t.Errorf("stolen permanent in seat %d's slice, want thief seat 0", sliceSeat)
	}
	if stolen.Controller != 0 {
		t.Errorf("stolen permanent Controller=%d, want 0 (slice/Controller must agree)", stolen.Controller)
	}

	// And the whole state must be CardIdentity-clean.
	for _, v := range gameengine.RunAllInvariants(gs) {
		if v.Name == "CardIdentity" {
			t.Fatalf("CardIdentity dup after Reaper steal: %s", v.Message)
		}
	}
}
