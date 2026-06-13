package tournament

// Regression for the fishtank-radar post-deploy r63 legality cluster
// (grinder seed 1781373913569584800, sampled game 114, turn 35, seat 1):
//
//   legality 117.1a — "cast:Mox Amber: sorcery-speed spell cast by
//                       non-active seat (active=2)"
//   legality 605.1a — "activate:Mox Amber#-1: ability resolved inline
//                       claiming mana ability but produced no mana in its
//                       window — opponents were denied responses"
//
// Both stem from ONE root cause in the turn runner. When a seat is
// eliminated mid-turn (CR §800.4a), HandleSeatElimination marks it Lost +
// LeftGame and — per the §800.4h MVP — advances gs.Active to the next
// living seat. But takeTurnImpl captured the active seat ONCE at entry and
// runMainPhase's cast / activation loops never re-checked whether that
// seat was still alive, and elimination leaves the dead seat's hand
// pointers intact (multiplayer.go nils nothing, only ceases InstanceIDs).
// So the loop kept casting from the DEAD seat's untouched hand while
// gs.Active already pointed elsewhere — the off-turn cast (117.1a) — and
// tapped its now-wiped board (no legendaries) for 0 mana (605.1a).
//
// Fix: seatCanAct() guards in runMainPhase / runInstantPriority — a
// departed player performs no further actions; the turn just runs to
// cleanup.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// eagerCastHat always casts the first castable card it is offered, so the
// cast loop is driven deterministically regardless of GreedyHat's value
// heuristics. It embeds GreedyHat for the rest of the Hat interface.
type eagerCastHat struct{ *hat.GreedyHat }

func (eagerCastHat) ChooseCastFromHand(_ *gameengine.GameState, _ int, castable []*gameengine.Card) *gameengine.Card {
	if len(castable) > 0 {
		return castable[0]
	}
	return nil
}

// freeArtifact builds a {0}-cost noncreature artifact (Mox-shaped): a
// legitimate free spell (hasNoManaCost==false for zero-CMC artifacts) so
// it stays castable even after elimination zeroes the seat's mana pool.
func freeArtifact(name string, owner int, iid string) *gameengine.Card {
	return &gameengine.Card{
		Name:       name,
		Types:      []string{"artifact"},
		TypeLine:   "artifact",
		CMC:        0,
		Owner:      owner,
		InstanceID: iid,
	}
}

// TestRunMainPhase_MidLoopEliminationStopsCasting is the headline pin.
// Seat 1 is the active player at 0 life with two free artifacts in hand.
// The first cast resolves, then the post-cast SBA eliminates seat 1
// (§704.5a) and advances gs.Active — the second card must NOT be cast,
// and no §117.1a off-turn-cast violation may be recorded.
func TestRunMainPhase_MidLoopEliminationStopsCasting(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(114)), nil)
	gameengine.SetupCommanderGame(gs, []*gameengine.CommanderDeck{{}, {}, {}, {}})
	for i := range gs.Seats {
		gs.Seats[i].Hat = eagerCastHat{&hat.GreedyHat{}}
		gs.Seats[i].Life = 40
	}
	gs.Legality = gameengine.NewLegalityValidator(114)
	gs.Active = 1
	gs.Turn = 35
	gs.Phase = "main"

	// Seat 1 is about to die: 0 life, but not yet SBA'd (Lost==false), so
	// it legally casts the first artifact this main phase.
	seat := gs.Seats[1]
	seat.Life = 0
	moxA := freeArtifact("Mox A", 1, "iid-mox-a")
	moxB := freeArtifact("Mox B", 1, "iid-mox-b")
	seat.Hand = []*gameengine.Card{moxA, moxB}

	runMainPhase(gs, 1, true)

	// The second artifact must remain in hand — the dead seat may not cast it.
	stillHasB := false
	for _, c := range seat.Hand {
		if c == moxB {
			stillHasB = true
		}
		if c == moxA {
			t.Fatalf("Mox A should have been cast (and then wiped on elimination), but it is still in hand")
		}
	}
	if !stillHasB {
		t.Fatalf("Mox B was cast from an eliminated seat's hand — dead seats must take no further actions (CR §800.4h)")
	}

	if !seat.Lost {
		t.Fatalf("seat 1 should be Lost after its post-cast SBA (life 0, §704.5a)")
	}

	for _, v := range gs.Legality.Violations {
		if v.Rule == "117.1a" {
			t.Fatalf("recorded a §117.1a off-turn-cast violation (seat=%d active-at-announce mismatch): %s — the dead-seat guard should have prevented the cast", v.Seat, v.Detail)
		}
	}
}

// TestRunMainPhase_DeadSeatCastsNothing pins the entry guard: a seat that
// is already Lost + LeftGame with gs.Active advanced past it (the steady
// state after HandleSeatElimination) casts nothing at all.
func TestRunMainPhase_DeadSeatCastsNothing(t *testing.T) {
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(7)), nil)
	gameengine.SetupCommanderGame(gs, []*gameengine.CommanderDeck{{}, {}, {}, {}})
	for i := range gs.Seats {
		gs.Seats[i].Hat = eagerCastHat{&hat.GreedyHat{}}
		gs.Seats[i].Life = 40
	}
	gs.Legality = gameengine.NewLegalityValidator(7)
	gs.Phase = "main"

	seat := gs.Seats[1]
	mox := freeArtifact("Mox C", 1, "iid-mox-c")
	seat.Hand = []*gameengine.Card{mox}
	seat.Lost = true
	seat.LeftGame = true
	gs.Active = 2 // already advanced past the eliminated seat 1

	runMainPhase(gs, 1, true)

	if len(seat.Hand) != 1 || seat.Hand[0] != mox {
		t.Fatalf("an eliminated seat cast a spell from hand; hand=%v", seat.Hand)
	}
	if len(gs.Legality.Violations) != 0 {
		t.Fatalf("eliminated seat produced %d legality violation(s); expected none", len(gs.Legality.Violations))
	}
}
