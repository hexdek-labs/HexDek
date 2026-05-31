package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestWrexial_CombatDamageRegistersZoneCastGrant(t *testing.T) {
	gs := newGame(t, 2)
	wrexial := addPerm(gs, 0, "Wrexial, the Risen Deep", "creature", "kraken")
	instant := newCardWithTypes("Counterspell", 1, "instant", "cmc:2")
	sorcery := newCardWithTypes("Wrath of God", 1, "sorcery", "cmc:4")
	gs.Seats[1].Graveyard = []*gameengine.Card{instant, sorcery}

	wrexialCombatDamage(gs, wrexial, map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Wrexial, the Risen Deep",
		"defender_seat": 1,
		"amount":        5,
	})

	if gs.ZoneCastGrants == nil || gs.ZoneCastGrants[sorcery] == nil {
		t.Fatalf("expected Wrath of God to have a ZoneCastGrant (highest-CMC pick)")
	}
	grant := gs.ZoneCastGrants[sorcery]
	if grant.RequireController != 0 {
		t.Errorf("grant must be restricted to Wrexial's controller (0), got %d", grant.RequireController)
	}
	if !grant.ExileOnResolve {
		t.Errorf("grant must exile-on-resolve per oracle text")
	}
	if grant.ManaCost != 0 {
		t.Errorf("grant must be free-cast (ManaCost 0), got %d", grant.ManaCost)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("grant must have EOT duration for cleanup, got %q", grant.Duration)
	}
}

func TestWrexial_DoesNotFireOnOpponentCombat(t *testing.T) {
	gs := newGame(t, 2)
	wrexial := addPerm(gs, 0, "Wrexial, the Risen Deep", "creature")
	instant := newCardWithTypes("Counterspell", 1, "instant")
	gs.Seats[1].Graveyard = []*gameengine.Card{instant}
	wrexialCombatDamage(gs, wrexial, map[string]interface{}{
		"source_seat":   1,
		"source_card":   "Some Opp Creature",
		"defender_seat": 0,
	})
	if gs.ZoneCastGrants != nil && gs.ZoneCastGrants[instant] != nil {
		t.Errorf("Wrexial must not fire on opp combat damage")
	}
}

func TestWrexial_NoopWithNoInstantOrSorceryInOppGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	wrexial := addPerm(gs, 0, "Wrexial, the Risen Deep", "creature")
	gs.Seats[1].Graveyard = []*gameengine.Card{
		newCardWithTypes("Llanowar Elves", 1, "creature"),
	}
	wrexialCombatDamage(gs, wrexial, map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Wrexial, the Risen Deep",
		"defender_seat": 1,
	})
	if gs.ZoneCastGrants != nil && len(gs.ZoneCastGrants) > 0 {
		t.Errorf("no grant should be registered when no instant/sorcery available")
	}
}

func TestWrexial_PicksHighestCMC(t *testing.T) {
	gs := newGame(t, 2)
	wrexial := addPerm(gs, 0, "Wrexial, the Risen Deep", "creature")
	small := newCardWithTypes("Lightning Bolt", 1, "instant", "cmc:1")
	big := newCardWithTypes("Cruel Ultimatum", 1, "sorcery", "cmc:7")
	gs.Seats[1].Graveyard = []*gameengine.Card{small, big}
	wrexialCombatDamage(gs, wrexial, map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Wrexial, the Risen Deep",
		"defender_seat": 1,
	})
	if gs.ZoneCastGrants[big] == nil {
		t.Errorf("expected Cruel Ultimatum (CMC 7) picked over Lightning Bolt")
	}
	if gs.ZoneCastGrants[small] != nil {
		t.Errorf("Lightning Bolt should NOT have a grant")
	}
}
