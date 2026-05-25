package per_card

// Loki r60 round 3 / game 1173 — Abuelo, Ancestral Echo appears in
// both seat 1 command_zone and seat 3 exile (same *Card pointer).
//
// Sequence captured by trace:
//
//   [3174] sacrifice seat=1 src=Abuelo, Ancestral Echo (Dusk Mangler)
//   [3175] zone_change seat=1 src=Abuelo to_zone=graveyard
//   ... triggers batched and queued on stack ...
//   [3183] sba_704_6d seat=1 src=Abuelo from_zone=graveyard
//          (commander SBA moves Abuelo from seat 1 graveyard → seat 1 cmdzone)
//   ... later ...
//   Gisa, Glorious Resurrector's creature_dies trigger resolves.
//   Its handler tries to lift dyingCard from seat 1 graveyard (no-op,
//   already moved), then appends dyingCard to seat 3 exile.
//
// Result: Abuelo *Card pointer lives in BOTH seat 1 command_zone
// (placed by §704.6d) AND seat 3 exile (placed by Gisa's handler).
// CardIdentity invariant fires at next snapshot.
//
// Root cause is the same race surface as the The Reaper / §704.6d
// fix: a per-card creature_dies handler that wants to "steal" the
// dying card must not run when sibling SBAs have already relocated
// the card.
//
// Fix: gisaResurrectorDies validates the card is still in dyingCard's
// owner's graveyard before stealing. If it's been moved (commander
// lifted to command zone, or any other relocation), skip the steal.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestGisaResurrector_SkipsWhenCommanderAlreadyInCommandZone(t *testing.T) {
	Reset()

	gs := gameengine.NewGameState(2, nil, nil)
	gs.CommanderFormat = true

	// Seat 0 (Gisa's controller).
	gisaCard := &gameengine.Card{
		Name: "Gisa, Glorious Resurrector", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	gisa := &gameengine.Permanent{
		Card: gisaCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, gisa)

	// Seat 1 commander — already moved into seat 1's command zone by
	// §704.6d before Gisa's trigger resolved.
	commanderCard := &gameengine.Card{
		Name: "Abuelo, Ancestral Echo", Owner: 1,
		Types: []string{"creature", "legendary"},
	}
	gs.Seats[1].CommanderNames = []string{"Abuelo, Ancestral Echo"}
	gs.Seats[1].CommandZone = append(gs.Seats[1].CommandZone, commanderCard)
	// NOT in seat 1 graveyard — that's the precondition for the race.

	dyingPerm := &gameengine.Permanent{
		Card: commanderCard, Controller: 1, Owner: 1,
	}
	ctx := map[string]interface{}{
		"perm":            dyingPerm,
		"card":            commanderCard,
		"controller_seat": 1,
	}
	gisaResurrectorDies(gs, gisa, ctx)

	// Post-fix: command_zone keeps its commander, Gisa's exile stays empty.
	if len(gs.Seats[1].CommandZone) != 1 || gs.Seats[1].CommandZone[0] != commanderCard {
		t.Fatalf("seat 1 command_zone should still hold commander, got %v", gs.Seats[1].CommandZone)
	}
	for _, c := range gs.Seats[0].Exile {
		if c == commanderCard {
			t.Fatalf("Gisa's exile must not contain the commander *Card pointer (cross-zone duplicate); got %v", gs.Seats[0].Exile)
		}
	}
}

func TestGisaResurrector_NoDuplicateAcrossExileAndCommandZone(t *testing.T) {
	// Stricter end-to-end check: no Card pointer appears in two zones
	// after the trigger resolves on a commander that was relocated.
	Reset()

	gs := gameengine.NewGameState(2, nil, nil)
	gs.CommanderFormat = true

	gisaCard := &gameengine.Card{
		Name: "Gisa, Glorious Resurrector", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	gisa := &gameengine.Permanent{
		Card: gisaCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, gisa)

	commanderCard := &gameengine.Card{
		Name: "Abuelo, Ancestral Echo", Owner: 1,
		Types: []string{"creature", "legendary"},
	}
	gs.Seats[1].CommanderNames = []string{"Abuelo, Ancestral Echo"}
	gs.Seats[1].CommandZone = append(gs.Seats[1].CommandZone, commanderCard)

	dyingPerm := &gameengine.Permanent{Card: commanderCard, Controller: 1, Owner: 1}
	gisaResurrectorDies(gs, gisa, map[string]interface{}{
		"perm": dyingPerm, "card": commanderCard, "controller_seat": 1,
	})

	zonesContaining := func(target *gameengine.Card) []string {
		var out []string
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, c := range s.Hand {
				if c == target {
					out = append(out, "hand")
				}
			}
			for _, c := range s.Graveyard {
				if c == target {
					out = append(out, "graveyard")
				}
			}
			for _, c := range s.Exile {
				if c == target {
					out = append(out, "exile")
				}
			}
			for _, c := range s.CommandZone {
				if c == target {
					out = append(out, "command_zone")
				}
			}
			for _, p := range s.Battlefield {
				if p != nil && p.Card == target {
					out = append(out, "battlefield")
				}
			}
		}
		return out
	}
	zones := zonesContaining(commanderCard)
	if len(zones) != 1 {
		t.Fatalf("commander *Card should live in exactly one zone, got %d: %v", len(zones), zones)
	}
}

func TestGisaResurrector_StillExilesNormalDyingCreature(t *testing.T) {
	// Regression-prevention: a non-commander dying creature that IS still
	// in its owner's graveyard at trigger-resolve time must still get
	// stolen into Gisa's exile.
	Reset()

	gs := gameengine.NewGameState(2, nil, nil)
	gs.CommanderFormat = true

	gisaCard := &gameengine.Card{
		Name: "Gisa, Glorious Resurrector", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	gisa := &gameengine.Permanent{
		Card: gisaCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, gisa)

	dyingCard := &gameengine.Card{
		Name: "Random Zombie Snack", Owner: 1,
		Types: []string{"creature"},
	}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, dyingCard)

	dyingPerm := &gameengine.Permanent{Card: dyingCard, Controller: 1, Owner: 1}
	gisaResurrectorDies(gs, gisa, map[string]interface{}{
		"perm": dyingPerm, "card": dyingCard, "controller_seat": 1,
	})

	// Gisa should have lifted the card out of seat 1 graveyard and
	// placed it in seat 0 exile.
	for _, c := range gs.Seats[1].Graveyard {
		if c == dyingCard {
			t.Fatal("dying card should be removed from seat 1 graveyard")
		}
	}
	found := false
	for _, c := range gs.Seats[0].Exile {
		if c == dyingCard {
			found = true
		}
	}
	if !found {
		t.Fatalf("dying card should be in Gisa's (seat 0) exile, got %v", gs.Seats[0].Exile)
	}
}
