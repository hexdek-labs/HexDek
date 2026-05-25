package per_card

// r60 follow-up — zone-cast grant expiry audit.
//
// The audit (see docs/zone-cast-grant-audit-r60.md) found five per_card
// grant call sites whose oracle text declared an expiry window but
// whose ZoneCastPermission left Duration / GrantTurn unset. The
// resulting grant was a silent leak: ExpireZoneCastGrants couldn't
// reclaim it (Duration mismatch in shouldExpireGrant's switch) and
// the ZoneCastGrantExpiry invariant couldn't flag it either
// (grantIsLeaked's switch matches the same Duration set).
//
// These regressions pin the fixed contract for each site: after the
// handler runs, the registered grant carries the canonical EOT-bound
// shape and a single ExpireZoneCastGrants pass at the cleanup of the
// grant turn reclaims it. The invariant agrees the post-cleanup state
// is clean.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// findGrantBySource scans gs.ZoneCastGrants for the first grant whose
// SourceName matches the given name and returns (card, grant). Used to
// avoid races with map-key inequality when the handler exiles a card
// we didn't directly construct.
func findGrantBySource(gs *gameengine.GameState, source string) (*gameengine.Card, *gameengine.ZoneCastPermission) {
	for c, g := range gs.ZoneCastGrants {
		if g != nil && g.SourceName == source {
			return c, g
		}
	}
	return nil, nil
}

// TestRagavan_ImpulseGrantExpiresAtEndOfTurn pins the highest-severity
// fix in the audit: Ragavan had no Duration AND no delayed-trigger
// cleanup, so the grant on an opponent's exiled top card persisted
// across turns. Verify the EOT contract.
func TestRagavan_ImpulseGrantExpiresAtEndOfTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 4
	ragavan := addPerm(gs, 0, "Ragavan, Nimble Pilferer", "creature")
	addLibrary(gs, 1, "Opp Top")

	ragavanCombatDamage(gs, ragavan, map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Ragavan, Nimble Pilferer",
		"defender_seat": 1,
	})

	card, grant := findGrantBySource(gs, "Ragavan, Nimble Pilferer")
	if grant == nil {
		t.Fatalf("Ragavan should register a zone-cast grant; grants=%+v", gs.ZoneCastGrants)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.GrantTurn != 4 {
		t.Errorf("GrantTurn: want 4, got %d", grant.GrantTurn)
	}
	if grant.RequireController != 0 {
		t.Errorf("RequireController: want 0 (Ragavan controller), got %d", grant.RequireController)
	}

	gameengine.ExpireZoneCastGrants(gs)
	if _, still := gs.ZoneCastGrants[card]; still {
		t.Errorf("Ragavan grant must be reclaimed at cleanup of its grant turn")
	}
}

// TestEmry_GraveyardGrantExpiresAtEndOfTurn — Emry's activated ability
// declares "you may cast it this turn". Without the Duration stamp,
// cleanup ran only via the delayed_trigger fallback, which can race
// with end-of-turn cleanup.
func TestEmry_GraveyardGrantExpiresAtEndOfTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 7
	emry := addPerm(gs, 0, "Emry, Lurker of the Loch", "creature")
	artifact := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, artifact)

	emryActivate(gs, emry, 0, nil)

	grant := gs.ZoneCastGrants[artifact]
	if grant == nil {
		t.Fatalf("Emry should register a graveyard cast grant on the artifact")
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.GrantTurn != 7 {
		t.Errorf("GrantTurn: want 7, got %d", grant.GrantTurn)
	}
	if grant.Zone != gameengine.ZoneGraveyard {
		t.Errorf("Zone: want graveyard, got %q", grant.Zone)
	}

	gameengine.ExpireZoneCastGrants(gs)
	if _, still := gs.ZoneCastGrants[artifact]; still {
		t.Errorf("Emry grant must be reclaimed at cleanup of its grant turn")
	}
}

