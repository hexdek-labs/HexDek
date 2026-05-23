package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 — Maestros Ascendancy verifies the once_per_turn_cast_from_
// graveyard primitive's sacrifice-additional-cost shape end-to-end.

func TestMaestrosAscendancy_ETBGrantsWithSacCreatureCost(t *testing.T) {
	gs := newGame(t, 2)
	maestros := addPerm(gs, 0, "Maestros Ascendancy", "enchantment")

	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	wrath := &gameengine.Card{Name: "Wrath of God", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt, wrath)

	maestrosAscendancyETB(gs, maestros)

	p := gameengine.GetZoneCastGrant(gs, bolt)
	if p == nil {
		t.Fatalf("ETB must grant instants in graveyard a cast permission")
	}
	if !p.ExileOnResolve {
		t.Fatalf("Maestros grants must be exile-on-resolve")
	}
	if !p.OncePerTurnPerSource || p.SourceTimestamp != maestros.Timestamp {
		t.Fatalf("Maestros grants must be once-per-turn pinned to this enchantment")
	}
	if len(p.AdditionalCosts) != 1 {
		t.Fatalf("Maestros grants must carry one additional cost; got %d", len(p.AdditionalCosts))
	}
	add := p.AdditionalCosts[0]
	if add.Kind != gameengine.AddCostKindSacrifice {
		t.Fatalf("additional cost must be a sacrifice; got %q", add.Kind)
	}
	if add.SacrificeFilter != "creature" {
		t.Fatalf("additional cost must filter to creatures; got %q", add.SacrificeFilter)
	}
}

func TestMaestrosAscendancy_DoesNotOverrideExistingGrant(t *testing.T) {
	// Maestros must not stomp a more specific grant like flashback or
	// Underworld Breach escape — those expose better terms (lower cost
	// or no sacrifice). Matches the "first grant wins" convention used
	// by Vivi/Underworld Breach.
	gs := newGame(t, 2)
	maestros := addPerm(gs, 0, "Maestros Ascendancy", "enchantment")
	bolt := &gameengine.Card{Name: "Faithless Looting", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	flashback := gameengine.NewFlashbackPermission(3)
	gameengine.RegisterZoneCastGrant(gs, bolt, flashback)

	maestrosAscendancyETB(gs, maestros)

	p := gameengine.GetZoneCastGrant(gs, bolt)
	if p == nil || p.Keyword != "flashback" {
		t.Fatalf("Maestros must not stomp a pre-existing more-specific grant; got %+v", p)
	}
}

func TestMaestrosAscendancy_GrantExpiresAfterSourceLeaves(t *testing.T) {
	// Engine EOT cleanup (ExpireZoneCastGrants) prunes
	// "while_source_on_bf" grants when no permanent with the source's
	// Timestamp is on the battlefield. Until then, the
	// oncePerTurnConsumed guard inside CastFromZone keeps stale grants
	// inert. This test exercises the cleanup path.
	gs := newGame(t, 2)
	maestros := addPerm(gs, 0, "Maestros Ascendancy", "enchantment")
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	maestrosAscendancyETB(gs, maestros)
	if gameengine.GetZoneCastGrant(gs, bolt) == nil {
		t.Fatal("precondition: grant should be present")
	}

	bf := gs.Seats[0].Battlefield[:0]
	for _, p := range gs.Seats[0].Battlefield {
		if p != maestros {
			bf = append(bf, p)
		}
	}
	gs.Seats[0].Battlefield = bf

	gameengine.ExpireZoneCastGrants(gs)
	if gameengine.GetZoneCastGrant(gs, bolt) != nil {
		t.Fatal("EOT cleanup must revoke grants whose source has left the battlefield")
	}
}

func TestMaestrosAscendancy_RegisteredOnGlobal(t *testing.T) {
	// Smoke test: zz_r60_register.go must wire Maestros into the global
	// registry so handler discovery (Kess/Karador/Hat) sees it.
	if !HasETB("Maestros Ascendancy") {
		t.Fatal("Maestros Ascendancy must have an ETB handler registered globally")
	}
	if !HasTrigger("Maestros Ascendancy", "zone_change") {
		t.Fatal("Maestros Ascendancy must have a zone_change handler to refresh grants on mid-turn mill")
	}
}
