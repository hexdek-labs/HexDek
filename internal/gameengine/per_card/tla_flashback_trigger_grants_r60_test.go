package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// tla_flashback_trigger_grants_r60_test.go — regression suite for the
// R60 single-target trigger-grant flashback family:
//
//   - Archmage's Newt (combat damage; saddle conditional cost)
//   - The Fugitive Doctor (attacks → sacrifice Clue → cost-override grant)
//   - Lost in Memories (Aura granting combat-damage trigger)
//
// Helpers (newGame, addPerm, newGraveyardCard) come from per_card_test.go
// and iroh_grand_lotus_test.go.

func newInstantOrSorceryInGraveyard(seat int, name, kind string, cmc int) *gameengine.Card {
	c := newGraveyardCard(name, seat, cmc)
	c.Types = []string{kind}
	return c
}

// -----------------------------------------------------------------------------
// Archmage's Newt
// -----------------------------------------------------------------------------

func TestArchmagesNewt_CombatDamageGrantsFlashbackAtManaCost(t *testing.T) {
	gs := newGame(t, 2)
	newt := addPerm(gs, 0, "Archmage's Newt", "creature", "salamander", "mount")

	gy := newInstantOrSorceryInGraveyard(0, "Lightning Bolt", "instant", 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   newt.Controller,
		"source_card":   newt.Card.DisplayName(),
		"defender_seat": 1,
		"amount":        2,
	})

	grant, ok := gs.ZoneCastGrants[gy]
	if !ok || grant == nil {
		t.Fatal("expected ZoneCastGrant registered for graveyard target")
	}
	if grant.ManaCost != 1 {
		t.Errorf("expected flashback cost = mana cost (1), got %d", grant.ManaCost)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("expected Duration=until_end_of_turn, got %q", grant.Duration)
	}
}

func TestArchmagesNewt_SaddledGrantsFlashbackZero(t *testing.T) {
	gs := newGame(t, 2)
	newt := addPerm(gs, 0, "Archmage's Newt", "creature", "salamander", "mount")
	newt.Flags["saddled"] = 1

	gy := newInstantOrSorceryInGraveyard(0, "Cruel Ultimatum", "sorcery", 7)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   newt.Controller,
		"source_card":   newt.Card.DisplayName(),
		"defender_seat": 1,
		"amount":        2,
	})

	grant := gs.ZoneCastGrants[gy]
	if grant == nil {
		t.Fatal("expected grant registered")
	}
	if grant.ManaCost != 0 {
		t.Errorf("saddled Newt should grant flashback {0}, got %d", grant.ManaCost)
	}
}

func TestArchmagesNewt_NoGrantWhenAnotherCreatureDealsDamage(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Archmage's Newt", "creature", "salamander")

	gy := newInstantOrSorceryInGraveyard(0, "Lightning Bolt", "instant", 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Grizzly Bears",
		"defender_seat": 1,
		"amount":        2,
	})

	if _, ok := gs.ZoneCastGrants[gy]; ok {
		t.Errorf("Newt should NOT grant flashback when a different creature damages")
	}
}

func TestArchmagesNewt_FailsGracefullyWithNoTarget(t *testing.T) {
	gs := newGame(t, 2)
	newt := addPerm(gs, 0, "Archmage's Newt", "creature", "salamander")

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   newt.Controller,
		"source_card":   newt.Card.DisplayName(),
		"defender_seat": 1,
		"amount":        2,
	})

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event with empty graveyard, events: %+v", gs.EventLog)
	}
}

// -----------------------------------------------------------------------------
// The Fugitive Doctor
// -----------------------------------------------------------------------------

func TestFugitiveDoctor_AttackSacrificesClueAndGrantsFlashback(t *testing.T) {
	gs := newGame(t, 2)
	doc := addPerm(gs, 0, "The Fugitive Doctor", "creature", "legendary", "time", "lord", "doctor")
	clue := addPerm(gs, 0, "Clue", "token", "artifact", "clue")

	gy := newInstantOrSorceryInGraveyard(0, "Mind's Desire", "sorcery", 6)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"attacker_perm": doc,
		"attacker_seat": doc.Controller,
		"attacker_card": doc.Card,
	})

	grant := gs.ZoneCastGrants[gy]
	if grant == nil {
		t.Fatal("expected grant registered after Fugitive Doctor attack + Clue sac")
	}
	if grant.ManaCost != 4 {
		t.Errorf("expected flashback cost {2}{R}{G} (4), got %d", grant.ManaCost)
	}

	// Clue should have been sacrificed → no longer on battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == clue {
			t.Errorf("Clue should have been sacrificed")
		}
	}
}

