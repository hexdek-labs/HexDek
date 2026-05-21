package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r53 regression — gerrardDies must verify the *Card is in the
// graveyard before exiling it. Stale-trigger paths (CR §903.9b
// commander-zone redirect, SBA §704.6d sweep, recast during the same
// turn) leave the trigger handler running with perm.Card already
// elsewhere — the pre-r53 handler blindly appended the *Card to exile
// anyway, producing the Loki r48 / r50 / r51 / r52 CardIdentity
// "appears in both seat 2 command_zone and seat 2 battlefield"
// violation (1352 hits in r48, 318+ in r50/r51 once the Mondrak fix
// unmasked the Gerrard surface; surfaced again in r52 by dev-6 +
// game-2432 trace).

func TestGerrardDies_ExilesSelfWhenInGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	gerrard := addPerm(gs, 0, "Gerrard, Weatherlight Hero", "creature", "legendary")
	// Put Gerrard's *Card in the graveyard, the way a "dies" SBA would
	// before the trigger fires.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gerrard.Card)

	gerrardDies(gs, gerrard, nil)

	// Self-exile path: Gerrard out of graveyard, into exile.
	for _, c := range gs.Seats[0].Graveyard {
		if c == gerrard.Card {
			t.Fatalf("Gerrard's Card should have been removed from graveyard")
		}
	}
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == gerrard.Card {
			foundInExile = true
		}
	}
	if !foundInExile {
		t.Fatalf("Gerrard's Card should be in exile after self-exile")
	}
}

func TestGerrardDies_RecursAllArtifactsAndCreaturesInGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	gerrard := addPerm(gs, 0, "Gerrard, Weatherlight Hero", "creature", "legendary")
	bear := &gameengine.Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}
	sword := &gameengine.Card{Name: "Sword of Fire and Ice", Owner: 0, Types: []string{"artifact", "equipment"}}
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bear, sword, bolt, gerrard.Card)

	gerrardDies(gs, gerrard, nil)

	// Bear + Sword should be back on the battlefield, Bolt stays in gy.
	foundBear, foundSword := false, false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card == bear {
			foundBear = true
		}
		if p.Card == sword {
			foundSword = true
		}
	}
	if !foundBear || !foundSword {
		t.Fatalf("expected Bear+Sword reanimated; foundBear=%v foundSword=%v; bf=%v",
			foundBear, foundSword, gs.Seats[0].Battlefield)
	}
	stillInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == bolt {
			stillInGY = true
		}
	}
	if !stillInGY {
		t.Fatalf("non-creature non-artifact Bolt should stay in graveyard")
	}
}

// CORE r53 regression — pin the leak shape directly.
// When gerrardDies fires with Gerrard's *Card already in the COMMAND
// ZONE (CR §903.9b redirect already moved it, or §704.6d SBA already
// swept it from graveyard), the handler must NOT append a stray *Card
// to seat.Exile. The pre-fix handler did, producing the CardIdentity
// "appears in both seat 2 command_zone and seat 2 battlefield"
// violation once the recast wrapped the same *Card on the battlefield.
func TestGerrardDies_DoesNotLeakWhenCardAlreadyInCommandZone(t *testing.T) {
	gs := newGame(t, 2)
	gerrard := addPerm(gs, 0, "Gerrard, Weatherlight Hero", "creature", "legendary")
	// Simulate: §903.9b put Gerrard's *Card in the command zone after
	// his prior death + SBA cycle. The graveyard is clean.
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, gerrard.Card)

	gerrardDies(gs, gerrard, nil)

	// The *Card must NOT appear in seat.Exile (the leak surface).
	for _, c := range gs.Seats[0].Exile {
		if c == gerrard.Card {
			t.Fatalf("REGRESSION: Gerrard's *Card appended to exile despite being in command_zone (loki r50/r51/r52 game-2432 leak)")
		}
	}
	// And the *Card must still be in the command_zone (uniquely).
	czCount := 0
	for _, c := range gs.Seats[0].CommandZone {
		if c == gerrard.Card {
			czCount++
		}
	}
	if czCount != 1 {
		t.Fatalf("expected Gerrard's *Card to remain singly in command_zone; got %d", czCount)
	}
	// The *Card must still be wrapped by exactly one Permanent on the
	// battlefield (the test fixture Permanent — we're simulating the
	// recast that happened between deaths).
	bfCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == gerrard.Card {
			bfCount++
		}
	}
	if bfCount != 1 {
		t.Fatalf("expected one Permanent wrapping Gerrard's Card; got %d", bfCount)
	}
}

// Companion regression: same shape, but Gerrard's *Card is already
// elsewhere (a previous trigger already moved him to exile). The
// handler's exile-move must not double-append.
func TestGerrardDies_DoesNotDoubleAppendWhenCardAlreadyInExile(t *testing.T) {
	gs := newGame(t, 2)
	gerrard := addPerm(gs, 0, "Gerrard, Weatherlight Hero", "creature", "legendary")
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, gerrard.Card)

	gerrardDies(gs, gerrard, nil)

	// Still exactly one exile entry.
	exCount := 0
	for _, c := range gs.Seats[0].Exile {
		if c == gerrard.Card {
			exCount++
		}
	}
	if exCount != 1 {
		t.Fatalf("expected single exile entry post-trigger; got %d (double-append regression)", exCount)
	}
}
