package gameengine

import (
	"testing"
)

// TestZoneCastGrant_GameEndPurgesEOTGrants pins the Cruelclaw / Narset leak
// surfaced by the r60 fuzz: a heist or Narset-exile grant registered with
// `until_end_of_turn` during turn N has its EOT cleanup skipped when the
// game ends in mid-combat of turn N (the last opponent is eliminated by
// combat damage, the turn loop returns before reaching the cleanup step).
// CheckEnd must purge ZoneCastGrants when it flips Flags["ended"]=1 so the
// invariant can't fire on grants whose cleanup window never came.
func TestZoneCastGrant_GameEndPurgesEOTGrants(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	gs.Turn = 58
	exiled := &Card{Name: "Commander's Plate", Owner: 0}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, exiled)
	RegisterZoneCastGrant(gs, exiled, &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "free_exile_cast",
		ManaCost:          0,
		RequireController: 1,
		SourceName:        "The Infamous Cruelclaw",
		Duration:          "until_end_of_turn",
		GrantTurn:         58,
	})
	if _, ok := gs.ZoneCastGrants[exiled]; !ok {
		t.Fatal("setup failed: grant not registered")
	}

	// Seat 0 loses, leaving seat 1 as the only survivor — game ends.
	gs.Seats[0].Lost = true
	if !gs.CheckEnd() {
		t.Fatal("CheckEnd should fire with one living seat")
	}

	if _, ok := gs.ZoneCastGrants[exiled]; ok {
		t.Fatal("game-end should have purged the until_end_of_turn grant")
	}
	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Fatalf("ZoneCastGrantExpiry violation after game-end purge: %v", err)
	}
}

// TestZoneCastGrant_SourceLTBPurgesWhileSourceOnBfGrants pins the
// Huatli's Final Strike / Yawgmoth's Agenda leak: a graveyard-cast grant
// registered with `while_source_on_bf` survives past the source permanent
// leaving the battlefield because no LTB path calls ExpireSourceGrants.
// Destroying the source should drop every grant whose SourceTimestamp
// matches the source's Timestamp.
func TestZoneCastGrant_SourceLTBPurgesWhileSourceOnBfGrants(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	gs.Turn = 34
	src := addBattlefield(gs, 0, "Yawgmoth's Agenda", 0, 0, "enchantment")
	src.Timestamp = 107
	gyCard := &Card{Name: "Huatli's Final Strike", Owner: 0}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gyCard)
	RegisterZoneCastGrant(gs, gyCard, &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "play_from_graveyard",
		ManaCost:          -1,
		RequireController: 0,
		SourceName:        src.Card.DisplayName(),
		SourceTimestamp:   src.Timestamp,
		Duration:          "while_source_on_bf",
		GrantTurn:         34,
	})

	if _, ok := gs.ZoneCastGrants[gyCard]; !ok {
		t.Fatal("setup failed: grant not registered")
	}

	// Source dies. Any permanent-LTB path should drop grants sourced from
	// this Timestamp.
	if !DestroyPermanent(gs, src, nil) {
		t.Fatal("DestroyPermanent failed")
	}

	if _, ok := gs.ZoneCastGrants[gyCard]; ok {
		t.Fatal("source LTB should have purged the while_source_on_bf grant")
	}
	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Fatalf("ZoneCastGrantExpiry violation after source LTB: %v", err)
	}
}

// TestZoneCastGrant_SourceExilePurgesWhileSourceOnBfGrants covers the
// exile path (Disenchant on Yawgmoth's Agenda, Aether Gust on Narset,
// etc.) — same invariant as the destroy path.
func TestZoneCastGrant_SourceExilePurgesWhileSourceOnBfGrants(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	src := addBattlefield(gs, 0, "Maestros Ascendancy", 0, 0, "enchantment")
	src.Timestamp = 88
	gyCard := &Card{Name: "Some Spell", Owner: 0}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gyCard)
	RegisterZoneCastGrant(gs, gyCard, &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "once_per_turn_cast_from_graveyard",
		RequireController: 0,
		SourceName:        src.Card.DisplayName(),
		SourceTimestamp:   src.Timestamp,
		Duration:          "while_source_on_bf",
	})

	if !ExilePermanent(gs, src, nil) {
		t.Fatal("ExilePermanent failed")
	}
	if _, ok := gs.ZoneCastGrants[gyCard]; ok {
		t.Fatal("source exile should have purged the while_source_on_bf grant")
	}
}

// TestZoneCastGrant_SourceBouncePurgesWhileSourceOnBfGrants covers
// bounce (Cyclonic Rift, Boomerang) of a graveyard-cast-grant source.
func TestZoneCastGrant_SourceBouncePurgesWhileSourceOnBfGrants(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	src := addBattlefield(gs, 0, "Karador, Ghost Chieftain", 0, 0, "creature")
	src.Timestamp = 42
	gyCard := &Card{Name: "Reanimate", Owner: 0}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gyCard)
	RegisterZoneCastGrant(gs, gyCard, &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "once_per_turn_cast_from_graveyard",
		RequireController: 0,
		SourceName:        src.Card.DisplayName(),
		SourceTimestamp:   src.Timestamp,
		Duration:          "while_source_on_bf",
	})

	if !BouncePermanent(gs, src, nil, "hand") {
		t.Fatal("BouncePermanent failed")
	}
	if _, ok := gs.ZoneCastGrants[gyCard]; ok {
		t.Fatal("source bounce should have purged the while_source_on_bf grant")
	}
}

// TestZoneCastGrant_SacrificePurgesWhileSourceOnBfGrants covers
// sacrifice (the catch-all SBA / cost / effect path).
func TestZoneCastGrant_SacrificePurgesWhileSourceOnBfGrants(t *testing.T) {
	gs := newMultiplayerGame(t, 2)
	src := addBattlefield(gs, 0, "Lurrus of the Dream-Den", 0, 0, "creature")
	src.Timestamp = 19
	gyCard := &Card{Name: "Mox Diamond", Owner: 0}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gyCard)
	RegisterZoneCastGrant(gs, gyCard, &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "once_per_turn_cast_from_graveyard",
		RequireController: 0,
		SourceName:        src.Card.DisplayName(),
		SourceTimestamp:   src.Timestamp,
		Duration:          "while_source_on_bf",
	})

	SacrificePermanent(gs, src, "test")
	if _, ok := gs.ZoneCastGrants[gyCard]; ok {
		t.Fatal("source sacrifice should have purged the while_source_on_bf grant")
	}
}
