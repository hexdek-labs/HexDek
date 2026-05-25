package gameengine

// R60 — once_per_turn_cast_from_graveyard primitive tests. Covers the
// shared gate (CanCastFromZone + CastFromZone) used by Kess Dissident
// Mage, Maestros Ascendancy, Karador, Lurrus, Gisa & Geralf, and the
// rest of the "Once during each of your turns, you may cast a [filter]
// spell from your graveyard" family. Per-card behavior is tested in
// per_card/*.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// installSourcePermanent mints a permanent with a known Timestamp on
// seat 0's battlefield so the OncePerTurnPerSource bookkeeping has
// somewhere to write its flag.
func installSourcePermanent(gs *GameState, name string) *Permanent {
	// Use an enchantment-typed card with positive toughness so SBAs
	// don't whisk it away after the first cast resolves.
	card := &Card{
		Name:          name,
		Owner:         0,
		Types:         []string{"enchantment"},
		BasePower:     2,
		BaseToughness: 2,
	}
	p := &Permanent{
		Card:       card,
		Controller: 0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	return p
}

// TestOncePerTurnGraveyardCast_FirstCastResolves verifies a plain
// "Kess-shaped" permission can cast one instant from the graveyard,
// exiles it on resolve, and stamps the source for the rest of the turn.
func TestOncePerTurnGraveyardCast_FirstCastResolves(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3
	src := installSourcePermanent(gs, "Kess, Dissident Mage")

	spell := addGraveyardCardWithEffect(gs, 0, "Lightning Bolt", 1,
		&gameast.Draw{Count: *gameast.NumInt(1)}, "instant")

	perm := NewOncePerTurnGraveyardCastPermission(0, "Kess, Dissident Mage",
		src.Timestamp, -1, true, nil)
	RegisterZoneCastGrant(gs, spell, perm)

	if got := CanCastFromZone(gs, 0, spell, ZoneGraveyard, []*ZoneCastPermission{perm}); got == nil {
		t.Fatal("first cast should be permitted")
	}

	_, err := CastFromZone(gs, 0, spell, ZoneGraveyard, perm, nil)
	if err != nil {
		t.Fatalf("CastFromZone: %v", err)
	}

	// Spell exiled per the "exile instead of graveyard" replacement.
	if !cardInZone(gs.Seats[0], spell, "exile") {
		t.Error("spell should be in exile after resolution")
	}
	if cardInZone(gs.Seats[0], spell, "graveyard") {
		t.Error("spell must not return to graveyard")
	}

	// Source stamped for the current turn.
	if got := src.Flags[permanentOncePerTurnCastFlag]; got != gs.Turn {
		t.Errorf("source not stamped with turn %d, got %d", gs.Turn, got)
	}
}

// TestOncePerTurnGraveyardCast_SecondCastBlocked drops two instants in
// the graveyard sharing one source. After the first cast the second
// must be rejected.
func TestOncePerTurnGraveyardCast_SecondCastBlocked(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10
	src := installSourcePermanent(gs, "Maestros Ascendancy")

	first := addGraveyardCardWithEffect(gs, 0, "Bolt A", 1,
		&gameast.Draw{Count: *gameast.NumInt(1)}, "instant")
	second := addGraveyardCardWithEffect(gs, 0, "Bolt B", 1,
		&gameast.Draw{Count: *gameast.NumInt(1)}, "instant")

	mkPerm := func() *ZoneCastPermission {
		return NewOncePerTurnGraveyardCastPermission(0, "Maestros Ascendancy",
			src.Timestamp, -1, true, nil)
	}
	pa := mkPerm()
	pb := mkPerm()
	RegisterZoneCastGrant(gs, first, pa)
	RegisterZoneCastGrant(gs, second, pb)

	if _, err := CastFromZone(gs, 0, first, ZoneGraveyard, pa, nil); err != nil {
		t.Fatalf("first cast failed: %v", err)
	}

	if got := CanCastFromZone(gs, 0, second, ZoneGraveyard, []*ZoneCastPermission{pb}); got != nil {
		t.Fatal("second cast must be blocked by once-per-turn budget")
	}

	_, err := CastFromZone(gs, 0, second, ZoneGraveyard, pb, nil)
	if err == nil {
		t.Fatal("CastFromZone should refuse second cast through same source")
	}
	if ce, ok := err.(*CastError); !ok || ce.Reason != "once_per_turn_consumed" {
		t.Errorf("expected once_per_turn_consumed CastError, got %v", err)
	}
}

// TestOncePerTurnGraveyardCast_ResetsNextTurn confirms the gate is keyed
// on the turn number — when gs.Turn advances, a new cast becomes legal
// even with the source's stale flag still in place.
func TestOncePerTurnGraveyardCast_ResetsNextTurn(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10
	addLibrary(gs, 0, "Lib1", "Lib2", "Lib3", "Lib4")
	src := installSourcePermanent(gs, "Karador, Ghost Chieftain")

	first := addGraveyardCardWithEffect(gs, 0, "Bolt A", 1,
		&gameast.Draw{Count: *gameast.NumInt(1)}, "instant")
	pa := NewOncePerTurnGraveyardCastPermission(0, "Karador, Ghost Chieftain",
		src.Timestamp, -1, true, nil)
	RegisterZoneCastGrant(gs, first, pa)

	if _, err := CastFromZone(gs, 0, first, ZoneGraveyard, pa, nil); err != nil {
		t.Fatalf("first cast: %v", err)
	}

	// Advance the turn; gate should re-open.
	gs.Turn++
	second := addGraveyardCardWithEffect(gs, 0, "Bolt B", 1,
		&gameast.Draw{Count: *gameast.NumInt(1)}, "instant")
	pb := NewOncePerTurnGraveyardCastPermission(0, "Karador, Ghost Chieftain",
		src.Timestamp, -1, true, nil)
	RegisterZoneCastGrant(gs, second, pb)
	gs.Seats[0].ManaPool = 5

	if got := CanCastFromZone(gs, 0, second, ZoneGraveyard, []*ZoneCastPermission{pb}); got == nil {
		t.Fatal("after turn advance the second cast should be permitted")
	}
}

// TestOncePerTurnGraveyardCast_SourceGone returns "consumed" when the
// granting permanent has left the battlefield. The grant lifecycle
// (ExpireSourceGrants on LTB, plus EOT cleanup) will eventually delete
// the entry; this guard is a safety net for the in-between window.
func TestOncePerTurnGraveyardCast_SourceGone(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	src := installSourcePermanent(gs, "Kess, Dissident Mage")
	missingTS := src.Timestamp + 999 // no permanent has this stamp

	spell := addGraveyardCard(gs, 0, "Lightning Bolt", 1, "instant")
	p := NewOncePerTurnGraveyardCastPermission(0, "Kess, Dissident Mage",
		missingTS, -1, true, nil)
	RegisterZoneCastGrant(gs, spell, p)

	if got := CanCastFromZone(gs, 0, spell, ZoneGraveyard, []*ZoneCastPermission{p}); got != nil {
		t.Fatal("missing source permanent should make grant unusable")
	}
}

// TestOncePerTurnGraveyardCast_MultipleSourcesEachGetOne models two
// Kesses out at once (or Kess + Maestros). Each owns its own
// SourceTimestamp, so each should be allowed exactly one cast per turn.
func TestOncePerTurnGraveyardCast_MultipleSourcesEachGetOne(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10
	addLibrary(gs, 0, "Lib1", "Lib2", "Lib3", "Lib4")
	srcA := installSourcePermanent(gs, "Kess, Dissident Mage")
	srcB := installSourcePermanent(gs, "Maestros Ascendancy")

	first := addGraveyardCardWithEffect(gs, 0, "Bolt A", 1,
		&gameast.Draw{Count: *gameast.NumInt(1)}, "instant")
	second := addGraveyardCardWithEffect(gs, 0, "Bolt B", 1,
		&gameast.Draw{Count: *gameast.NumInt(1)}, "instant")

	pa := NewOncePerTurnGraveyardCastPermission(0, "Kess, Dissident Mage",
		srcA.Timestamp, -1, true, nil)
	pb := NewOncePerTurnGraveyardCastPermission(0, "Maestros Ascendancy",
		srcB.Timestamp, -1, true, nil)

	if _, err := CastFromZone(gs, 0, first, ZoneGraveyard, pa, nil); err != nil {
		t.Fatalf("cast via Kess: %v", err)
	}
	// Maestros's budget is untouched.
	if got := CanCastFromZone(gs, 0, second, ZoneGraveyard, []*ZoneCastPermission{pb}); got == nil {
		t.Fatal("Maestros should still grant its first cast of the turn")
	}
	if _, err := CastFromZone(gs, 0, second, ZoneGraveyard, pb, nil); err != nil {
		t.Fatalf("cast via Maestros: %v", err)
	}
}

