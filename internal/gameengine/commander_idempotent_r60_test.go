package gameengine

// Loki r60 residual / game 3042 — CardIdentity duplicate where a
// commander appears in BOTH its seat's command_zone AND its seat's
// battlefield (same *Card pointer in both zones). Signature:
//   "The Convention Enthusiast" (ptr 0xc00400e360) in seat 1
//   command_zone + seat 1 battlefield, turn 59 cleanup.
//
// Root-cause class: defensive sanity. moveToZone (state.go:1524) and
// the SBA §704.6d commander sweep (sba.go:1729 `inCommandZone` helper)
// both use idempotent inserts — appending a *Card to a zone is skipped
// if the same pointer is already there. The two remaining
// command_zone-append paths did NOT:
//
//   - FireZoneChange's §903.9b branch (commander.go:297-310) — appends
//     unconditionally when ev.Payload["to_zone"] resolves to
//     "command_zone".
//   - addToZone's rollback branch (zone_cast.go:483) — appends
//     unconditionally when a failed cast needs to return a commander
//     to its source command_zone.
//
// Either path, fired while the commander's previous battlefield/zone
// reference is still live (e.g. a §903.9b redirect that races a
// concurrent SBA sweep, or a rollback for a cast that the SBA already
// re-shelved), produces the in-two-zones duplicate.
//
// Fix: both branches now check `inSlice(s.CommandZone)` before
// appending — matching the moveToZone / sba704_6d shape.

import (
	"testing"
)

func TestFireZoneChange_CommanderRedirect_IdempotentInsert(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.CommanderFormat = true
	seat := gs.Seats[0]

	cmdr := &Card{Name: "Test Commander", Owner: 0, Types: []string{"creature", "legendary"}}
	seat.CommandZone = append(seat.CommandZone, cmdr)
	seat.CommanderNames = []string{"Test Commander"}

	registerCommanderZoneReplacement(gs, 0, "Test Commander")

	// Fire a would_change_zone for the commander targeting hand —
	// §903.9b redirects to command_zone. The card is ALREADY in the
	// command zone; the redirect must not duplicate it.
	dest := FireZoneChange(gs, nil, cmdr, 0, "stack", "hand")
	if dest != "command_zone" {
		t.Fatalf("expected §903.9b redirect to command_zone, got %q", dest)
	}
	if len(seat.CommandZone) != 1 {
		t.Fatalf("expected exactly 1 commander in command_zone after redirect, got %d", len(seat.CommandZone))
	}
	if seat.CommandZone[0] != cmdr {
		t.Fatalf("command_zone[0] should be the original *Card pointer, got %p (want %p)", seat.CommandZone[0], cmdr)
	}
}

func TestFireZoneChange_CommanderRedirect_FirstInsertStillWorks(t *testing.T) {
	// The guard must only suppress DUPLICATE inserts — the legitimate
	// first redirect (commander dies → graveyard → §903.9b → command
	// zone) must still place the card.
	gs := NewGameState(2, nil, nil)
	gs.CommanderFormat = true
	seat := gs.Seats[0]
	seat.CommanderNames = []string{"Test Commander"}
	cmdr := &Card{Name: "Test Commander", Owner: 0, Types: []string{"creature", "legendary"}}
	// Commander is currently on battlefield-equivalent (not in command zone).
	seat.Graveyard = append(seat.Graveyard, cmdr)

	registerCommanderZoneReplacement(gs, 0, "Test Commander")

	dest := FireZoneChange(gs, nil, cmdr, 0, "battlefield", "hand")
	if dest != "command_zone" {
		t.Fatalf("expected §903.9b redirect to command_zone, got %q", dest)
	}
	if len(seat.CommandZone) != 1 || seat.CommandZone[0] != cmdr {
		t.Fatalf("expected commander placed in command_zone, got %d entries: %v", len(seat.CommandZone), seat.CommandZone)
	}
}

