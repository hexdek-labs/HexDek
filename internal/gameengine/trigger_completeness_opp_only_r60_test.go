package gameengine

import (
	"math/rand"
	"strings"
	"testing"
)

// Loki r60 mega-stress / seed 31415 game 237 turn 55 surfaced:
//
//   "TriggerCompleteness: death event 'sacrifice' at index 2728 with
//    trigger-bearer(s) [{Gisa, Glorious Resurrector 0}] on battlefield,
//    but no subsequent trigger/effect event found"
//
// Sequence captured in the event log:
//   [2720] stack_resolve seat=0 source=Birthing Ritual
//   [2722] sacrifice seat=0 source=creature token green saproling Token
//   [2723] trigger_evaluated seat=0 source=Birthing Ritual
//   ...
//   [2728] sacrifice seat=0 source=Blackbloom Rogue // Blackbloom Bog
//   [2729] zone_change seat=0 source=Blackbloom Rogue // Blackbloom Bog
//
// Gisa, Glorious Resurrector was on seat 0's battlefield. Blackbloom
// Rogue (a CREATURE) was sacrificed by seat 0 (own controller) as
// part of a Birthing Ritual chain. Gisa's printed oracle text and
// her per_card handler both gate on `controllerSeat == perm.Controller`
// — she does NOT trigger on her own controller's creatures dying.
//
// But the TriggerCompleteness invariant matched bearer.seat == death.seat
// without considering opp-only handlers, so it demanded a trigger that
// correctly didn't fire.
//
// NOT the same as PR #161 (cross-seat CardIdentity race with §704.6d
// commander sweep). That was a per_card *handler* bug. This is an
// *invariant* false positive — the engine state is correct.
//
// Fix: `opponentOnlyCreatureDiesTriggers` map in invariants.go lists
// the 6 known per_card handlers that early-return on same-seat dies;
// the invariant now matches those bearers against DIFFERENT-seat
// deaths, while keeping the standard same-seat match for OWN-creature
// trigger bearers (Sefris, Pitiless Plunderer, etc.).

// stubHasTriggerHook installs a HasTriggerHook that reports the given
// card names as having a creature_dies trigger. Reset on test cleanup.
func stubHasTriggerHook(t *testing.T, names ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	HasTriggerHook = func(cardName, event string) bool {
		return event == "creature_dies" && set[cardName]
	}
	t.Cleanup(func() { HasTriggerHook = nil })
}

