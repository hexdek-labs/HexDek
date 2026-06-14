package gameengine

// r63 — close the ZoneCastGrantExpiry leak (CLAUDE.md Loki r41 open issue) and
// the cast-from-zone grant-removal gap (impulse-draw property probe).
//
// Leak: ExpireZoneCastGrants runs only at the §514.2 cleanup step. An impulse /
// cast-from-exile grant registered after that step, or on a turn whose cleanup
// was skipped, survives into a later turn and trips the ZoneCastGrantExpiry
// invariant. SweepLeakedZoneCastGrants — wired into ScanExpiredDurations (every
// scan, including the §502 untap step that opens each turn) — reaps any grant
// already past its declared expiry, using the invariant's own grantIsLeaked
// predicate so in-window grants are untouched.

import "testing"

func backstopGame(turn int) *GameState {
	return &GameState{
		Turn:           turn,
		Seats:          []*Seat{{Idx: 0, Life: 40}, {Idx: 1, Life: 40}},
		Flags:          map[string]int{},
		ZoneCastGrants: map[*Card]*ZoneCastPermission{},
	}
}

func ueotGrant(turn int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone: ZoneExile, Keyword: "impulse_play", ManaCost: -1,
		RequireController: 0, SourceName: "Outpost Siege",
		Duration: "until_end_of_turn", GrantTurn: turn,
	}
}

// A leaked until_end_of_turn grant from a prior turn is reaped at the untap
// scan that opens the new turn — and the invariant is clean afterward.
func TestZoneCastBackstop_LeakedUEOTReapedAtUntap(t *testing.T) {
	gs := backstopGame(5)
	card := &Card{Name: "Stranded Impulse Card", Owner: 0}
	gs.Seats[0].Exile = []*Card{card}
	RegisterZoneCastGrant(gs, card, ueotGrant(5))

	// Simulate the leak: turn advances to 6 without cleanup having reaped it.
	gs.Turn = 6
	if err := checkZoneCastGrantExpiry(gs); err == nil {
		t.Fatalf("precondition: an unreaped turn-5 grant should be a leak at turn 6")
	}

	// The §502 untap-step scan must reap it.
	ScanExpiredDurations(gs, "beginning", "untap")
	if _, ok := gs.ZoneCastGrants[card]; ok {
		t.Errorf("leaked until_end_of_turn grant should be swept at the untap scan")
	}
	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Errorf("ZoneCastGrantExpiry should be clean after the backstop: %v", err)
	}
}

// The backstop must NOT reap a grant that is still inside its window.
func TestZoneCastBackstop_DoesNotReapInWindow(t *testing.T) {
	// Same-turn until_end_of_turn grant — still valid this turn.
	gs := backstopGame(5)
	c1 := &Card{Name: "Fresh UEOT", Owner: 0}
	gs.Seats[0].Exile = []*Card{c1}
	RegisterZoneCastGrant(gs, c1, ueotGrant(5))
	SweepLeakedZoneCastGrants(gs)
	if _, ok := gs.ZoneCastGrants[c1]; !ok {
		t.Errorf("a same-turn until_end_of_turn grant must survive the backstop")
	}

	// until_end_of_next_turn from turn 5 survives turn 6, dies at turn 7.
	gs2 := backstopGame(5)
	c2 := &Card{Name: "Light Up the Stage Card", Owner: 0}
	gs2.Seats[0].Exile = []*Card{c2}
	RegisterZoneCastGrant(gs2, c2, &ZoneCastPermission{
		Zone: ZoneExile, Keyword: "impulse_play", RequireController: 0,
		Duration: "until_end_of_next_turn", GrantTurn: 5,
	})
	gs2.Turn = 6
	SweepLeakedZoneCastGrants(gs2)
	if _, ok := gs2.ZoneCastGrants[c2]; !ok {
		t.Errorf("until_end_of_next_turn grant must survive its own next turn (6)")
	}
	gs2.Turn = 7
	SweepLeakedZoneCastGrants(gs2)
	if _, ok := gs2.ZoneCastGrants[c2]; ok {
		t.Errorf("until_end_of_next_turn grant must be reaped at turn 7")
	}
}

// Property (c)/(d): casting a card from a zone via a registered grant removes
// the card from exile AND drops its ZoneCastGrants entry (no stale grant).
func TestCastFromZone_RemovesGrantOnCast(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 4

	card := addExileCard(gs, 0, "Impulsed Bolt", 1, "instant")
	card.CMC = 1
	perm := &ZoneCastPermission{
		Zone: ZoneExile, Keyword: "impulse_play", ManaCost: -1,
		RequireController: 0, SourceName: "Bonehoard Dracosaur",
		Duration: "until_end_of_turn", GrantTurn: gs.Turn,
	}
	RegisterZoneCastGrant(gs, card, perm)

	if GetZoneCastGrant(gs, card) == nil {
		t.Fatalf("precondition: grant should be registered")
	}

	if _, err := CastFromZone(gs, 0, card, ZoneExile, perm, nil); err != nil {
		t.Fatalf("CastFromZone failed: %v", err)
	}

	// Card left exile.
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			t.Errorf("card should have left exile after being cast")
		}
	}
	// Grant entry dropped — no stale permission.
	if GetZoneCastGrant(gs, card) != nil {
		t.Errorf("ZoneCastGrant must be removed when the card is cast from exile")
	}
	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Errorf("ZoneCastGrantExpiry should be clean after cast: %v", err)
	}
	// Real cost paid (CMC 1).
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected 3 mana left after paying the card's real cost, got %d", gs.Seats[0].ManaPool)
	}
}

// Property (f): the grant is per-card — casting one impulsed card does not
// disturb another impulsed card's grant.
func TestCastFromZone_PerCardGrantNoLeak(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 4

	a := addExileCard(gs, 0, "Impulse A", 1, "instant")
	a.CMC = 1
	b := addExileCard(gs, 0, "Impulse B", 1, "instant")
	b.CMC = 1
	mk := func(name string) *ZoneCastPermission {
		return &ZoneCastPermission{
			Zone: ZoneExile, Keyword: "impulse_play", ManaCost: -1,
			RequireController: 0, SourceName: name,
			Duration: "until_end_of_turn", GrantTurn: gs.Turn,
		}
	}
	permA := mk("src")
	RegisterZoneCastGrant(gs, a, permA)
	RegisterZoneCastGrant(gs, b, mk("src"))

	if _, err := CastFromZone(gs, 0, a, ZoneExile, permA, nil); err != nil {
		t.Fatalf("CastFromZone(A) failed: %v", err)
	}
	if GetZoneCastGrant(gs, a) != nil {
		t.Errorf("A's grant should be gone after casting A")
	}
	if GetZoneCastGrant(gs, b) == nil {
		t.Errorf("B's grant must remain intact when A is cast (per-card, no leak)")
	}
}