func TestFugitiveDoctor_NoClueNoGrant(t *testing.T) {
	gs := newGame(t, 2)
	doc := addPerm(gs, 0, "The Fugitive Doctor", "creature", "legendary")

	gy := newInstantOrSorceryInGraveyard(0, "Faithless Looting", "sorcery", 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"attacker_perm": doc,
		"attacker_seat": doc.Controller,
		"attacker_card": doc.Card,
	})

	if _, ok := gs.ZoneCastGrants[gy]; ok {
		t.Errorf("Fugitive Doctor must not grant flashback without a Clue to sacrifice")
	}
}

func TestFugitiveDoctor_DoesNotFireForOtherAttackers(t *testing.T) {
	gs := newGame(t, 2)
	doc := addPerm(gs, 0, "The Fugitive Doctor", "creature", "legendary")
	_ = doc
	other := addPerm(gs, 0, "Grizzly Bears", "creature")
	addPerm(gs, 0, "Clue", "token", "artifact", "clue")

	gy := newInstantOrSorceryInGraveyard(0, "Faithless Looting", "sorcery", 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": other.Controller,
		"attacker_card": other.Card,
	})

	if _, ok := gs.ZoneCastGrants[gy]; ok {
		t.Errorf("Fugitive Doctor must not fire when a different creature attacks")
	}
}

// -----------------------------------------------------------------------------
// Lost in Memories
// -----------------------------------------------------------------------------

func TestLostInMemories_EnchantedCreatureCombatDamageGrantsFlashback(t *testing.T) {
	gs := newGame(t, 2)
	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	aura := addPerm(gs, 0, "Lost in Memories", "enchantment", "aura")
	aura.AttachedTo = bear

	gy := newInstantOrSorceryInGraveyard(0, "Counterspell", "instant", 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   bear.Controller,
		"source_card":   bear.Card.DisplayName(),
		"defender_seat": 1,
		"amount":        3,
	})

	grant := gs.ZoneCastGrants[gy]
	if grant == nil {
		t.Fatal("expected Lost in Memories to grant flashback after enchanted creature deals damage")
	}
	if grant.ManaCost != 2 {
		t.Errorf("expected flashback cost = mana cost (2), got %d", grant.ManaCost)
	}
}

func TestLostInMemories_DoesNotFireForNonEnchantedAttacker(t *testing.T) {
	gs := newGame(t, 2)
	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	other := addPerm(gs, 0, "Llanowar Elves", "creature")
	aura := addPerm(gs, 0, "Lost in Memories", "enchantment", "aura")
	aura.AttachedTo = bear
	_ = other

	gy := newInstantOrSorceryInGraveyard(0, "Counterspell", "instant", 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	// Llanowar Elves (not enchanted) deals damage — Lost in Memories
	// must not grant.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   other.Controller,
		"source_card":   other.Card.DisplayName(),
		"defender_seat": 1,
		"amount":        1,
	})

	if _, ok := gs.ZoneCastGrants[gy]; ok {
		t.Errorf("Lost in Memories must only fire on the enchanted creature's combat damage")
	}
}

func TestLostInMemories_NoTargetSafeFail(t *testing.T) {
	gs := newGame(t, 2)
	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	aura := addPerm(gs, 0, "Lost in Memories", "enchantment", "aura")
	aura.AttachedTo = bear

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   bear.Controller,
		"source_card":   bear.Card.DisplayName(),
		"defender_seat": 1,
		"amount":        2,
	})

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when graveyard empty")
	}
}

// -----------------------------------------------------------------------------
// Primitive: GrantFlashbackUntilEOTWithCost
// -----------------------------------------------------------------------------

func TestGrantFlashbackUntilEOTWithCost_StampsOverrideCost(t *testing.T) {
	gs := newGame(t, 2)
	card := newInstantOrSorceryInGraveyard(0, "Worldfire", "sorcery", 8)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	gameengine.GrantFlashbackUntilEOTWithCost(gs, card, 0, "Test Source", 3)

	g := gs.ZoneCastGrants[card]
	if g == nil {
		t.Fatal("expected grant registered")
	}
	if g.ManaCost != 3 {
		t.Errorf("expected override cost 3, got %d", g.ManaCost)
	}
	if g.Keyword != "flashback" {
		t.Errorf("expected keyword 'flashback', got %q", g.Keyword)
	}
}
