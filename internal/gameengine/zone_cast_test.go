package gameengine

// Wave 2 tests — cast-from-other-zones primitive.
//
// Tests the zone_cast.go infrastructure: flashback from graveyard,
// escape (Underworld Breach), cast from exile, cast from library top.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Helper: put a card in graveyard.
// ---------------------------------------------------------------------------

func addGraveyardCard(gs *GameState, seat int, name string, cost int, types ...string) *Card {
	t := []string{}
	t = append(t, types...)
	if cost > 0 {
		t = append(t, "cost:"+itoa(cost))
	}
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: t,
		CMC:   cost,
	}
	gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, c)
	return c
}

func addGraveyardCardWithEffect(gs *GameState, seat int, name string, cost int, eff gameast.Effect, types ...string) *Card {
	c := addGraveyardCard(gs, seat, name, cost, types...)
	ast := &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Activated{Effect: eff},
		},
	}
	c.AST = ast
	return c
}

// Helper: put a card in exile zone.
func addExileCard(gs *GameState, seat int, name string, cost int, types ...string) *Card {
	t := []string{}
	t = append(t, types...)
	if cost > 0 {
		t = append(t, "cost:"+itoa(cost))
	}
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: t,
		CMC:   cost,
	}
	gs.Seats[seat].Exile = append(gs.Seats[seat].Exile, c)
	return c
}

// ---------------------------------------------------------------------------
// 1. Flashback — cast from graveyard, exiled after resolution
// ---------------------------------------------------------------------------

func TestZoneCast_Flashback_CastFromGraveyard(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3

	// Put a flashback spell in graveyard.
	fb := addGraveyardCardWithEffect(gs, 0, "Faithless Looting", 1,
		&gameast.Draw{Count: *gameast.NumInt(2)}, "sorcery")

	// Flashback costs 3 (different from normal cost of 1).
	perm := NewFlashbackPermission(3)

	// Verify it's castable.
	found := CanCastFromZone(gs, 0, fb, ZoneGraveyard, []*ZoneCastPermission{perm})
	if found == nil {
		t.Fatal("flashback should be castable from graveyard with 3 mana")
	}

	// Cast it.
	result, err := CastFromZone(gs, 0, fb, ZoneGraveyard, perm, nil)
	if err != nil {
		t.Fatalf("CastFromZone failed: %v", err)
	}
	_ = result

	// Mana should be 0 (paid 3).
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana 0, got %d", gs.Seats[0].ManaPool)
	}

	// Spell should NOT be in graveyard after resolution — it should be exiled.
	fbInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == fb {
			fbInGY = true
			break
		}
	}
	if fbInGY {
		t.Error("flashback spell should NOT be in graveyard after resolution")
	}

	// Should be in exile.
	fbInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == fb {
			fbInExile = true
			break
		}
	}
	if !fbInExile {
		t.Error("flashback spell should be in exile after resolution")
	}

	// Check for the resolve event with exile destination.
	foundExileResolve := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "resolve" {
			if det, ok := ev.Details["to"]; ok && det == "exile" {
				foundExileResolve = true
				break
			}
		}
	}
	if !foundExileResolve {
		t.Error("expected a resolve event with to=exile for flashback")
	}
}

// ---------------------------------------------------------------------------
// 2. Flashback — cannot cast without enough mana
// ---------------------------------------------------------------------------

func TestZoneCast_Flashback_InsufficientMana(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1 // flashback costs 3

	fb := addGraveyardCard(gs, 0, "Faithless Looting", 1, "sorcery")
	perm := NewFlashbackPermission(3)

	found := CanCastFromZone(gs, 0, fb, ZoneGraveyard, []*ZoneCastPermission{perm})
	if found != nil {
		t.Fatal("flashback should NOT be castable with only 1 mana (costs 3)")
	}
}

// ---------------------------------------------------------------------------
// 3. Flashback — spell is removed from graveyard when cast
// ---------------------------------------------------------------------------

