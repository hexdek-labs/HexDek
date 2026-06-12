package gameengine

// r63 — production zone-disappearance root cause (grinder server.log
// feynman zone_accounting: seats 5-20 cards LIGHT, e.g. "seat 0 owns 82,
// expected ~100, diff=-18").
//
// HandleSeatElimination's battlefield sweep removes every permanent
// matching `p.Controller == seatIdx || p.Owner == seatIdx`. For
// permanents the leaver merely CONTROLLED but another player OWNS
// (Gilded Drake trades, reanimated opponents' creatures — the Geth
// decks in the warned games), the doc comment says "our MVP simply
// exiles it by removing from play" — but nothing ever appended the card
// to ANY zone. The card object vanished from all zone accounting:
//   - the owner's zone_accounting count reads light (the live signal);
//   - realCardsLeaving counted the OTHER-owned card into the LEAVER's
//     cards_left_game, deflating the leaver's expected count too;
//   - the InstanceID is (correctly) NOT ceased — under the strict
//     census this is a disappearance: expected present, found nowhere.
//
// CR §800.4a/c: the control effect ends; an object that would remain
// under the departed player's control is EXILED — it never ceases to
// exist unless the OWNER left. Fix: route other-owned permanents to
// their owner's exile zone and exclude them from realCardsLeaving.

import (
	"testing"
)

func eliminationTheftFixture(t *testing.T) (*GameState, *Permanent) {
	t.Helper()
	gs := newFixtureGame(t)
	// Seat 0 owns the creature; seat 1 controls it (theft/reanimation).
	stolen := &Permanent{
		Card: &Card{
			Name: "Stolen Bear", Owner: 0,
			Types: []string{"creature"},
		},
		Controller: 1, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	MintOGInstanceID(gs, stolen.Card)
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, stolen)
	return gs, stolen
}

// The stolen card must land in its OWNER's exile zone when the thief is
// eliminated — never vanish from all zones.
func TestElimination_StolenPermanentExiledToOwner_NotVanished(t *testing.T) {
	gs, stolen := eliminationTheftFixture(t)

	HandleSeatElimination(gs, 1)

	// r63 owner-immutability work: the shared return-to-owner operation
	// REVERTS control (owner's battlefield) instead of the #1046
	// exile-always MVP — §800.4a control effects end first. The
	// invariant under test is unchanged: the card exists in EXACTLY one
	// zone, the owner's.
	zones := 0
	onOwnerBF := false
	for _, s := range gs.Seats {
		for _, c := range s.Exile {
			if c == stolen.Card {
				zones++
			}
		}
		for _, c := range s.Graveyard {
			if c == stolen.Card {
				zones++
			}
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card == stolen.Card {
				zones++
				if s.Idx == 0 {
					onOwnerBF = true
				}
			}
		}
	}
	if zones != 1 || !onOwnerBF {
		t.Fatalf("stolen card: zones=%d onOwnerBF=%v — want exactly 1 zone, the owner's battlefield (§800.4a revert)", zones, onOwnerBF)
	}

	// The owner did not leave: the InstanceID must NOT be ceased.
	if _, ceased := gs.CeasedInstanceIDs[stolen.Card.InstanceID]; ceased {
		t.Error("other-owned card's InstanceID wrongly ceased — owner is still in the game")
	}
}

// The other-owned card must NOT inflate the leaver's cards_left_game —
// it did not leave the game (it moved to exile).
func TestElimination_StolenPermanentNotCountedAsLeft(t *testing.T) {
	gs, _ := eliminationTheftFixture(t)
	// Give the leaver one genuinely-owned battlefield card too.
	// (addBattlefield leaves Permanent.Owner at 0 — set it explicitly,
	// matching the real entry paths.)
	own := addBattlefield(gs, 1, "Own Bear", 2, 2, "creature")
	own.Owner = 1
	MintOGInstanceID(gs, own.Card)

	HandleSeatElimination(gs, 1)

	if got := gs.Seats[1].Flags["cards_left_game"]; got != 1 {
		t.Errorf("cards_left_game must count ONLY the leaver's own card: want 1, got %d", got)
	}
}

// Owner-side elimination unchanged: the leaver's OWN cards cease.
func TestElimination_OwnPermanentStillCeases(t *testing.T) {
	gs := newFixtureGame(t)
	own := addBattlefield(gs, 1, "Own Bear", 2, 2, "creature")
	own.Owner = 1
	MintOGInstanceID(gs, own.Card)

	HandleSeatElimination(gs, 1)

	if _, ceased := gs.CeasedInstanceIDs[own.Card.InstanceID]; !ceased {
		t.Error("leaver's own permanent must cease per §800.4a")
	}
	for _, s := range gs.Seats {
		for _, c := range s.Exile {
			if c == own.Card {
				t.Error("leaver's own card must not be exiled — it leaves the game entirely")
			}
		}
	}
}

// r63 seat-outcome checker finding (seed 42 games 85/89): theft-style
// handlers corrupt Permanent.Owner to the thief's seat while Card.Owner
// (CR §108.3, immutable) still names the victim. The elimination sweep
// must trust Card.Owner — a corrupt perm must still revert to its true
// owner's exile, never cease.
func TestElimination_CorruptPermOwner_CardOwnerAuthoritative(t *testing.T) {
	gs := newFixtureGame(t)
	corrupt := &Permanent{
		Card: &Card{Name: "Brute Suit", Owner: 0, Types: []string{"artifact"}},
		// The theft-handler corruption: BOTH perm fields claim the thief.
		Controller: 1, Owner: 1,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	MintOGInstanceID(gs, corrupt.Card)
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, corrupt)

	HandleSeatElimination(gs, 1)

	if _, ceased := gs.CeasedInstanceIDs[corrupt.Card.InstanceID]; ceased {
		t.Error("card OWNED (Card.Owner) by a surviving seat must never cease on another's elimination")
	}
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == corrupt.Card {
			found = true
		}
	}
	for _, c := range gs.Seats[0].Exile {
		if c == corrupt.Card {
			found = true
		}
	}
	if !found {
		t.Error("corrupt-perm card must return to its Card.Owner (battlefield revert, or exile when unrevertable)")
	}
	if got := gs.Seats[1].Flags["cards_left_game"]; got != 0 {
		t.Errorf("other-owned card must not count as the leaver's departure: want 0, got %d", got)
	}
}
