package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// tla_flashback_single_target_r60_test.go — regressions for the
// remaining single-target trigger-grant family: Snapcaster Mage,
// Sphinx of Forgotten Lore, Slickshot Lockpicker, Katilda and Lier.
//
// All four reuse the GrantFlashbackUntilEOT primitive; coverage focuses
// on (a) the trigger fires and the grant is stamped, (b) the grant
// expires at EOT (covered by an end-to-end Snapcaster test that drives
// ExpireZoneCastGrants), (c) trigger gates work — Sphinx only fires when
// it itself attacks, Katilda only on Human-subtype caster's spells.

// -----------------------------------------------------------------------------
// Snapcaster Mage
// -----------------------------------------------------------------------------

func TestSnapcasterMage_ETBGrantsFlashbackToInstantInGraveyard(t *testing.T) {
	gs := newGame(t, 2)

	gy := newInstantOrSorceryInGraveyard(0, "Counterspell", "instant", 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	snap := addPerm(gs, 0, "Snapcaster Mage", "creature", "human", "wizard")
	gameengine.InvokeETBHook(gs, snap)

	grant := gs.ZoneCastGrants[gy]
	if grant == nil {
		t.Fatal("expected ZoneCastGrant registered after Snapcaster ETB")
	}
	if grant.Keyword != "flashback" {
		t.Errorf("expected Keyword=flashback, got %q", grant.Keyword)
	}
	if grant.ManaCost != 2 {
		t.Errorf("expected flashback cost = 2, got %d", grant.ManaCost)
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("expected EOT duration, got %q", grant.Duration)
	}
}

func TestSnapcasterMage_GrantExpiresAtEndOfTurnCleanup(t *testing.T) {
	gs := newGame(t, 2)
	gy := newInstantOrSorceryInGraveyard(0, "Lightning Bolt", "instant", 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	snap := addPerm(gs, 0, "Snapcaster Mage", "creature", "human", "wizard")
	gameengine.InvokeETBHook(gs, snap)

	if gs.ZoneCastGrants[gy] == nil {
		t.Fatalf("setup: expected grant registered")
	}

	gameengine.ExpireZoneCastGrants(gs)

	if gs.ZoneCastGrants[gy] != nil {
		t.Errorf("expected Snapcaster grant flushed by ExpireZoneCastGrants, still present")
	}
}

func TestSnapcasterMage_NoGrantWithEmptyGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	snap := addPerm(gs, 0, "Snapcaster Mage", "creature", "human", "wizard")
	gameengine.InvokeETBHook(gs, snap)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event on empty graveyard")
	}
}

// -----------------------------------------------------------------------------
// Sphinx of Forgotten Lore
// -----------------------------------------------------------------------------

func TestSphinxOfForgottenLore_AttackGrantsFlashback(t *testing.T) {
	gs := newGame(t, 2)
	sphinx := addPerm(gs, 0, "Sphinx of Forgotten Lore", "creature", "sphinx")

	gy := newInstantOrSorceryInGraveyard(0, "Mind's Desire", "sorcery", 6)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"attacker_perm": sphinx,
		"attacker_seat": sphinx.Controller,
		"attacker_card": sphinx.Card,
	})

	grant := gs.ZoneCastGrants[gy]
	if grant == nil {
		t.Fatal("expected Sphinx of Forgotten Lore to grant flashback on attack")
	}
	if grant.ManaCost != 6 {
		t.Errorf("expected flashback cost = 6 (mana cost), got %d", grant.ManaCost)
	}
}

func TestSphinxOfForgottenLore_DoesNotFireForOtherAttackers(t *testing.T) {
	gs := newGame(t, 2)
	sphinx := addPerm(gs, 0, "Sphinx of Forgotten Lore", "creature", "sphinx")
	_ = sphinx
	other := addPerm(gs, 0, "Grizzly Bears", "creature")

	gy := newInstantOrSorceryInGraveyard(0, "Lightning Bolt", "instant", 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	gameengine.FireCardTrigger(gs, "attacks", map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": other.Controller,
		"attacker_card": other.Card,
	})

	if _, ok := gs.ZoneCastGrants[gy]; ok {
		t.Errorf("Sphinx must only fire when it itself attacks")
	}
}

// -----------------------------------------------------------------------------
// Slickshot Lockpicker
// -----------------------------------------------------------------------------

func TestSlickshotLockpicker_ETBGrantsFlashback(t *testing.T) {
	gs := newGame(t, 2)

	gy := newInstantOrSorceryInGraveyard(0, "Brainstorm", "instant", 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	slick := addPerm(gs, 0, "Slickshot Lockpicker", "creature", "human", "rogue")
	gameengine.InvokeETBHook(gs, slick)

	grant := gs.ZoneCastGrants[gy]
	if grant == nil {
		t.Fatal("expected Slickshot Lockpicker ETB to grant flashback")
	}
	if grant.ManaCost != 1 {
		t.Errorf("expected flashback cost = 1, got %d", grant.ManaCost)
	}
}

// -----------------------------------------------------------------------------
// Katilda and Lier
// -----------------------------------------------------------------------------

func TestKatildaAndLier_HumanSpellCastGrantsFlashback(t *testing.T) {
	gs := newGame(t, 2)
	katilda := addPerm(gs, 0, "Katilda and Lier", "creature", "legendary", "human")

	gy := newInstantOrSorceryInGraveyard(0, "Counterspell", "instant", 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	humanSpell := &gameengine.Card{
		Name:  "Champion of the Parish",
		Owner: 0,
		Types: []string{"creature", "human", "soldier"},
		CMC:   1,
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": katilda.Controller,
		"card":        humanSpell,
		"is_creature": true,
		"spell_name":  humanSpell.DisplayName(),
	})

	grant := gs.ZoneCastGrants[gy]
	if grant == nil {
		t.Fatal("expected Katilda and Lier to grant flashback after a Human spell cast")
	}
	if grant.ManaCost != 2 {
		t.Errorf("expected flashback cost = 2, got %d", grant.ManaCost)
	}
}

func TestKatildaAndLier_DoesNotFireForNonHumanCasts(t *testing.T) {
	gs := newGame(t, 2)
	katilda := addPerm(gs, 0, "Katilda and Lier", "creature", "legendary", "human")
	_ = katilda

	gy := newInstantOrSorceryInGraveyard(0, "Counterspell", "instant", 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	elfSpell := &gameengine.Card{
		Name:  "Llanowar Elves",
		Owner: 0,
		Types: []string{"creature", "elf", "druid"},
		CMC:   1,
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        elfSpell,
		"is_creature": true,
		"spell_name":  elfSpell.DisplayName(),
	})

	if _, ok := gs.ZoneCastGrants[gy]; ok {
		t.Errorf("Katilda and Lier must not grant flashback on non-Human spells")
	}
}

func TestKatildaAndLier_DoesNotFireForOpponentCasts(t *testing.T) {
	gs := newGame(t, 2)
	katilda := addPerm(gs, 0, "Katilda and Lier", "creature", "legendary", "human")
	_ = katilda

	gy := newInstantOrSorceryInGraveyard(0, "Counterspell", "instant", 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, gy)

	humanSpell := &gameengine.Card{
		Name:  "Thraben Inspector",
		Owner: 1,
		Types: []string{"creature", "human", "soldier"},
		CMC:   1,
	}
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"card":        humanSpell,
		"is_creature": true,
		"spell_name":  humanSpell.DisplayName(),
	})

	if _, ok := gs.ZoneCastGrants[gy]; ok {
		t.Errorf("Katilda and Lier must not fire on opponent's Human spell casts")
	}
}
