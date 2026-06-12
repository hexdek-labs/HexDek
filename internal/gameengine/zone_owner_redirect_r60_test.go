package gameengine

import (
	"testing"
)

// zone_owner_redirect_r60_test.go — regressions for the §400.7 /
// §402-406 owner-scoped zone redirect added to moveToZone. PR #685
// surfaced an 828-violation Loki cluster rooted in cross-seat exile
// routing; this enforcement at the API boundary catches the bug
// class for any non-zero-Owner card.

// -----------------------------------------------------------------------------
// isOwnerScopedZone — unit
// -----------------------------------------------------------------------------

func TestIsOwnerScopedZone_RecognizesPrivateZones(t *testing.T) {
	cases := []struct {
		zone string
		want bool
	}{
		{"hand", true},
		{"library", true},
		{"library_top", true},
		{"library_bottom", true},
		{"graveyard", true},
		{"exile", true},
		{"command_zone", true},
		{"battlefield", false},
		{"battlefield_tapped", false},
		{"stack", false},
		{"", false},
		{"foo", false},
	}
	for _, c := range cases {
		if got := isOwnerScopedZone(c.zone); got != c.want {
			t.Errorf("isOwnerScopedZone(%q) = %v, want %v", c.zone, got, c.want)
		}
	}
}

// -----------------------------------------------------------------------------
// moveToZone owner-redirect — integration
// -----------------------------------------------------------------------------

// Headline ask: a card with Owner=1 routed to seat 0's exile gets
// REDIRECTED to seat 1's exile per CR §400.7. The Etali bug shape.
func TestMoveToZone_NonZeroOwnerRedirectsToOwnerExile(t *testing.T) {
	gs := newTestState(4)
	c := &Card{Name: "Etali Tutored", Owner: 1, Types: []string{"sorcery"}}

	// Buggy caller: route to seat 0's exile.
	gs.moveToZone(0, c, "exile")

	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("seat 0 exile should be empty (redirect should fire); got %d cards", len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[1].Exile) != 1 || gs.Seats[1].Exile[0] != c {
		t.Errorf("seat 1 (owner) exile should hold the card; got %d cards", len(gs.Seats[1].Exile))
	}
	// Audit event should fire.
	if !hasEventKind(gs, "zone_owner_redirect") {
		t.Error("expected zone_owner_redirect event emitted for audit visibility")
	}
}

// All owner-scoped zones get the same treatment.
func TestMoveToZone_RedirectAppliesToAllPrivateZones(t *testing.T) {
	zones := []string{"hand", "library", "library_top", "library_bottom", "graveyard", "exile", "command_zone"}
	for _, z := range zones {
		t.Run(z, func(t *testing.T) {
			gs := newTestState(4)
			c := &Card{Name: "Card_" + z, Owner: 2}
			gs.moveToZone(0, c, z)
			// Find the card in seat 2's zones.
			s2 := gs.Seats[2]
			present := containsCardInSlice(s2.Hand, c) || containsCardInSlice(s2.Library, c) ||
				containsCardInSlice(s2.Graveyard, c) || containsCardInSlice(s2.Exile, c) ||
				containsCardInSlice(s2.CommandZone, c)
			if !present {
				t.Errorf("zone %q: card should land in seat 2 (owner); not found", z)
			}
		})
	}
}

// Battlefield is NOT owner-scoped — gain-control effects legitimately
// place a card on a non-owner's battlefield (CR §110.2a). The redirect
// must not fire for "battlefield" or "battlefield_tapped".
func TestMoveToZone_BattlefieldKeepsControllerSemantics(t *testing.T) {
	gs := newTestState(2)
	c := &Card{Name: "Stolen Bear", Owner: 1, BasePower: 2, BaseToughness: 2,
		Types: []string{"creature"}}

	gs.moveToZone(0, c, "battlefield")

	if len(gs.Seats[0].Battlefield) != 1 {
		t.Errorf("battlefield route should NOT redirect — gain-control legit; seat 0 bf=%d", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("owner's battlefield should NOT receive the card; seat 1 bf=%d", len(gs.Seats[1].Battlefield))
	}
	if hasEventKind(gs, "zone_owner_redirect") {
		t.Error("battlefield route should not emit zone_owner_redirect")
	}
}

// Zero-Owner cards (test convention / unloaded) bypass the redirect
// — Go's int zero-value collides with "seat 0 owner", and the per_card
// test suite constructs cards without explicit Owner then places them
// in non-zero seats' zones. Real-game cards loaded via decklist always
// have Owner set, so the actual bug class is still caught.
func TestMoveToZone_ZeroOwnerBypassesRedirect(t *testing.T) {
	gs := newTestState(2)
	c := &Card{Name: "Untagged Test Card"} // Owner defaults to 0

	gs.moveToZone(1, c, "graveyard")

	if len(gs.Seats[1].Graveyard) != 1 || gs.Seats[1].Graveyard[0] != c {
		t.Errorf("Owner=0 card should respect caller-passed seat 1; gy=%d", len(gs.Seats[1].Graveyard))
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("seat 0 graveyard should NOT receive the Owner=0 card via redirect")
	}
	if hasEventKind(gs, "zone_owner_redirect") {
		t.Error("Owner=0 should not emit redirect — bypass for test convention")
	}
}

// Owner == seat (no mismatch) → no redirect, no audit event, normal
// placement.
func TestMoveToZone_OwnerMatchesSeatNoRedirect(t *testing.T) {
	gs := newTestState(2)
	c := &Card{Name: "Own Card", Owner: 1}

	gs.moveToZone(1, c, "graveyard")

	if len(gs.Seats[1].Graveyard) != 1 {
		t.Errorf("owner-matched move should place in seat 1; got %d", len(gs.Seats[1].Graveyard))
	}
	if hasEventKind(gs, "zone_owner_redirect") {
		t.Error("matching seat/owner should NOT emit redirect event")
	}
}

// Owner out-of-range (defensive) shouldn't crash and shouldn't redirect.
func TestMoveToZone_OwnerOutOfRangeBypasses(t *testing.T) {
	gs := newTestState(2)
	c := &Card{Name: "Bad Owner Card", Owner: 99}

	gs.moveToZone(0, c, "graveyard")

	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("Owner out of range should fall through to caller seat; gy=%d", len(gs.Seats[0].Graveyard))
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func newTestState(seats int) *GameState {
	gs := &GameState{
		Seats:        make([]*Seat, seats),
		Flags:        map[string]int{},
		EventPolicy: EventLogFull,
	}
	for i := 0; i < seats; i++ {
		gs.Seats[i] = &Seat{Idx: i, Life: 40}
	}
	return gs
}

func hasEventKind(gs *GameState, kind string) bool {
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}

func containsCardInSlice(slice []*Card, c *Card) bool {
	for _, x := range slice {
		if x == c {
			return true
		}
	}
	return false
}
