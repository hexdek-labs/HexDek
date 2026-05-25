package gameengine

import (
	"math/rand"
	"testing"
)

// Loki r60 extreme-stress / seed 42 game 5480 turn 38 surfaced:
//
//   ZoneCastGrantExpiry: grant for "Compound Fracture"
//   (zone=graveyard duration=while_source_on_bf grantTurn=0
//    sourceTimestamp=32 source=Kess, Dissident Mage)
//   has expired but is still in ZoneCastGrants — cleanup missed
//
// Sequence: Kess, Dissident Mage on seat 0's battlefield (timestamp
// 32) registered a `while_source_on_bf` graveyard-cast grant on
// Compound Fracture. Seat 0 decked out (library=0) and was eliminated
// by SBA 704.5b. `HandleSeatElimination` at multiplayer.go:344-356
// removed Kess from the battlefield as part of the §800.4a cleanup,
// called `UnregisterReplacementsForPermanent` and
// `UnregisterContinuousEffectsForPermanent`, but did NOT call
// `ExpireSourceGrants(gs, p.Timestamp)`. Kess's grant therefore
// outlived its source — invariant fires.
//
// Same cluster spread across seeds 31415 game 5610 and 271828 game
// 5399, all on different commander pods but the same lifecycle gap:
// the seat-elimination path was the one LTB-equivalent path that
// PR #106's plumbing (DestroyPermanent / ExilePermanent /
// sacrificePermanentImpl / BouncePermanent / destroyPermSBA /
// sacrificePermSBA) didn't cover.
//
// Fix: HandleSeatElimination now calls ExpireSourceGrants on every
// permanent owned/controlled by the leaving seat as it gets swept.

// TestSeatElimination_ExpiresWhileSourceOnBfGrant pins the bit-stable
// signature: Kess on seat 0's battlefield grants a graveyard-cast
// permission for Compound Fracture; seat 0 is eliminated; the grant
// must be removed from gs.ZoneCastGrants.
func TestSeatElimination_ExpiresWhileSourceOnBfGrant(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Kess, Dissident Mage on seat 0's battlefield.
	kessCard := &Card{
		Name:  "Kess, Dissident Mage",
		Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	kess := &Permanent{
		Card:       kessCard,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, kess)

	// Compound Fracture in seat 0's graveyard with a Kess grant.
	fracture := &Card{
		Name:  "Compound Fracture",
		Owner: 0,
		Types: []string{"instant"},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, fracture)

	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{}
	}
	gs.ZoneCastGrants[fracture] = &ZoneCastPermission{
		Zone:            ZoneGraveyard,
		Duration:        "while_source_on_bf",
		SourceTimestamp: kess.Timestamp,
		SourceName:      "Kess, Dissident Mage",
	}

	// Eliminate seat 0 (e.g. decked out, lost via SBA 704.5b).
	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	// The grant must be gone — Kess left play, while_source_on_bf
	// expires.
	if _, present := gs.ZoneCastGrants[fracture]; present {
		t.Fatalf("Kess's while_source_on_bf grant must expire when seat 0 is eliminated; ZoneCastGrants still contains the entry")
	}

	// The invariant must now run clean.
	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Fatalf("ZoneCastGrantExpiry invariant must pass post-elimination; got: %v", err)
	}
}

// TestSeatElimination_PreservesGrantsFromOtherSeats confirms the
// elimination cleanup is scoped — grants whose source is on a
// SURVIVING seat's battlefield must NOT be touched.
func TestSeatElimination_PreservesGrantsFromOtherSeats(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Kess on seat 1's battlefield (survives).
	kessCard := &Card{Name: "Kess, Dissident Mage", Owner: 1, Types: []string{"creature", "legendary"}}
	kess := &Permanent{
		Card: kessCard, Controller: 1, Owner: 1,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, kess)

	// Compound Fracture in seat 1's graveyard with a Kess grant.
	fracture := &Card{Name: "Compound Fracture", Owner: 1, Types: []string{"instant"}}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, fracture)

	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{}
	}
	gs.ZoneCastGrants[fracture] = &ZoneCastPermission{
		Zone:            ZoneGraveyard,
		Duration:        "while_source_on_bf",
		SourceTimestamp: kess.Timestamp,
		SourceName:      "Kess, Dissident Mage",
	}

	// Seat 0 (a different seat) is eliminated.
	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	if _, present := gs.ZoneCastGrants[fracture]; !present {
		t.Fatal("seat 0's elimination must NOT expire seat 1's Kess grant — the source permanent is still on seat 1's battlefield")
	}
}

// TestSeatElimination_ExpiresGrantsFromControlledPermanents covers the
// Gilded-Drake-style case: seat 0 owns Kess but seat 1 stole control
// via Gilded Drake; seat 1 is then eliminated. The §800.4a cleanup
// removes Kess from seat 1's battlefield (controller==1), so the
// grant must expire on seat 1's elimination too.
func TestSeatElimination_ExpiresGrantsFromControlledPermanents(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Kess owned by seat 0 but controlled by seat 1 (Gilded Drake trade).
	kessCard := &Card{Name: "Kess, Dissident Mage", Owner: 0, Types: []string{"creature", "legendary"}}
	kess := &Permanent{
		Card: kessCard, Controller: 1, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, kess)

	fracture := &Card{Name: "Compound Fracture", Owner: 1, Types: []string{"instant"}}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, fracture)

	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{}
	}
	gs.ZoneCastGrants[fracture] = &ZoneCastPermission{
		Zone:            ZoneGraveyard,
		Duration:        "while_source_on_bf",
		SourceTimestamp: kess.Timestamp,
		SourceName:      "Kess, Dissident Mage",
	}

	// Seat 1 is eliminated. Kess (controller=1) gets swept from battlefield.
	gs.Seats[1].Lost = true
	HandleSeatElimination(gs, 1)

	if _, present := gs.ZoneCastGrants[fracture]; present {
		t.Fatal("seat 1's elimination removes Kess from its battlefield; the while_source_on_bf grant must expire")
	}
}

// TestSeatElimination_NoOpWhenSeatHasNoGrantSource confirms the new
// hook doesn't error or spuriously remove unrelated grants when the
// eliminated seat has no grant-source permanents.
func TestSeatElimination_NoOpWhenSeatHasNoGrantSource(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	bear := &Permanent{
		Card:       &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bear)

	// An unrelated grant from a different timestamp.
	otherCard := &Card{Name: "Unrelated", Owner: 1}
	gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{
		otherCard: {
			Zone:            ZoneGraveyard,
			Duration:        "while_source_on_bf",
			SourceTimestamp: 9999,
			SourceName:      "OtherCard",
		},
	}

	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	if _, present := gs.ZoneCastGrants[otherCard]; !present {
		t.Fatal("unrelated grant (different SourceTimestamp) must not be removed by seat 0's elimination")
	}
}