func TestAddToZone_CommandZoneRollback_IdempotentInsert(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	cmdr := &Card{Name: "Test Commander", Owner: 0, Types: []string{"creature", "legendary"}}
	seat.CommandZone = append(seat.CommandZone, cmdr)

	// Simulate a rollback path that re-adds the commander to command_zone
	// even though it's still there (e.g. cast failed AFTER an SBA sweep
	// already pushed the commander back).
	addToZone(seat, cmdr, "command_zone")

	if len(seat.CommandZone) != 1 {
		t.Fatalf("addToZone(command_zone) should be idempotent, got %d entries", len(seat.CommandZone))
	}
}

func TestAddToZone_CommandZoneRollback_FirstInsertStillWorks(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]

	cmdr := &Card{Name: "Test Commander", Owner: 0, Types: []string{"creature", "legendary"}}
	// Nothing in command zone yet — legitimate rollback should land.
	addToZone(seat, cmdr, "command_zone")

	if len(seat.CommandZone) != 1 || seat.CommandZone[0] != cmdr {
		t.Fatalf("addToZone should place commander on first call, got %v", seat.CommandZone)
	}
}

// Loki r60 / game 536 — ZoneCastGrantExpiry false-positive on
// Illusionary Mask → Midnight Covenant impulse_play grant. Grant was
// registered on turn 55, invariant fired at turn 55 draw step (BEFORE
// turn 55 cleanup runs), shouldExpireGrant's `gs.Turn >= grantTurn`
// returned true → flagged as "expired but still in ZoneCastGrants".
// The grant was in fact still alive; cleanup just hadn't run yet.
//
// Fix: the invariant now calls a stricter grantIsLeaked (gs.Turn >
// grantTurn) — only flag grants that should have been cleaned up by a
// PREVIOUS turn's cleanup. The cleanup itself keeps the `>=` semantics
// so it still removes grants at the correct end-of-turn moment.
func TestZoneCastGrantInvariant_DoesNotFlagSameTurnGrant(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Turn = 55
	card := &Card{Name: "Midnight Covenant", Owner: 3}
	grant := &ZoneCastPermission{
		Zone:      ZoneExile,
		Duration:  "until_end_of_turn",
		GrantTurn: 55, // registered THIS turn
	}
	gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{card: grant}

	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Fatalf("invariant should not flag a same-turn until_end_of_turn grant, got: %v", err)
	}

	// shouldExpireGrant (cleanup semantics) MUST still mark it as
	// expirable so end-of-turn cleanup removes it.
	if !shouldExpireGrant(gs, grant) {
		t.Fatalf("cleanup should remove a same-turn until_end_of_turn grant when run at end of turn")
	}
}

func TestZoneCastGrantInvariant_FlagsTrulyStaleGrant(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Turn = 56 // advanced past the grant turn
	card := &Card{Name: "Midnight Covenant", Owner: 3}
	gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{
		card: {
			Zone:      ZoneExile,
			Duration:  "until_end_of_turn",
			GrantTurn: 55,
		},
	}

	if err := checkZoneCastGrantExpiry(gs); err == nil {
		t.Fatal("invariant should flag a grant whose turn has passed without cleanup")
	}
}

func TestZoneCastGrantInvariant_UntilEndOfNextTurn(t *testing.T) {
	// until_end_of_next_turn semantics: alive during gs.Turn ≤ grantTurn+1.
	// At grantTurn+1 (same as the "next turn"), still alive — invariant
	// must not flag. At grantTurn+2, stale — invariant must flag.
	gs := NewGameState(2, nil, nil)
	card := &Card{Name: "X", Owner: 0}
	g := &ZoneCastPermission{Duration: "until_end_of_next_turn", GrantTurn: 10}
	gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{card: g}

	gs.Turn = 11
	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Fatalf("grantTurn=10 + until_end_of_next_turn must survive turn 11, got: %v", err)
	}
	gs.Turn = 12
	if err := checkZoneCastGrantExpiry(gs); err == nil {
		t.Fatal("grantTurn=10 + until_end_of_next_turn must be stale by turn 12")
	}
}
