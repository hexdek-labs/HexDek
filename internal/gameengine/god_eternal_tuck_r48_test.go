package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Regression tests for the god_eternal_tuck handler (resolve_helpers.go),
// pinning the R48 fix for the CardIdentity zone-leak surfaced by Loki r47
// game 333 / seed 3330043 (God-Eternal Oketra appearing in both seat 2
// library and seat 2 graveyard).
//
// Bug shape: God-Eternal Oketra's dies/exile trigger ("put it third from
// the top of its owner's library") used to call removePermanent on a
// Permanent that had already been moved to the graveyard by SBA. The
// removePermanent call no-op'd, and src.Card was inserted into the
// library at index 2 — leaving the same *Card in both graveyard and
// library. Same pattern Dread and Adric exhibited (closed 2026-05-08).
//
// Fix: fall through to remove the card from graveyard / exile / hand if
// removePermanent doesn't find it on the battlefield, then insert.

func newOketraGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(48))
	return NewGameState(2, rng, nil)
}

func oketraSeedLibrary(gs *GameState, seat int, names ...string) {
	for _, n := range names {
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, &Card{Name: n, Owner: seat})
	}
}

// godEternalTuckMod returns a ModificationEffect with the right ModKind.
func godEternalTuckMod() *gameast.ModificationEffect {
	return &gameast.ModificationEffect{ModKind: "god_eternal_tuck"}
}

func TestGodEternalTuck_FromBattlefieldGoesToLibraryThirdFromTop(t *testing.T) {
	gs := newOketraGame(t)
	oketra := &Card{Name: "God-Eternal Oketra", Owner: 0, Types: []string{"creature", "legendary"}}
	perm := &Permanent{
		Card:       oketra,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	oketraSeedLibrary(gs, 0, "Top1", "Top2", "Top3", "Top4")

	resolveModificationEffect(gs, perm, godEternalTuckMod())

	if len(gs.Seats[0].Battlefield) != 0 {
		t.Errorf("expected battlefield empty after tuck, got %d", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Library) != 5 {
		t.Fatalf("expected library size 5, got %d", len(gs.Seats[0].Library))
	}
	if gs.Seats[0].Library[2] != oketra {
		t.Errorf("expected Oketra at index 2 (third from top); got %q", gs.Seats[0].Library[2].DisplayName())
	}
	// Should not appear in graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == oketra {
			t.Errorf("Oketra leaked into graveyard from battlefield path")
		}
	}
}

func TestGodEternalTuck_FromGraveyardDoesNotDuplicate(t *testing.T) {
	gs := newOketraGame(t)
	oketra := &Card{Name: "God-Eternal Oketra", Owner: 0, Types: []string{"creature", "legendary"}}
	// Simulate the real-game state at trigger-resolve time: Oketra has
	// already died and is in the graveyard. The Permanent pointer is the
	// stale ex-battlefield reference the trigger carries.
	perm := &Permanent{
		Card:       oketra,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, oketra)
	oketraSeedLibrary(gs, 0, "Top1", "Top2", "Top3")

	resolveModificationEffect(gs, perm, godEternalTuckMod())

	// The card must be removed from the graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == oketra {
			t.Errorf("CardIdentity leak: Oketra still in graveyard after tuck")
		}
	}
	// Library should now contain Oketra at index 2 (third from top).
	if len(gs.Seats[0].Library) != 4 {
		t.Fatalf("expected library size 4 after tuck, got %d", len(gs.Seats[0].Library))
	}
	if gs.Seats[0].Library[2] != oketra {
		t.Errorf("expected Oketra at index 2; got %q", gs.Seats[0].Library[2].DisplayName())
	}
	// And NOT anywhere else in the library.
	for i, c := range gs.Seats[0].Library {
		if i != 2 && c == oketra {
			t.Errorf("Oketra duplicated at library index %d", i)
		}
	}
}

func TestGodEternalTuck_FromExileDoesNotDuplicate(t *testing.T) {
	gs := newOketraGame(t)
	oketra := &Card{Name: "God-Eternal Oketra", Owner: 0, Types: []string{"creature", "legendary"}}
	// Oracle covers exile too: "When God-Eternal Oketra dies or is put
	// into exile from anywhere, you may put it into your owner's library
	// third from the top."
	perm := &Permanent{
		Card:       oketra,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, oketra)
	oketraSeedLibrary(gs, 0, "Top1", "Top2")

	resolveModificationEffect(gs, perm, godEternalTuckMod())

	for _, c := range gs.Seats[0].Exile {
		if c == oketra {
			t.Errorf("CardIdentity leak: Oketra still in exile after tuck")
		}
	}
	// Library was size 2 < 3 → insert at end (index 2).
	if got := len(gs.Seats[0].Library); got != 3 {
		t.Fatalf("expected library size 3, got %d", got)
	}
	if gs.Seats[0].Library[2] != oketra {
		t.Errorf("expected Oketra at index 2 (end of small library)")
	}
}

