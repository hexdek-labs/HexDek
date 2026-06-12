package gameengine

import "testing"

// ---------------------------------------------------------------------------
// Test 1: CastAdventure registers a ZoneCastGrant on the exiled card
// ---------------------------------------------------------------------------

func TestAdventureCastRegistersZoneCastGrant(t *testing.T) {
	card := &Card{Name: "Bonecrusher Giant", Owner: 0, CMC: 3, Types: []string{"creature"}}

	gs := &GameState{
		Turn:           1,
		Seats:          []*Seat{{Idx: 0, Life: 40, Hand: []*Card{card}, ManaPool: 10}},
		Flags:          map[string]int{},
		ZoneCastGrants: map[*Card]*ZoneCastPermission{},
		EventPolicy:   EventLogFull,
		EventLog:       make([]Event, 0, 64),
	}

	err := CastAdventure(gs, 0, card, 2)
	if err != nil {
		t.Fatalf("CastAdventure returned error: %v", err)
	}

	// Card should be in exile after adventure resolves.
	seat := gs.Seats[0]
	found := false
	for _, c := range seat.Exile {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("card should be in exile after CastAdventure")
	}

	// Card should no longer be in hand.
	for _, c := range seat.Hand {
		if c == card {
			t.Errorf("card should not remain in hand after CastAdventure")
		}
	}

	// ZoneCastGrant should be registered.
	grant := gs.ZoneCastGrants[card]
	if grant == nil {
		t.Fatal("expected ZoneCastGrants[card] to be non-nil")
	}
	if grant.Zone != ZoneExile {
		t.Errorf("grant.Zone = %q, want %q", grant.Zone, ZoneExile)
	}
	if grant.Keyword != "adventure" {
		t.Errorf("grant.Keyword = %q, want %q", grant.Keyword, "adventure")
	}

	// Mana should have been spent.
	if seat.ManaPool != 8 {
		t.Errorf("ManaPool = %d, want 8 (started 10, paid 2)", seat.ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Prepared field on Permanent and Unprepare()
// ---------------------------------------------------------------------------

func TestPreparedFieldSetOnBecomePrepared(t *testing.T) {
	perm := &Permanent{
		Card:       &Card{Name: "Abigale, Poet Laureate", Owner: 0},
		Controller: 0,
		Timestamp:  1,
		Flags:      map[string]int{},
	}

	// Initially false.
	if perm.Prepared {
		t.Errorf("perm.Prepared should be false initially")
	}

	// Set prepared.
	perm.Prepared = true
	perm.Flags["prepared"] = 1

	if !perm.Prepared {
		t.Errorf("perm.Prepared should be true after setting")
	}

	// Unprepare clears both the bool and the flag.
	Unprepare(perm)

	if perm.Prepared {
		t.Errorf("perm.Prepared should be false after Unprepare")
	}
	if perm.Flags["prepared"] != 0 {
		t.Errorf("perm.Flags[\"prepared\"] = %d, want 0", perm.Flags["prepared"])
	}
}

func TestUnprepareNilSafe(t *testing.T) {
	// Should not panic on nil permanent.
	Unprepare(nil)

	// Should not panic on permanent with nil Flags.
	perm := &Permanent{
		Card:       &Card{Name: "Test Card"},
		Controller: 0,
		Prepared:   true,
	}
	Unprepare(perm)
	if perm.Prepared {
		t.Errorf("perm.Prepared should be false after Unprepare even with nil Flags")
	}
}

// ---------------------------------------------------------------------------
// Test 3: RegisterParadigmExile tracking
// ---------------------------------------------------------------------------

func TestParadigmExileTracking(t *testing.T) {
	gs := &GameState{
		Seats:         []*Seat{{Idx: 0, Life: 40}},
		Flags:         map[string]int{},
		ParadigmExile: map[int][]*Card{},
	}

	card := &Card{Name: "Paradigm Spell", Owner: 0}

	RegisterParadigmExile(gs, 0, card)

	if len(gs.ParadigmExile[0]) != 1 {
		t.Fatalf("len(ParadigmExile[0]) = %d, want 1", len(gs.ParadigmExile[0]))
	}
	if gs.ParadigmExile[0][0] != card {
		t.Errorf("ParadigmExile[0][0] should be the registered card")
	}
}

func TestParadigmExileTrackingMultipleCards(t *testing.T) {
	gs := &GameState{
		Seats: []*Seat{{Idx: 0, Life: 40}},
		Flags: map[string]int{},
	}
	// ParadigmExile intentionally nil — RegisterParadigmExile should init it.

	card1 := &Card{Name: "Paradigm Spell A", Owner: 0}
	card2 := &Card{Name: "Paradigm Spell B", Owner: 0}

	RegisterParadigmExile(gs, 0, card1)
	RegisterParadigmExile(gs, 0, card2)

	if len(gs.ParadigmExile[0]) != 2 {
		t.Fatalf("len(ParadigmExile[0]) = %d, want 2", len(gs.ParadigmExile[0]))
	}
}

func TestRegisterParadigmExileNilSafe(t *testing.T) {
	// nil gs should not panic.
	RegisterParadigmExile(nil, 0, &Card{Name: "Test"})

	// nil card should not panic.
	gs := &GameState{Seats: []*Seat{{Idx: 0, Life: 40}}}
	RegisterParadigmExile(gs, 0, nil)
	if gs.ParadigmExile != nil && len(gs.ParadigmExile[0]) != 0 {
		t.Errorf("nil card should not be registered")
	}
}

// ---------------------------------------------------------------------------
// Test 4: ResolveParadigmCopies — original stays in exile
// ---------------------------------------------------------------------------

func TestResolveParadigmCopies_CreatesAndResolves(t *testing.T) {
	card := &Card{Name: "Paradigm Bolt", Owner: 0, Types: []string{"sorcery"}}

	gs := &GameState{
		Turn:          1,
		Active:        0,
		Seats:         []*Seat{{Idx: 0, Life: 40, Exile: []*Card{card}}},
		Flags:         map[string]int{},
		ParadigmExile: map[int][]*Card{0: {card}},
		EventPolicy:  EventLogFull,
		EventLog:      make([]Event, 0, 64),
	}

	ResolveParadigmCopies(gs, 0)

	// The original card should still be in exile (paradigm copies don't
	// consume the original).
	found := false
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("original card should remain in exile after ResolveParadigmCopies")
	}

	// Check that a paradigm_copy_cast event was logged.
	foundEvent := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "paradigm_copy_cast" && ev.Seat == 0 {
			foundEvent = true
			break
		}
	}
	if !foundEvent {
		t.Errorf("expected paradigm_copy_cast event in EventLog")
	}
}

func TestResolveParadigmCopies_SkipsCardNotInExile(t *testing.T) {
	// Card is tracked in ParadigmExile but NOT actually in the exile zone.
	card := &Card{Name: "Missing Card", Owner: 0}

	gs := &GameState{
		Turn:          1,
		Active:        0,
		Seats:         []*Seat{{Idx: 0, Life: 40, Exile: []*Card{}}},
		Flags:         map[string]int{},
		ParadigmExile: map[int][]*Card{0: {card}},
		EventPolicy:  EventLogFull,
		EventLog:      make([]Event, 0, 64),
	}

	ResolveParadigmCopies(gs, 0)

	// No paradigm_copy_cast event should fire for a card not in exile.
	for _, ev := range gs.EventLog {
		if ev.Kind == "paradigm_copy_cast" {
			t.Errorf("should not fire paradigm_copy_cast for card not in exile zone")
		}
	}
}

func TestResolveParadigmCopies_NilSafe(t *testing.T) {
	// Explicit "doesn't panic" assertion via recover() — pre-Phase-2B
	// this test relied on the absence of a runtime panic without any
	// guard, which the audit flagged as a no-assertion test.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ResolveParadigmCopies must not panic; got %v", r)
		}
	}()

	ResolveParadigmCopies(nil, 0)
	gs := &GameState{
		Seats: []*Seat{{Idx: 0, Life: 40}},
		Flags: map[string]int{},
	}
	ResolveParadigmCopies(gs, 0)
}
