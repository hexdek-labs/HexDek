package gameengine

import (
	"math/rand"
	"testing"
)

// Round-28b tests for ApplyManifestDread (CR §701.62). Covers the five
// required cases plus nil-safety and event-log shape.

func md_makeGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(62)), nil)
}

func md_makeCard(name string, isCreature bool) *Card {
	types := []string{"sorcery"}
	if isCreature {
		types = []string{"creature"}
	}
	return &Card{
		Name:          name,
		Types:         types,
		BasePower:     1,
		BaseToughness: 1,
	}
}

// libraryContains reports whether `seat.Library` still holds the given
// card pointer.
func md_libraryContains(seat *Seat, c *Card) bool {
	for _, x := range seat.Library {
		if x == c {
			return true
		}
	}
	return false
}

func md_graveyardContains(seat *Seat, c *Card) bool {
	for _, x := range seat.Graveyard {
		if x == c {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// (a) Top 2 revealed, caller picks 0 = first manifested + second milled.
// ---------------------------------------------------------------------------

func TestApplyManifestDread_PicksFirst(t *testing.T) {
	gs := md_makeGame(t)
	a := md_makeCard("Top A", true)
	b := md_makeCard("Second B", true)
	filler := md_makeCard("Filler C", false)
	gs.Seats[0].Library = []*Card{a, b, filler}
	for _, c := range gs.Seats[0].Library {
		c.Owner = 0
	}

	var sawCallback bool
	perm := ApplyManifestDread(gs, 0, func(top2 [2]*Card) int {
		sawCallback = true
		if top2[0] != a || top2[1] != b {
			t.Fatalf("callback saw wrong top2: %v / %v", top2[0], top2[1])
		}
		return 0
	})

	if !sawCallback {
		t.Fatal("callback should have been invoked when library has 2+ cards")
	}
	if perm == nil {
		t.Fatal("ApplyManifestDread should return a manifested permanent")
	}
	// a manifested → no longer in library, in battlefield as face-down.
	if md_libraryContains(gs.Seats[0], a) {
		t.Fatal("chosen card should be removed from library")
	}
	if !a.FaceDown {
		t.Fatal("chosen card should be FaceDown")
	}
	// b milled → not in library, in graveyard.
	if md_libraryContains(gs.Seats[0], b) {
		t.Fatal("unchosen card should be removed from library")
	}
	if !md_graveyardContains(gs.Seats[0], b) {
		t.Fatal("unchosen card should be in graveyard")
	}
	// filler still on top.
	if len(gs.Seats[0].Library) != 1 || gs.Seats[0].Library[0] != filler {
		t.Fatalf("filler should remain at top of library, got %v", gs.Seats[0].Library)
	}
}

// ---------------------------------------------------------------------------
// (b) Caller picks 1 = second manifested + first milled.
// ---------------------------------------------------------------------------

func TestApplyManifestDread_PicksSecond(t *testing.T) {
	gs := md_makeGame(t)
	a := md_makeCard("Top A", false)
	b := md_makeCard("Second B", true)
	filler := md_makeCard("Filler", false)
	gs.Seats[0].Library = []*Card{a, b, filler}

	perm := ApplyManifestDread(gs, 0, func(top2 [2]*Card) int { return 1 })
	if perm == nil {
		t.Fatal("expected manifested permanent")
	}
	// b manifested → out of library, face down.
	if md_libraryContains(gs.Seats[0], b) {
		t.Fatal("b should be removed from library")
	}
	if !b.FaceDown {
		t.Fatal("b should be FaceDown after manifest")
	}
	// a milled → out of library, in graveyard.
	if md_libraryContains(gs.Seats[0], a) {
		t.Fatal("a should be removed from library")
	}
	if !md_graveyardContains(gs.Seats[0], a) {
		t.Fatal("a should be in graveyard")
	}
}

// ---------------------------------------------------------------------------
// (c) Library too small (only 1 card) = manifest that 1, no mill.
// ---------------------------------------------------------------------------

func TestApplyManifestDread_LibraryOfOne(t *testing.T) {
	gs := md_makeGame(t)
	only := md_makeCard("Only Card", true)
	gs.Seats[0].Library = []*Card{only}

	called := false
	perm := ApplyManifestDread(gs, 0, func(top2 [2]*Card) int {
		called = true
		return 0
	})
	if called {
		t.Fatal("callback should NOT fire when library has exactly 1 card")
	}
	if perm == nil {
		t.Fatal("expected manifested permanent from the single card")
	}
	if md_libraryContains(gs.Seats[0], only) {
		t.Fatal("card should be removed from library after manifest")
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Fatalf("no mill expected in single-card library; got %d in graveyard",
			len(gs.Seats[0].Graveyard))
	}
	// no_mill flag should appear in the manifest_dread event.
	foundNoMill := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "manifest_dread" {
			if v, ok := ev.Details["no_mill"]; ok {
				if b, ok2 := v.(bool); ok2 && b {
					foundNoMill = true
				}
			}
		}
	}
	if !foundNoMill {
		t.Fatal("manifest_dread event should record no_mill=true for library-of-one")
	}
}

// ---------------------------------------------------------------------------
// (d) Library empty = no-op.
// ---------------------------------------------------------------------------

func TestApplyManifestDread_EmptyLibrary(t *testing.T) {
	gs := md_makeGame(t)
	gs.Seats[0].Library = nil
	called := false
	perm := ApplyManifestDread(gs, 0, func(top2 [2]*Card) int {
		called = true
		return 0
	})
	if perm != nil {
		t.Fatal("empty library should return nil permanent")
	}
	if called {
		t.Fatal("callback must not fire on empty library")
	}
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatalf("battlefield should remain empty; got %d perms",
			len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// (e) Manifested creature is 2/2 face-down per CR §701.40b.
// ---------------------------------------------------------------------------

func TestApplyManifestDread_ManifestedIs2_2FaceDown(t *testing.T) {
	gs := md_makeGame(t)
	a := md_makeCard("Top A", true) // creature underneath
	b := md_makeCard("Second B", false)
	gs.Seats[0].Library = []*Card{a, b}

	perm := ApplyManifestDread(gs, 0, func(top2 [2]*Card) int { return 0 })
	if perm == nil {
		t.Fatal("expected manifested permanent")
	}
	if perm.Card == nil {
		t.Fatal("perm should have a Card stub")
	}
	if !perm.Card.FaceDown {
		t.Fatal("manifested permanent must be face-down")
	}
	// Real-card overlay model: the real card is the permanent of record;
	// the 2/2 shape is the §707.2 effective characteristics from the
	// "manifest" template.
	if perm.Card != a {
		t.Fatal("chosen real card should be the permanent of record (perm.Card)")
	}
	if perm.Power() != 2 || perm.Toughness() != 2 {
		t.Fatalf("manifested permanent must be effective 2/2; got %d/%d",
			perm.Power(), perm.Toughness())
	}
	if perm.Flags == nil || perm.Flags["manifested"] != 1 {
		t.Fatal("perm should carry manifested=1 flag")
	}
	if perm.Flags["manifest_dread"] != 1 {
		t.Fatal("perm should carry manifest_dread=1 flag to distinguish from plain manifest")
	}
	// Real card is a creature → manifest_is_creature flag set.
	if perm.Flags["manifest_is_creature"] != 1 {
		t.Fatal("manifest_is_creature should be set when underlying card is a creature")
	}
	// Real-card model: the chosen card's identity lives on perm.Card, not
	// on a BackFaceAST stash.
	if perm.BackFaceAST != nil {
		t.Fatal("manifest-dread no longer stashes the real card on BackFaceAST")
	}
}

// ---------------------------------------------------------------------------
// Bonus: nil callback → defaults to picking index 0.
// ---------------------------------------------------------------------------

func TestApplyManifestDread_NilCallbackPicksFirst(t *testing.T) {
	gs := md_makeGame(t)
	a := md_makeCard("Top", true)
	b := md_makeCard("Second", true)
	gs.Seats[0].Library = []*Card{a, b}

	perm := ApplyManifestDread(gs, 0, nil)
	if perm == nil {
		t.Fatal("expected manifested permanent with nil callback")
	}
	if md_libraryContains(gs.Seats[0], a) {
		t.Fatal("nil callback should default to picking first (a); a should be manifested")
	}
	if !md_graveyardContains(gs.Seats[0], b) {
		t.Fatal("with nil callback, b should be milled")
	}
}

// Bonus: out-of-range callback choice defaults to 0 + logs warning.
func TestApplyManifestDread_OutOfRangeChoiceDefaultsToZero(t *testing.T) {
	gs := md_makeGame(t)
	a := md_makeCard("Top", true)
	b := md_makeCard("Second", true)
	gs.Seats[0].Library = []*Card{a, b}

	perm := ApplyManifestDread(gs, 0, func(top2 [2]*Card) int { return 99 })
	if perm == nil {
		t.Fatal("expected manifested permanent")
	}
	if md_libraryContains(gs.Seats[0], a) {
		t.Fatal("out-of-range choice should default to picking first (a)")
	}
	sawWarn := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "manifest_dread_invalid_choice" {
			sawWarn = true
			break
		}
	}
	if !sawWarn {
		t.Fatal("expected manifest_dread_invalid_choice event for out-of-range pick")
	}
}

// Bonus: nil-safety on game / seat.
func TestApplyManifestDread_NilSafe(t *testing.T) {
	if ApplyManifestDread(nil, 0, nil) != nil {
		t.Fatal("nil game should return nil")
	}
	gs := md_makeGame(t)
	if ApplyManifestDread(gs, -1, nil) != nil {
		t.Fatal("invalid seat should return nil")
	}
	if ApplyManifestDread(gs, 99, nil) != nil {
		t.Fatal("out-of-range seat should return nil")
	}
}