func TestGodEternalTuck_ShortLibraryClampsToEnd(t *testing.T) {
	// When the library has fewer than 2 cards, "third from the top"
	// resolves to "end of library" via the clamp.
	gs := newOketraGame(t)
	oketra := &Card{Name: "God-Eternal Oketra", Owner: 0, Types: []string{"creature", "legendary"}}
	perm := &Permanent{
		Card:       oketra,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	oketraSeedLibrary(gs, 0, "Sole")

	resolveModificationEffect(gs, perm, godEternalTuckMod())

	if got := len(gs.Seats[0].Library); got != 2 {
		t.Fatalf("expected library size 2, got %d", got)
	}
	if gs.Seats[0].Library[1] != oketra {
		t.Errorf("expected Oketra clamped to library tail; got %q", gs.Seats[0].Library[1].DisplayName())
	}
}

func TestGodEternalTuck_EmptyLibraryStillInserts(t *testing.T) {
	gs := newOketraGame(t)
	oketra := &Card{Name: "God-Eternal Oketra", Owner: 0, Types: []string{"creature", "legendary"}}
	perm := &Permanent{
		Card:       oketra,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, oketra)

	resolveModificationEffect(gs, perm, godEternalTuckMod())

	if got := len(gs.Seats[0].Library); got != 1 {
		t.Fatalf("expected library size 1 (Oketra only), got %d", got)
	}
	if gs.Seats[0].Library[0] != oketra {
		t.Errorf("expected Oketra at library[0]")
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected graveyard cleared, got %d", len(gs.Seats[0].Graveyard))
	}
}

// TestGodEternalTuck_R56_FromCommandZoneDoesNotDuplicate pins the R56 fix
// for Loki r55 game-3458 / God-Eternal Bontu library↔command_zone leak.
//
// Bug shape (pre-r56): when a God-Eternal is the controller's commander
// and dies, CR §903.9b redirects the death to command_zone instead of
// graveyard. The God-Eternal's own "die or be exiled → owner's library
// third from top" trigger fires AFTER the §903.9b redirect, so by
// resolve time the card is in command_zone. The handler's fallthrough
// scanned graveyard / exile / hand but NOT command_zone — the card was
// inserted into the library without being removed from command_zone,
// duplicating the *Card across both zones.
//
// Fix: extend the fallthrough scan to include command_zone.
func TestGodEternalTuck_R56_FromCommandZoneDoesNotDuplicate(t *testing.T) {
	gs := newOketraGame(t)
	bontu := &Card{Name: "God-Eternal Bontu", Owner: 0, Types: []string{"creature", "legendary"}}
	// Simulate the real-game state: Bontu died as commander, §903.9b
	// redirected to command_zone. The Permanent pointer is the stale
	// ex-battlefield reference the trigger carries.
	perm := &Permanent{
		Card:       bontu,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, bontu)
	oketraSeedLibrary(gs, 0, "Top1", "Top2", "Top3")

	resolveModificationEffect(gs, perm, godEternalTuckMod())

	// The card must be removed from the command_zone.
	for _, c := range gs.Seats[0].CommandZone {
		if c == bontu {
			t.Errorf("CardIdentity leak: Bontu still in command_zone after tuck (r56 regression)")
		}
	}
	// Library should now contain Bontu at index 2 (third from top).
	if len(gs.Seats[0].Library) != 4 {
		t.Fatalf("expected library size 4 after tuck, got %d", len(gs.Seats[0].Library))
	}
	if gs.Seats[0].Library[2] != bontu {
		t.Errorf("expected Bontu at index 2; got %q", gs.Seats[0].Library[2].DisplayName())
	}
	// And NOT anywhere else in the library.
	for i, c := range gs.Seats[0].Library {
		if i != 2 && c == bontu {
			t.Errorf("Bontu duplicated at library index %d", i)
		}
	}
}

// TestGodEternalTuck_R56_CommandZoneOnOpponentSeat covers the case where
// the commander's command_zone instance lives on a different seat index
// from the perm.Controller (e.g., a §903.9b redirect on a stolen Bontu —
// the §903.9b redirect goes to the OWNER's command zone, not the
// controller's). The fallthrough loops over every seat, so the scan
// finds the card regardless of which seat hosts the command_zone entry.
func TestGodEternalTuck_R56_CommandZoneOnOwnerNotController(t *testing.T) {
	gs := newOketraGame(t)
	bontu := &Card{Name: "God-Eternal Bontu", Owner: 1, Types: []string{"creature", "legendary"}}
	// Bontu was stolen by seat 0 (controller=0), but owner=1; §903.9b
	// redirects to seat 1's command zone.
	perm := &Permanent{
		Card:       bontu,
		Controller: 0,
		Owner:      1,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[1].CommandZone = append(gs.Seats[1].CommandZone, bontu)
	oketraSeedLibrary(gs, 1, "Top1", "Top2", "Top3")

	resolveModificationEffect(gs, perm, godEternalTuckMod())

	// Bontu must be removed from seat 1's command_zone.
	for _, c := range gs.Seats[1].CommandZone {
		if c == bontu {
			t.Errorf("CardIdentity leak: Bontu still in seat 1 command_zone after tuck")
		}
	}
	// And must end up in OWNER's (seat 1) library at index 2.
	if len(gs.Seats[1].Library) != 4 {
		t.Fatalf("expected seat 1 library size 4, got %d", len(gs.Seats[1].Library))
	}
	if gs.Seats[1].Library[2] != bontu {
		t.Errorf("expected Bontu at seat 1 library index 2; got %q", gs.Seats[1].Library[2].DisplayName())
	}
}