func TestZoneCast_Flashback_RemovedFromGraveyard(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3

	fb := addGraveyardCardWithEffect(gs, 0, "Think Twice", 2,
		&gameast.Draw{Count: *gameast.NumInt(1)}, "instant")

	perm := NewFlashbackPermission(3)

	_, err := CastFromZone(gs, 0, fb, ZoneGraveyard, perm, nil)
	if err != nil {
		t.Fatalf("CastFromZone failed: %v", err)
	}

	// After casting and resolution, the card should not be in graveyard
	// (it was removed to cast, then exiled on resolution).
	for _, c := range gs.Seats[0].Graveyard {
		if c == fb {
			t.Error("Think Twice should not be in graveyard after flashback")
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Escape (Underworld Breach) — cast from graveyard + exile 3 others
// ---------------------------------------------------------------------------

func TestZoneCast_Escape_UnderworldBreach(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2

	// Target spell in graveyard.
	spell := addGraveyardCardWithEffect(gs, 0, "Dark Ritual", 1,
		&gameast.AddMana{
			Pool: []gameast.ManaSymbol{
				{Raw: "{B}", Color: []string{"B"}},
				{Raw: "{B}", Color: []string{"B"}},
				{Raw: "{B}", Color: []string{"B"}},
			},
		}, "instant")
	spell.CMC = 1

	// 3 other cards in graveyard to exile.
	filler1 := addGraveyardCard(gs, 0, "Filler1", 0, "creature")
	filler2 := addGraveyardCard(gs, 0, "Filler2", 0, "creature")
	filler3 := addGraveyardCard(gs, 0, "Filler3", 0, "creature")
	_ = filler1
	_ = filler2
	_ = filler3

	// Breach grants escape: mana cost of the card + exile 3 others.
	perm := NewBreachEscapePermission(1)

	// Should be castable.
	found := CanCastFromZone(gs, 0, spell, ZoneGraveyard, []*ZoneCastPermission{perm})
	if found == nil {
		t.Fatal("escape should be castable with enough mana and 3 other cards in graveyard")
	}

	startGYSize := len(gs.Seats[0].Graveyard)
	if startGYSize < 4 {
		t.Fatalf("expected at least 4 cards in graveyard, got %d", startGYSize)
	}

	result, err := CastFromZone(gs, 0, spell, ZoneGraveyard, perm, nil)
	if err != nil {
		t.Fatalf("CastFromZone failed: %v", err)
	}

	// 3 cards should have been exiled as additional cost.
	if len(result.ExiledCards) != 3 {
		t.Errorf("expected 3 exiled cards, got %d", len(result.ExiledCards))
	}

	// The spell itself should be in exile (escape = exile on resolve).
	spellInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == spell {
			spellInExile = true
			break
		}
	}
	if !spellInExile {
		t.Error("escape spell should be in exile after resolution")
	}

	// Graveyard should have 0 cards (1 spell + 3 fillers all removed).
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected 0 cards in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
}

// ---------------------------------------------------------------------------
// 5. Escape — cannot cast without 3 other cards in graveyard
// ---------------------------------------------------------------------------

func TestZoneCast_Escape_NotEnoughGraveyardCards(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	spell := addGraveyardCard(gs, 0, "Dark Ritual", 1, "instant")
	spell.CMC = 1

	// Only 1 other card (need 3).
	_ = addGraveyardCard(gs, 0, "Filler1", 0, "creature")

	perm := NewBreachEscapePermission(1)

	found := CanCastFromZone(gs, 0, spell, ZoneGraveyard, []*ZoneCastPermission{perm})
	if found != nil {
		t.Fatal("escape should NOT be castable with only 1 other card in graveyard")
	}
}

// ---------------------------------------------------------------------------
// 6. Cast from exile — Misthollow Griffin
// ---------------------------------------------------------------------------

func TestZoneCast_CastFromExile(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 4

	// Misthollow Griffin is in exile, costs 4, creature 3/3.
	griffin := addExileCard(gs, 0, "Misthollow Griffin", 4, "creature")
	griffin.CMC = 4
	griffin.BasePower = 3
	griffin.BaseToughness = 3

	perm := NewExileCastPermission()

	// Should be castable.
	found := CanCastFromZone(gs, 0, griffin, ZoneExile, []*ZoneCastPermission{perm})
	if found == nil {
		t.Fatal("should be able to cast Misthollow Griffin from exile")
	}

	_, err := CastFromZone(gs, 0, griffin, ZoneExile, perm, nil)
	if err != nil {
		t.Fatalf("CastFromZone failed: %v", err)
	}

	// Mana should be 0 (paid 4).
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana 0, got %d", gs.Seats[0].ManaPool)
	}

	// Griffin should NOT still be in exile (it was cast as a creature and
	// enters the battlefield).
	griffinInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == griffin {
			griffinInExile = true
		}
	}
	if griffinInExile {
		t.Error("Misthollow Griffin should not be in exile after casting")
	}

	// Griffin should be on the battlefield (it's a creature spell that
	// resolves to a permanent).
	griffinOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Misthollow Griffin" {
			griffinOnBF = true
		}
	}
	if !griffinOnBF {
		t.Error("Misthollow Griffin should be on battlefield after resolving")
	}
}

// ---------------------------------------------------------------------------
// 7. Cast from library top — Bolas's Citadel (pay life = CMC)
// ---------------------------------------------------------------------------

