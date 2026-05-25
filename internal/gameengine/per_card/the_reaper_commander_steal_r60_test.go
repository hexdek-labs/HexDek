package per_card

// Loki r60 / game 3042 — The Convention Enthusiast appears in both
// seat 1 command_zone and seat 1 battlefield (same *Card pointer) at
// turn 59 cleanup. Repro extracted into a unit test:
//
//  1. Seat 1's commander (Convention Enthusiast stand-in) is on seat 1's
//     battlefield with a -1/-1 counter on it (from The Reaper's ETB).
//  2. Some effect destroys it. Standard SBA cycle:
//      - §704.5g → destroy
//      - zone_change battlefield → graveyard (commander lands in graveyard;
//        §903.9b only redirects hand/library, not graveyard)
//      - §704.6d → owner moves commander from graveyard to command_zone
//  3. The Reaper's creature_dies trigger then resolves. Pre-fix it called
//     MoveCard(dyingCard, owner, "graveyard", "battlefield") — but the
//     card is no longer in the graveyard (§704.6d already lifted it).
//     MoveCard's "remove from graveyard" silently no-ops while "add to
//     battlefield" still wraps a fresh Permanent around the same *Card
//     pointer. Result: card lives in command_zone AND battlefield.
//
// Fix: theReaperDies validates the card is still in dyingCard.Owner's
// graveyard before stealing. If §704.6d (or any other SBA) lifted it
// elsewhere, skip the steal entirely.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestTheReaperDies_SkipsWhenDyingCommanderAlreadyInCommandZone(t *testing.T) {
	Reset()

	gs := gameengine.NewGameState(2, nil, nil)
	gs.CommanderFormat = true

	// Seat 0 controls The Reaper.
	reaperCard := &gameengine.Card{
		Name: "The Reaper, King No More", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	reaper := &gameengine.Permanent{
		Card: reaperCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, reaper)

	// Seat 1's commander — a stand-in legendary creature already moved
	// from its graveyard into its command zone by SBA §704.6d before
	// this trigger resolves.
	commanderCard := &gameengine.Card{
		Name: "Test Commander Victim", Owner: 1,
		Types: []string{"creature", "legendary"},
	}
	gs.Seats[1].CommanderNames = []string{"Test Commander Victim"}
	gs.Seats[1].CommandZone = append(gs.Seats[1].CommandZone, commanderCard)

	// The Permanent that "died" — a synthetic instance representing the
	// state at the moment of the death event. Counters carry the -1/-1
	// the steal predicate reads. The Permanent itself is no longer on
	// any battlefield (the destroy already removed it).
	dyingPerm := &gameengine.Permanent{
		Card: commanderCard, Controller: 1, Owner: 1,
		Counters: map[string]int{"-1/-1": 1},
	}

	ctx := map[string]interface{}{
		"perm":            dyingPerm,
		"card":            commanderCard,
		"controller_seat": 1,
	}
	theReaperDies(gs, reaper, ctx)

	// Post-fix: the steal must be a no-op. Command zone keeps the
	// commander; nothing appears on seat 0's battlefield (other than
	// The Reaper itself), and no duplicate appears anywhere.
	if len(gs.Seats[1].CommandZone) != 1 || gs.Seats[1].CommandZone[0] != commanderCard {
		t.Fatalf("seat 1 command_zone should retain the commander, got %v", gs.Seats[1].CommandZone)
	}
	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card == commanderCard {
				t.Fatalf("commander *Card must not appear on any battlefield; found on seat %d", s.Idx)
			}
		}
	}
}

func TestTheReaperDies_StillStealsWhenCardActuallyInGraveyard(t *testing.T) {
	// Regression-prevention: the validation must NOT break the legitimate
	// path. A non-commander dying creature with a -1/-1 counter in its
	// owner's graveyard should still get stolen.
	Reset()

	gs := gameengine.NewGameState(2, nil, nil)
	reaperCard := &gameengine.Card{
		Name: "The Reaper, King No More", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	reaper := &gameengine.Permanent{
		Card: reaperCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, reaper)

	dyingCard := &gameengine.Card{
		Name: "Random Goblin", Owner: 1,
		Types: []string{"creature"},
	}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, dyingCard)

	dyingPerm := &gameengine.Permanent{
		Card: dyingCard, Controller: 1, Owner: 1,
		Counters: map[string]int{"-1/-1": 1},
	}
	ctx := map[string]interface{}{
		"perm":            dyingPerm,
		"card":            dyingCard,
		"controller_seat": 1,
	}
	theReaperDies(gs, reaper, ctx)

	// Goblin should now be on some battlefield (stolen by The Reaper).
	// MoveCard places the Permanent in the owner's battlefield slice but
	// the handler updates Controller = perm.Controller (= 0).
	var stolen *gameengine.Permanent
	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.Card == dyingCard {
				stolen = p
			}
		}
	}
	if stolen == nil {
		t.Fatal("non-commander dying creature should be stolen onto a battlefield")
	}
	if stolen.Controller != 0 {
		t.Errorf("stolen creature should be controlled by The Reaper's seat (0), got %d", stolen.Controller)
	}
	// And the graveyard slot must be cleared.
	for _, c := range gs.Seats[1].Graveyard {
		if c == dyingCard {
			t.Fatal("dying card should be removed from seat 1 graveyard after steal")
		}
	}
}
