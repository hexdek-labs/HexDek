package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
)

// Property tests for the InstanceID gap-walk backstop helper
// (instanceid_gap_walk.go). The helper closes residual CardIdentity +
// ZoneConservation gaps surfaced by Phase A's 5000-game Loki sweep.

func TestGapWalk_EmptyInstanceID_NoOp(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	card := &Card{Name: "x"}
	if EnforceBattlefieldUniqueInstanceID(gs, card, 0) {
		t.Fatal("expected no-op when InstanceID is empty, got true")
	}
}

func TestGapWalk_NilMinter_NoOp(t *testing.T) {
	gs := &GameState{Seats: []*Seat{newSeat(0)}}
	card := &Card{InstanceID: "h0OGVC000010"}
	if EnforceBattlefieldUniqueInstanceID(gs, card, 0) {
		t.Fatal("expected no-op when IIDMinter is nil, got true")
	}
}

func TestGapWalk_NoCollision_NoOp(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	card := &Card{Name: "Unique", Owner: 0}
	MintOGInstanceID(gs, card)
	if card.InstanceID == "" {
		t.Fatal("MintOGInstanceID failed to stamp ID")
	}
	if EnforceBattlefieldUniqueInstanceID(gs, card, 0) {
		t.Fatal("expected no-op when no collision exists, got true")
	}
}

func TestGapWalk_DifferentPointerSameID_Remints(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	// First card: minted as OG, placed on seat 0 battlefield.
	original := &Card{Name: "Source", Owner: 0, Colors: []string{"R"}, CMC: 2}
	MintOGInstanceID(gs, original)
	origID := original.InstanceID
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: original, Controller: 0, Owner: 0,
	})

	// Second card: a sibling DeepCopy carrying the SAME ID. This is the
	// pre-fix bug shape (Calix / Hashaton / Brudiclad).
	sibling := original.DeepCopy()
	if sibling.InstanceID != origID {
		t.Fatalf("DeepCopy did not preserve ID, got %q want %q", sibling.InstanceID, origID)
	}

	if !EnforceBattlefieldUniqueInstanceID(gs, sibling, 0) {
		t.Fatal("expected re-mint on different-pointer same-ID collision")
	}
	if sibling.InstanceID == origID {
		t.Fatalf("sibling InstanceID was not changed: still %q", sibling.InstanceID)
	}
	if sibling.InstanceID == "" {
		t.Fatal("sibling InstanceID is empty after re-mint")
	}
	if sibling.SourceInstanceID != origID {
		t.Fatalf("lineage not preserved: SourceInstanceID=%q want %q", sibling.SourceInstanceID, origID)
	}
	if sibling.Provenance != instanceid.ProvTK {
		t.Fatalf("re-minted Provenance should be ProvTK, got %v", sibling.Provenance)
	}
	// Original keeps its ID untouched.
	if original.InstanceID != origID {
		t.Fatalf("original ID was mutated: %q want %q", original.InstanceID, origID)
	}
	if _, ok := gs.MintedInstanceIDs[sibling.InstanceID]; !ok {
		t.Fatalf("new minted ID not recorded in MintedInstanceIDs: %q", sibling.InstanceID)
	}
}

func TestGapWalk_SamePointerInGraveyard_PurgesStale(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	card := &Card{Name: "Tower", Owner: 0, Colors: []string{}, CMC: 0}
	MintOGInstanceID(gs, card)
	cardID := card.InstanceID
	// Stage the stale shape: same *Card pointer in BOTH graveyard AND
	// pending-battlefield slots (CR §400.7c violation).
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	if !EnforceBattlefieldUniqueInstanceID(gs, card, 0) {
		t.Fatal("expected purge action on same-pointer-in-other-zone collision")
	}
	// Graveyard should be empty after purge.
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Fatalf("graveyard not purged: %d entries left", len(gs.Seats[0].Graveyard))
	}
	// ID should be unchanged on the card — purge does not re-mint.
	if card.InstanceID != cardID {
		t.Fatalf("card ID was changed by purge: %q want %q", card.InstanceID, cardID)
	}
}

func TestGapWalk_ScansAllPrivateZones(t *testing.T) {
	// Each private zone holding the same-pointer stale reference is
	// independently purged.
	for _, zone := range []string{"hand", "graveyard", "exile", "library", "command_zone"} {
		zone := zone
		t.Run(zone, func(t *testing.T) {
			gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
			card := &Card{Name: "X", Owner: 0}
			MintOGInstanceID(gs, card)
			seat := gs.Seats[0]
			switch zone {
			case "hand":
				seat.Hand = append(seat.Hand, card)
			case "graveyard":
				seat.Graveyard = append(seat.Graveyard, card)
			case "exile":
				seat.Exile = append(seat.Exile, card)
			case "library":
				seat.Library = append(seat.Library, card)
			case "command_zone":
				seat.CommandZone = append(seat.CommandZone, card)
			}
			if !EnforceBattlefieldUniqueInstanceID(gs, card, 0) {
				t.Fatalf("expected purge from %s", zone)
			}
		})
	}
}

func TestGapWalk_CrossSeatDifferentPtrSameID_Remints(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(1)), nil)
	original := &Card{Name: "S", Owner: 3, Colors: []string{"U"}, CMC: 4}
	MintOGInstanceID(gs, original)
	origID := original.InstanceID
	// Original lives on seat 3's battlefield.
	gs.Seats[3].Battlefield = append(gs.Seats[3].Battlefield, &Permanent{
		Card: original, Controller: 3, Owner: 3,
	})
	// Sibling DeepCopy now lands on seat 0.
	sibling := original.DeepCopy()
	if !EnforceBattlefieldUniqueInstanceID(gs, sibling, 0) {
		t.Fatal("expected re-mint on cross-seat sibling collision")
	}
	if sibling.InstanceID == origID {
		t.Fatal("sibling not re-minted")
	}
}

func TestGapWalk_TokenTypeStampedOnRemint(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	original := &Card{Name: "X", Owner: 0, Types: []string{"creature"}, CMC: 1}
	MintOGInstanceID(gs, original)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: original, Controller: 0, Owner: 0,
	})
	sibling := original.DeepCopy()
	EnforceBattlefieldUniqueInstanceID(gs, sibling, 0)
	hasToken := false
	for _, t := range sibling.Types {
		if t == "token" {
			hasToken = true
			break
		}
	}
	if !hasToken {
		t.Fatalf("token type not stamped on re-mint; types=%v", sibling.Types)
	}
}