// TestJeskasWill_ExileGrantExpiresAtEndOfTurn — oracle reads "Until end
// of turn, you may play those cards." All exiled cards should carry
// the EOT-bound grant.
func TestJeskasWill_ExileGrantExpiresAtEndOfTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	// Seed library so jeskasWill exiles real cards.
	addLibrary(gs, 0, "Top1", "Top2", "Top3", "Top4")
	// Put a commander permanent in play so the exile-mode branch fires.
	// Jeska's Will detects commander via CommanderNames matching a
	// battlefield card's DisplayName (case-insensitive).
	addPerm(gs, 0, "Korvold, Fae-Cursed King", "creature")
	gs.Seats[0].CommanderNames = []string{"Korvold, Fae-Cursed King"}

	will := addCard(gs, 0, "Jeska's Will", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: will}
	gameengine.InvokeResolveHook(gs, item)

	// Find at least one grant attributed to Jeska's Will.
	grantCount := 0
	for card, g := range gs.ZoneCastGrants {
		if g == nil || g.SourceName != "Jeska's Will" {
			continue
		}
		grantCount++
		if g.Duration != "until_end_of_turn" {
			t.Errorf("card %q: Duration want until_end_of_turn, got %q", card.DisplayName(), g.Duration)
		}
		if g.GrantTurn != 5 {
			t.Errorf("card %q: GrantTurn want 5, got %d", card.DisplayName(), g.GrantTurn)
		}
	}
	if grantCount == 0 {
		t.Fatalf("Jeska's Will should register at least one grant; grants=%+v", gs.ZoneCastGrants)
	}

	gameengine.ExpireZoneCastGrants(gs)
	for _, g := range gs.ZoneCastGrants {
		if g != nil && g.SourceName == "Jeska's Will" {
			t.Errorf("Jeska's Will grant survived cleanup of its grant turn")
		}
	}
}

// TestVivi_GraveyardGrantExpiresAtEndOfTurn — Vivi's combat-damage
// trigger grants a free cast from the controller's graveyard for the
// remainder of the turn. The delayed-trigger fallback existed already;
// this pin ensures the Duration stamp also reaps deterministically.
func TestVivi_GraveyardGrantExpiresAtEndOfTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 3
	vivi := addPerm(gs, 0, "Vivi Ornitier", "creature")
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	viviOrnitierCombatDamage(gs, vivi, map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Vivi Ornitier",
		"defender_seat": 1,
	})

	grant := gs.ZoneCastGrants[bolt]
	if grant == nil {
		t.Fatalf("Vivi should register a free-cast grant on the graveyard instant")
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.GrantTurn != 3 {
		t.Errorf("GrantTurn: want 3, got %d", grant.GrantTurn)
	}
	if grant.ManaCost != 0 {
		t.Errorf("ManaCost: want 0 (without paying), got %d", grant.ManaCost)
	}
	if !grant.ExileOnResolve {
		t.Errorf("ExileOnResolve: want true (cast-this-way → exile)")
	}

	gameengine.ExpireZoneCastGrants(gs)
	if _, still := gs.ZoneCastGrants[bolt]; still {
		t.Errorf("Vivi grant must be reclaimed at cleanup of its grant turn")
	}
}

// TestYasminKhan_GrantUsesCanonicalDurationString — pre-fix the field
// carried "until_next_end_step" which neither shouldExpireGrant nor
// grantIsLeaked recognised, making the reaper a silent no-op. The fix
// normalises to "until_end_of_turn" (Yasmin is sorcery-speed-only on
// her own turn so "your next end step" = current EOT).
func TestYasminKhan_GrantUsesCanonicalDurationString(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 6
	yasmin := addPerm(gs, 0, "Yasmin Khan", "creature")
	libTop := &gameengine.Card{Name: "Top Card", Owner: 0}
	gs.Seats[0].Library = []*gameengine.Card{libTop}

	yasminKhanActivate(gs, yasmin, 0, nil)

	grant := gs.ZoneCastGrants[libTop]
	if grant == nil {
		t.Fatalf("Yasmin Khan should register a may-play grant on the exiled card")
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("Duration: want until_end_of_turn (canonical), got %q", grant.Duration)
	}
	if grant.GrantTurn != 6 {
		t.Errorf("GrantTurn: want 6, got %d", grant.GrantTurn)
	}

	gameengine.ExpireZoneCastGrants(gs)
	if _, still := gs.ZoneCastGrants[libTop]; still {
		t.Errorf("Yasmin Khan grant must be reclaimed at cleanup of its grant turn")
	}
}