func TestZoneCast_LibraryCast_BolassCitadel(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 0
	gs.Seats[0].Life = 20

	// Put a spell on top of library.
	topCard := &Card{
		Name:  "Preordain",
		Owner: 0,
		Types: []string{"sorcery", "cost:1"},
		CMC:   1,
		AST: &gameast.CardAST{
			Name: "Preordain",
			Abilities: []gameast.Ability{
				&gameast.Activated{Effect: &gameast.Draw{Count: *gameast.NumInt(1)}},
			},
		},
	}
	gs.Seats[0].Library = append([]*Card{topCard}, gs.Seats[0].Library...)

	// Bolas's Citadel: pay life = CMC instead of mana.
	perm := NewLibraryCastPermission(topCard.CMC)

	// Should be castable.
	found := CanCastFromZone(gs, 0, topCard, ZoneLibrary, []*ZoneCastPermission{perm})
	if found == nil {
		t.Fatal("should be able to cast from library top with Bolas's Citadel")
	}

	_, err := CastFromZone(gs, 0, topCard, ZoneLibrary, perm, nil)
	if err != nil {
		t.Fatalf("CastFromZone failed: %v", err)
	}

	// Life should be 19 (paid 1 life for CMC 1).
	if gs.Seats[0].Life != 19 {
		t.Errorf("expected life 19, got %d", gs.Seats[0].Life)
	}

	// Mana should still be 0.
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana 0, got %d", gs.Seats[0].ManaPool)
	}

	// Card should not be on top of library.
	if len(gs.Seats[0].Library) > 0 && gs.Seats[0].Library[0] == topCard {
		t.Error("card should have been removed from top of library")
	}
}

// ---------------------------------------------------------------------------
// 8. ShouldExileOnResolve — helper tests
// ---------------------------------------------------------------------------

func TestZoneCast_ShouldExileOnResolve(t *testing.T) {
	// With exile_on_resolve in CostMeta.
	item := &StackItem{
		CostMeta: map[string]interface{}{
			"exile_on_resolve": true,
		},
	}
	if !ShouldExileOnResolve(item) {
		t.Error("expected ShouldExileOnResolve true")
	}

	// Without CostMeta.
	item2 := &StackItem{}
	if ShouldExileOnResolve(item2) {
		t.Error("expected ShouldExileOnResolve false for empty item")
	}

	// With false.
	item3 := &StackItem{
		CostMeta: map[string]interface{}{
			"exile_on_resolve": false,
		},
	}
	if ShouldExileOnResolve(item3) {
		t.Error("expected ShouldExileOnResolve false when set to false")
	}
}

// ---------------------------------------------------------------------------
// 9. Zone helpers — removeFromZone / addToZone
// ---------------------------------------------------------------------------

func TestZoneCast_RemoveFromZone(t *testing.T) {
	gs := newFixtureGame(t)
	seat := gs.Seats[0]

	card := &Card{Name: "Test Card", Owner: 0}
	seat.Graveyard = append(seat.Graveyard, card)

	if !removeFromZone(seat, card, "graveyard") {
		t.Fatal("should remove card from graveyard")
	}
	if len(seat.Graveyard) != 0 {
		t.Errorf("graveyard should be empty, got %d", len(seat.Graveyard))
	}

	// Add it back.
	addToZone(seat, card, "graveyard")
	if len(seat.Graveyard) != 1 {
		t.Errorf("graveyard should have 1 card, got %d", len(seat.Graveyard))
	}
}

// ---------------------------------------------------------------------------
// 10. Permission constructors produce correct structs
// ---------------------------------------------------------------------------

func TestZoneCast_PermissionConstructors(t *testing.T) {
	fb := NewFlashbackPermission(5)
	if fb.Zone != ZoneGraveyard {
		t.Errorf("flashback zone should be graveyard, got %s", fb.Zone)
	}
	if fb.ManaCost != 5 {
		t.Errorf("flashback mana cost should be 5, got %d", fb.ManaCost)
	}
	if !fb.ExileOnResolve {
		t.Error("flashback should exile on resolve")
	}

	esc := NewEscapePermission(3, 4)
	if esc.Zone != ZoneGraveyard {
		t.Errorf("escape zone should be graveyard, got %s", esc.Zone)
	}
	if len(esc.AdditionalCosts) != 1 {
		t.Fatalf("escape should have 1 additional cost, got %d", len(esc.AdditionalCosts))
	}
	if esc.AdditionalCosts[0].ExileCount != 4 {
		t.Errorf("escape exile count should be 4, got %d", esc.AdditionalCosts[0].ExileCount)
	}

	ec := NewExileCastPermission()
	if ec.Zone != ZoneExile {
		t.Errorf("exile cast zone should be exile, got %s", ec.Zone)
	}
	if ec.ManaCost != -1 {
		t.Errorf("exile cast mana cost should be -1, got %d", ec.ManaCost)
	}

	lc := NewLibraryCastPermission(3)
	if lc.Zone != ZoneLibrary {
		t.Errorf("library cast zone should be library, got %s", lc.Zone)
	}
	if lc.LifeCostInsteadOfMana != 3 {
		t.Errorf("library cast life cost should be 3, got %d", lc.LifeCostInsteadOfMana)
	}

	bc := NewBreachEscapePermission(2)
	if bc.Keyword != "escape" {
		t.Errorf("breach escape keyword should be 'escape', got %s", bc.Keyword)
	}
	if bc.SourceName != "Underworld Breach" {
		t.Errorf("breach escape source should be 'Underworld Breach', got %s", bc.SourceName)
	}
}