func newTwoSeatGameForTriggerTest() *GameState {
	rng := rand.New(rand.NewSource(31415))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

// TestTriggerCompleteness_GisaOwnSacrificeDoesNotFalsePositive pins
// the exact loki r60 mega-stress seed-31415 game-237 shape: Gisa is
// on seat 0, seat 0 sacrifices its own creature (Blackbloom Rogue
// via Birthing Ritual chain), and the invariant must NOT demand a
// trigger.
func TestTriggerCompleteness_GisaOwnSacrificeDoesNotFalsePositive(t *testing.T) {
	stubHasTriggerHook(t, "Gisa, Glorious Resurrector")
	gs := newTwoSeatGameForTriggerTest()
	gisaCard := &Card{
		Name: "Gisa, Glorious Resurrector", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	gisa := &Permanent{
		Card: gisaCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, gisa)

	// Seat 0 sacrifices its own creature. Mirror the loki event shape:
	// kind=sacrifice, seat=0, with `was_creature=true` in Details.
	gs.EventLog = append(gs.EventLog, Event{
		Kind:   "sacrifice",
		Seat:   0,
		Source: "Blackbloom Rogue // Blackbloom Bog",
		Details: map[string]interface{}{
			"was_creature": true,
			"to_zone":      "graveyard",
		},
	})

	if err := checkTriggerCompleteness(gs); err != nil {
		t.Fatalf("TriggerCompleteness must NOT fire on Gisa's same-controller sacrifice; got: %v", err)
	}
}

// TestTriggerCompleteness_GisaOppDeathStillDemandsTrigger pins the
// other direction: when an OPPONENT'S creature dies and Gisa is on
// the bearer's battlefield, the invariant SHOULD still fire if no
// trigger event is logged (catching real bugs where Gisa's handler
// silently no-ops).
func TestTriggerCompleteness_GisaOppDeathStillDemandsTrigger(t *testing.T) {
	stubHasTriggerHook(t, "Gisa, Glorious Resurrector")
	gs := newTwoSeatGameForTriggerTest()
	gisaCard := &Card{
		Name: "Gisa, Glorious Resurrector", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	gisa := &Permanent{
		Card: gisaCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, gisa)

	// Seat 1 (opponent) creature dies, no follow-up trigger event.
	gs.EventLog = append(gs.EventLog, Event{
		Kind:   "sacrifice",
		Seat:   1,
		Source: "Some Opponent Creature",
		Details: map[string]interface{}{
			"was_creature": true,
			"to_zone":      "graveyard",
		},
	})

	err := checkTriggerCompleteness(gs)
	if err == nil {
		t.Fatal("TriggerCompleteness must fire when an opp creature dies with no Gisa trigger event")
	}
	if !strings.Contains(err.Error(), "Gisa, Glorious Resurrector") {
		t.Fatalf("error must name Gisa as the bearer; got: %v", err)
	}
}

// TestTriggerCompleteness_GisaOppDeathWithTriggerPasses confirms the
// normal flow: opp creature dies AND a trigger event follows →
// invariant clean.
func TestTriggerCompleteness_GisaOppDeathWithTriggerPasses(t *testing.T) {
	stubHasTriggerHook(t, "Gisa, Glorious Resurrector")
	gs := newTwoSeatGameForTriggerTest()
	gisaCard := &Card{
		Name: "Gisa, Glorious Resurrector", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	gisa := &Permanent{
		Card: gisaCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, gisa)

	gs.EventLog = append(gs.EventLog, Event{
		Kind: "sacrifice", Seat: 1, Source: "Some Opponent Creature",
		Details: map[string]interface{}{"was_creature": true, "to_zone": "graveyard"},
	})
	gs.EventLog = append(gs.EventLog, Event{
		Kind: "trigger_evaluated", Seat: 0, Source: "Gisa, Glorious Resurrector",
	})

	if err := checkTriggerCompleteness(gs); err != nil {
		t.Fatalf("TriggerCompleteness must NOT fire when trigger event follows opp death; got: %v", err)
	}
}

// TestTriggerCompleteness_OwnTriggerBearerSameSeatStillDemandsTrigger
// pins the legitimate OWN-side path so the opp-only carve-out doesn't
// regress the original invariant behavior for cards like Sefris of
// the Hidden Ways, Pitiless Plunderer, etc. (Same-seat death + no
// trigger → invariant must still fire.)
func TestTriggerCompleteness_OwnTriggerBearerSameSeatStillDemandsTrigger(t *testing.T) {
	stubHasTriggerHook(t, "Sefris of the Hidden Ways")
	gs := newTwoSeatGameForTriggerTest()
	sefrisCard := &Card{
		Name: "Sefris of the Hidden Ways", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	sefris := &Permanent{
		Card: sefrisCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, sefris)

	gs.EventLog = append(gs.EventLog, Event{
		Kind: "sacrifice", Seat: 0, Source: "Some Own Creature",
		Details: map[string]interface{}{"was_creature": true, "to_zone": "graveyard"},
	})

	err := checkTriggerCompleteness(gs)
	if err == nil {
		t.Fatal("TriggerCompleteness must fire when own creature dies with no Sefris trigger event")
	}
}

// TestTriggerCompleteness_OwnTriggerBearerOppDeathPasses confirms the
// other half: own-trigger bearers ignore opp-side deaths.
func TestTriggerCompleteness_OwnTriggerBearerOppDeathPasses(t *testing.T) {
	stubHasTriggerHook(t, "Sefris of the Hidden Ways")
	gs := newTwoSeatGameForTriggerTest()
	sefrisCard := &Card{
		Name: "Sefris of the Hidden Ways", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	sefris := &Permanent{
		Card: sefrisCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, sefris)

	// Opp creature dies — Sefris doesn't care.
	gs.EventLog = append(gs.EventLog, Event{
		Kind: "sacrifice", Seat: 1, Source: "Some Opp Creature",
		Details: map[string]interface{}{"was_creature": true, "to_zone": "graveyard"},
	})

	if err := checkTriggerCompleteness(gs); err != nil {
		t.Fatalf("Sefris must NOT demand a trigger on opp creature death; got: %v", err)
	}
}

// TestTriggerCompleteness_AllSixOppOnlyBearersTreatedConsistently is
// a sweep test pinning that every entry in the
// `opponentOnlyCreatureDiesTriggers` registry receives the same
// treatment — own-seat sacrifice is silently ignored, opp-seat
// sacrifice without trigger fires the invariant. Adding a new
// opp-only handler to the map should be enough to make this pass
// for that handler.
func TestTriggerCompleteness_AllSixOppOnlyBearersTreatedConsistently(t *testing.T) {
	cards := []string{
		"Gisa, Glorious Resurrector",
		"The Reaper, King No More",
		"Toxrill, the Corrosive",
		"Yahenni, Undying Partisan",
		"Grave Pact",
		"Grave Betrayal",
	}
	for _, name := range cards {
		t.Run(name, func(t *testing.T) {
			if !opponentOnlyCreatureDiesTriggers[name] {
				t.Fatalf("%q must be registered in opponentOnlyCreatureDiesTriggers", name)
			}

			// Own-sacrifice clean run.
			stubHasTriggerHook(t, name)
			gs := newTwoSeatGameForTriggerTest()
			perm := &Permanent{
				Card:       &Card{Name: name, Owner: 0, Types: []string{"creature"}},
				Controller: 0, Owner: 0,
				Timestamp: gs.NextTimestamp(),
				Counters:  map[string]int{}, Flags: map[string]int{},
			}
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
			gs.EventLog = append(gs.EventLog, Event{
				Kind: "sacrifice", Seat: 0, Source: "Own Creature",
				Details: map[string]interface{}{"was_creature": true, "to_zone": "graveyard"},
			})
			if err := checkTriggerCompleteness(gs); err != nil {
				t.Fatalf("%q must NOT demand trigger on own creature sacrifice; got: %v", name, err)
			}

			// Opp-sacrifice without trigger fires.
			gs2 := newTwoSeatGameForTriggerTest()
			perm2 := &Permanent{
				Card:       &Card{Name: name, Owner: 0, Types: []string{"creature"}},
				Controller: 0, Owner: 0,
				Timestamp: gs2.NextTimestamp(),
				Counters:  map[string]int{}, Flags: map[string]int{},
			}
			gs2.Seats[0].Battlefield = append(gs2.Seats[0].Battlefield, perm2)
			gs2.EventLog = append(gs2.EventLog, Event{
				Kind: "sacrifice", Seat: 1, Source: "Opp Creature",
				Details: map[string]interface{}{"was_creature": true, "to_zone": "graveyard"},
			})
			if err := checkTriggerCompleteness(gs2); err == nil {
				t.Fatalf("%q must demand a trigger on opp creature death; got nil error", name)
			}
		})
	}
}
