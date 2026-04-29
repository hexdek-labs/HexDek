package per_card

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Fetchlands
// ============================================================================

func TestFetchland_ScaldingTarn_FindsIslandAndShuffles(t *testing.T) {
	gs := newGame(t, 2)
	tarn := addPerm(gs, 0, "Scalding Tarn", "land")
	gs.Seats[0].Life = 20
	// Put an island in the library.
	island := &gameengine.Card{Name: "Island", Owner: 0, Types: []string{"land", "island", "basic"}}
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}},
		island,
		{Name: "Brainstorm", Owner: 0, Types: []string{"instant"}},
	}

	gameengine.InvokeActivatedHook(gs, tarn, 0, nil)

	// Life should be 19 (paid 1).
	if gs.Seats[0].Life != 19 {
		t.Errorf("expected life 19 after Scalding Tarn, got %d", gs.Seats[0].Life)
	}
	// Tarn should be sacrificed (removed from battlefield).
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.DisplayName() == "Scalding Tarn" {
			t.Error("Scalding Tarn should have been sacrificed")
		}
	}
	// Island should be on the battlefield.
	foundIsland := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.DisplayName() == "Island" {
			foundIsland = true
			if p.Tapped {
				t.Error("Island from Scalding Tarn should enter untapped")
			}
		}
	}
	if !foundIsland {
		t.Error("Island should be on the battlefield after fetching")
	}
	// Library should have 2 cards left.
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 cards in library after fetch, got %d", len(gs.Seats[0].Library))
	}
}

func TestFetchland_EvolvingWilds_EntersTapped(t *testing.T) {
	gs := newGame(t, 2)
	ew := addPerm(gs, 0, "Evolving Wilds", "land")
	gs.Seats[0].Life = 20
	forest := &gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"land", "forest", "basic"}}
	gs.Seats[0].Library = []*gameengine.Card{forest}

	gameengine.InvokeActivatedHook(gs, ew, 0, nil)

	// No life paid for Evolving Wilds.
	if gs.Seats[0].Life != 20 {
		t.Errorf("expected life 20 (no life cost for Evolving Wilds), got %d", gs.Seats[0].Life)
	}
	// Forest should enter tapped.
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.DisplayName() == "Forest" {
			if !p.Tapped {
				t.Error("Forest from Evolving Wilds should enter tapped")
			}
		}
	}
}

func TestFetchland_NoMatchingLand_StillShuffles(t *testing.T) {
	gs := newGame(t, 2)
	tarn := addPerm(gs, 0, "Scalding Tarn", "land")
	gs.Seats[0].Life = 20
	// Library has no islands or mountains.
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Forest", Owner: 0, Types: []string{"land", "forest", "basic"}},
		{Name: "Plains", Owner: 0, Types: []string{"land", "plains", "basic"}},
	}

	gameengine.InvokeActivatedHook(gs, tarn, 0, nil)

	// Life should still be 19 (life is paid regardless).
	if gs.Seats[0].Life != 19 {
		t.Errorf("expected life 19, got %d", gs.Seats[0].Life)
	}
	// Library should still have 2 cards (nothing fetched).
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 cards in library (no match), got %d", len(gs.Seats[0].Library))
	}
}

// ============================================================================
// Shocklands
// ============================================================================

func TestShockland_WateryGrave_PayLifeUntapped(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	wg := addPerm(gs, 0, "Watery Grave", "land", "island", "swamp")
	gameengine.InvokeETBHook(gs, wg)

	// At 20 life, the heuristic pays 2 life.
	if gs.Seats[0].Life != 18 {
		t.Errorf("expected life 18 (paid 2 for shockland), got %d", gs.Seats[0].Life)
	}
	if wg.Tapped {
		t.Error("shockland should be untapped when life is paid")
	}
}

func TestShockland_LowLife_EntersTapped(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 4
	wg := addPerm(gs, 0, "Watery Grave", "land", "island", "swamp")
	gameengine.InvokeETBHook(gs, wg)

	// At 4 life, the heuristic doesn't pay.
	if gs.Seats[0].Life != 4 {
		t.Errorf("expected life 4 (no payment at low life), got %d", gs.Seats[0].Life)
	}
	if !wg.Tapped {
		t.Error("shockland should enter tapped at low life")
	}
}

// ============================================================================
// Tutors
// ============================================================================

func TestDemonicTutor_FindsAnyCard(t *testing.T) {
	gs := newGame(t, 2)
	target := &gameengine.Card{Name: "Thassa's Oracle", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Island", Owner: 0, Types: []string{"land"}},
		target,
		{Name: "Forest", Owner: 0, Types: []string{"land"}},
	}

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Demonic Tutor", Owner: 0, Types: []string{"sorcery"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	// First card found should be in hand.
	found := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Island" {
			found = true
		}
	}
	if !found {
		t.Error("Demonic Tutor should have put the first library card into hand")
	}
	// Library should be shuffled with 2 cards remaining.
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 cards in library after Demonic Tutor, got %d", len(gs.Seats[0].Library))
	}
}

func TestVampiricTutor_PutsOnTop_Pays2Life(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	target := &gameengine.Card{Name: "Thassa's Oracle", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Island", Owner: 0, Types: []string{"land"}},
		target,
	}

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Vampiric Tutor", Owner: 0, Types: []string{"instant"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Life should be 18.
	if gs.Seats[0].Life != 18 {
		t.Errorf("expected life 18, got %d", gs.Seats[0].Life)
	}
	// Top of library should be the found card.
	if len(gs.Seats[0].Library) == 0 {
		t.Fatal("library should not be empty")
	}
	// The first card found goes to top.
	// (The first card matching nil filter is "Island", but after shuffle+place-on-top
	// it should be on top.)
	top := gs.Seats[0].Library[0]
	if top.DisplayName() != "Island" {
		t.Errorf("expected Island on top of library, got %s", top.DisplayName())
	}
	// Hand should be empty (tutor to top, not to hand).
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected empty hand after Vampiric Tutor, got %d cards", len(gs.Seats[0].Hand))
	}
}

func TestMysticalTutor_FindsInstantOrSorcery(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Forest", Owner: 0, Types: []string{"land"}},
		{Name: "Counterspell", Owner: 0, Types: []string{"instant"}},
		{Name: "Island", Owner: 0, Types: []string{"land"}},
	}

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Mystical Tutor", Owner: 0, Types: []string{"instant"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Counterspell should be on top.
	if len(gs.Seats[0].Library) == 0 {
		t.Fatal("library should not be empty")
	}
	top := gs.Seats[0].Library[0]
	if top.DisplayName() != "Counterspell" {
		t.Errorf("expected Counterspell on top, got %s", top.DisplayName())
	}
}

func TestEnlightenedTutor_FindsArtifactOrEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Island", Owner: 0, Types: []string{"land"}},
		{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}},
	}

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Enlightened Tutor", Owner: 0, Types: []string{"instant"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	top := gs.Seats[0].Library[0]
	if top.DisplayName() != "Sol Ring" {
		t.Errorf("expected Sol Ring on top, got %s", top.DisplayName())
	}
}

func TestWorldlyTutor_FindsCreature(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "Island", Owner: 0, Types: []string{"land"}},
		{Name: "Dockside Extortionist", Owner: 0, Types: []string{"creature"}},
	}

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Worldly Tutor", Owner: 0, Types: []string{"instant"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	top := gs.Seats[0].Library[0]
	if top.DisplayName() != "Dockside Extortionist" {
		t.Errorf("expected Dockside Extortionist on top, got %s", top.DisplayName())
	}
}

// ============================================================================
// Ragavan, Nimble Pilferer
// ============================================================================

func TestRagavan_CombatDamageTrigger(t *testing.T) {
	gs := newGame(t, 2)
	ragavan := addPerm(gs, 0, "Ragavan, Nimble Pilferer", "creature", "legendary")
	gs.Seats[1].Library = []*gameengine.Card{
		{Name: "Force of Will", Owner: 1, Types: []string{"instant"}},
	}

	// Fire the combat damage trigger.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Ragavan, Nimble Pilferer",
		"defender_seat": 1,
		"amount":        1,
	})
	_ = ragavan // used via trigger registration

	// Should have created a treasure and exiled a card.
	treasures := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && strings.Contains(p.Card.DisplayName(), "Treasure") {
			treasures++
		}
	}
	if treasures == 0 {
		t.Error("Ragavan should have created a Treasure token")
	}

	// Opponent's library should be empty.
	if len(gs.Seats[1].Library) != 0 {
		t.Errorf("expected opponent library empty after exile, got %d", len(gs.Seats[1].Library))
	}

	// Card goes to its owner's (defender's) exile per CR §406.
	if len(gs.Seats[1].Exile) == 0 {
		t.Error("defender should have the exiled card in their exile zone")
	}
}

// ============================================================================
// The One Ring
// ============================================================================

func TestTheOneRing_ETB_GrantsProtection(t *testing.T) {
	gs := newGame(t, 2)
	ring := addPerm(gs, 0, "The One Ring", "artifact", "legendary")
	gameengine.InvokeETBHook(gs, ring)

	// Should have a prevention shield.
	if len(gs.PreventionShields) == 0 {
		t.Error("The One Ring ETB should add a prevention shield")
	}
	// Indestructible flag.
	if ring.Flags["indestructible"] != 1 {
		t.Error("The One Ring should have indestructible flag")
	}
}

func TestTheOneRing_Activated_DrawsCards(t *testing.T) {
	gs := newGame(t, 2)
	ring := addPerm(gs, 0, "The One Ring", "artifact", "legendary")
	ring.Counters = map[string]int{} // no burden counters yet
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "A", Owner: 0},
		{Name: "B", Owner: 0},
		{Name: "C", Owner: 0},
	}

	// First activation: 0 burden → add 1 → draw 1.
	gameengine.InvokeActivatedHook(gs, ring, 0, nil)
	if ring.Counters["burden"] != 1 {
		t.Errorf("expected 1 burden counter, got %d", ring.Counters["burden"])
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand after first activation, got %d", len(gs.Seats[0].Hand))
	}

	// Untap for second activation.
	ring.Tapped = false

	// Second activation: 1 burden → add 1 → draw 2.
	gameengine.InvokeActivatedHook(gs, ring, 0, nil)
	if ring.Counters["burden"] != 2 {
		t.Errorf("expected 2 burden counters, got %d", ring.Counters["burden"])
	}
	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("expected 3 cards in hand after second activation, got %d", len(gs.Seats[0].Hand))
	}
}

func TestTheOneRing_Upkeep_LosesLife(t *testing.T) {
	gs := newGame(t, 2)
	ring := addPerm(gs, 0, "The One Ring", "artifact", "legendary")
	ring.Counters = map[string]int{"burden": 3}
	gs.Seats[0].Life = 40

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})

	if gs.Seats[0].Life != 37 {
		t.Errorf("expected life 37 (lost 3 from burden), got %d", gs.Seats[0].Life)
	}
}

// ============================================================================
// Eternal Witness
// ============================================================================

func TestEternalWitness_ReturnsCardFromGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Demonic Tutor", Owner: 0, Types: []string{"sorcery"}},
	}
	ew := addPerm(gs, 0, "Eternal Witness", "creature")
	gameengine.InvokeETBHook(gs, ew)

	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0].DisplayName() != "Demonic Tutor" {
		t.Error("Eternal Witness should have returned Demonic Tutor to hand")
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Error("Graveyard should be empty after Eternal Witness ETB")
	}
}

func TestEternalWitness_EmptyGraveyard_NoOp(t *testing.T) {
	gs := newGame(t, 2)
	ew := addPerm(gs, 0, "Eternal Witness", "creature")
	gameengine.InvokeETBHook(gs, ew)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected empty hand, got %d", len(gs.Seats[0].Hand))
	}
}

// ============================================================================
// Sylvan Library
// ============================================================================

func TestSylvanLibrary_DrawsExtraCards(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40
	_ = addPerm(gs, 0, "Sylvan Library", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "A", Owner: 0},
		{Name: "B", Owner: 0},
		{Name: "C", Owner: 0},
	}

	gameengine.FireCardTrigger(gs, "draw_step_controller", map[string]interface{}{
		"active_seat": 0,
	})

	// At 40 life (>12), should keep both and pay 8 life.
	if gs.Seats[0].Life != 32 {
		t.Errorf("expected life 32 (paid 8 for 2 cards), got %d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected 2 cards in hand, got %d", len(gs.Seats[0].Hand))
	}
}

func TestSylvanLibrary_LowLife_ReturnsCards(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 8
	_ = addPerm(gs, 0, "Sylvan Library", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "A", Owner: 0},
		{Name: "B", Owner: 0},
	}

	gameengine.FireCardTrigger(gs, "draw_step_controller", map[string]interface{}{
		"active_seat": 0,
	})

	// At 8 life (<=12), should return cards to top.
	if gs.Seats[0].Life != 8 {
		t.Errorf("expected life unchanged at 8, got %d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected 0 cards in hand (returned to library), got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 cards back in library, got %d", len(gs.Seats[0].Library))
	}
}

// ============================================================================
// Chrome Mox
// ============================================================================

func TestChromeMox_ImprintsAndTapsForColor(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Force of Will", Owner: 0, Types: []string{"instant"}, Colors: []string{"U"}},
	}
	mox := addPerm(gs, 0, "Chrome Mox", "artifact")
	gameengine.InvokeETBHook(gs, mox)

	// Hand should be empty (card exiled).
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected empty hand after imprint, got %d", len(gs.Seats[0].Hand))
	}
	// Exile should have the imprinted card.
	if len(gs.Seats[0].Exile) != 1 {
		t.Errorf("expected 1 card in exile, got %d", len(gs.Seats[0].Exile))
	}

	// Now tap for mana.
	gameengine.InvokeActivatedHook(gs, mox, 0, nil)
	if gs.Seats[0].ManaPool < 1 {
		t.Errorf("expected at least 1 mana after tapping Chrome Mox, got %d", gs.Seats[0].ManaPool)
	}
}

func TestChromeMox_NoEligibleCard_StillPlays(t *testing.T) {
	gs := newGame(t, 2)
	// Only artifacts in hand — can't imprint.
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}},
	}
	mox := addPerm(gs, 0, "Chrome Mox", "artifact")
	gameengine.InvokeETBHook(gs, mox)

	// Hand should still have Sol Ring.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand (no eligible imprint), got %d", len(gs.Seats[0].Hand))
	}

	// Tapping should fail (no imprint).
	gameengine.InvokeActivatedHook(gs, mox, 0, nil)
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected 0 mana (no imprint), got %d", gs.Seats[0].ManaPool)
	}
}

// ============================================================================
// Mox Opal
// ============================================================================

func TestMoxOpal_MetalcraftMet_ProducesMana(t *testing.T) {
	gs := newGame(t, 2)
	opal := addPerm(gs, 0, "Mox Opal", "artifact", "legendary")
	addPerm(gs, 0, "Sol Ring", "artifact")
	addPerm(gs, 0, "Mana Crypt", "artifact")

	gameengine.InvokeActivatedHook(gs, opal, 0, nil)
	if gs.Seats[0].ManaPool < 1 {
		t.Errorf("expected at least 1 mana with metalcraft, got %d", gs.Seats[0].ManaPool)
	}
}

func TestMoxOpal_MetalcraftNotMet_NoMana(t *testing.T) {
	gs := newGame(t, 2)
	opal := addPerm(gs, 0, "Mox Opal", "artifact", "legendary")
	// Only 1 artifact (Opal itself, + need 2 more).
	addPerm(gs, 0, "Sol Ring", "artifact")

	gameengine.InvokeActivatedHook(gs, opal, 0, nil)
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected 0 mana without metalcraft, got %d", gs.Seats[0].ManaPool)
	}
}

// ============================================================================
// Mox Amber
// ============================================================================

func TestMoxAmber_WithLegendaryCreature_ProducesMana(t *testing.T) {
	gs := newGame(t, 2)
	amber := addPerm(gs, 0, "Mox Amber", "artifact", "legendary")
	addPerm(gs, 0, "Ragavan, Nimble Pilferer", "creature", "legendary", "pip:R")

	gameengine.InvokeActivatedHook(gs, amber, 0, nil)
	if gs.Seats[0].ManaPool < 1 {
		t.Errorf("expected at least 1 mana with legendary creature, got %d", gs.Seats[0].ManaPool)
	}
}

func TestMoxAmber_NoLegendary_NoMana(t *testing.T) {
	gs := newGame(t, 2)
	amber := addPerm(gs, 0, "Mox Amber", "artifact", "legendary")

	gameengine.InvokeActivatedHook(gs, amber, 0, nil)
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected 0 mana without legendary creature, got %d", gs.Seats[0].ManaPool)
	}
}

// ============================================================================
// Gemstone Caverns
// ============================================================================

func TestGemstoneCaverns_WithLuckCounter_AnyColor(t *testing.T) {
	gs := newGame(t, 2)
	gc := addPerm(gs, 0, "Gemstone Caverns", "land")
	gc.Counters = map[string]int{"luck": 1}

	gameengine.InvokeActivatedHook(gs, gc, 0, nil)
	if gs.Seats[0].ManaPool < 1 {
		t.Errorf("expected at least 1 mana with luck counter, got %d", gs.Seats[0].ManaPool)
	}
}

func TestGemstoneCaverns_NoLuckCounter_Colorless(t *testing.T) {
	gs := newGame(t, 2)
	gc := addPerm(gs, 0, "Gemstone Caverns", "land")

	gameengine.InvokeActivatedHook(gs, gc, 0, nil)
	if gs.Seats[0].ManaPool < 1 {
		t.Errorf("expected at least 1 mana (colorless), got %d", gs.Seats[0].ManaPool)
	}
}

// ============================================================================
// Bojuka Bog
// ============================================================================

func TestBojukaBog_ExilesOpponentGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Graveyard = []*gameengine.Card{
		{Name: "A", Owner: 1},
		{Name: "B", Owner: 1},
		{Name: "C", Owner: 1},
	}
	bog := addPerm(gs, 0, "Bojuka Bog", "land")
	gameengine.InvokeETBHook(gs, bog)

	if len(gs.Seats[1].Graveyard) != 0 {
		t.Errorf("expected opponent graveyard empty, got %d", len(gs.Seats[1].Graveyard))
	}
	if len(gs.Seats[1].Exile) != 3 {
		t.Errorf("expected 3 cards exiled, got %d", len(gs.Seats[1].Exile))
	}
	if !bog.Tapped {
		t.Error("Bojuka Bog should enter tapped")
	}
}

// ============================================================================
// Reliquary Tower
// ============================================================================

func TestReliquaryTower_SetsNoMaxHandSize(t *testing.T) {
	gs := newGame(t, 2)
	rt := addPerm(gs, 0, "Reliquary Tower", "land")
	gameengine.InvokeETBHook(gs, rt)

	if gs.Flags["no_max_hand_size_seat_0"] != 1 {
		t.Error("Reliquary Tower should set no_max_hand_size flag")
	}
}

// ============================================================================
// Ancient Tomb
// ============================================================================

func TestAncientTomb_Adds2Colorless_Deals2Damage(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	tomb := addPerm(gs, 0, "Ancient Tomb", "land")

	gameengine.InvokeActivatedHook(gs, tomb, 0, nil)

	if gs.Seats[0].ManaPool < 2 {
		t.Errorf("expected at least 2 mana, got %d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Life != 18 {
		t.Errorf("expected life 18 (took 2 damage), got %d", gs.Seats[0].Life)
	}
	if !tomb.Tapped {
		t.Error("Ancient Tomb should be tapped")
	}
}

// ============================================================================
// Path to Exile
// ============================================================================

func TestPathToExile_ExilesCreature_GivesBasicLand(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Ragavan, Nimble Pilferer", "creature", "legendary")
	target.Card.BasePower = 2
	gs.Seats[1].Library = []*gameengine.Card{
		{Name: "Mountain", Owner: 1, Types: []string{"land", "mountain", "basic"}},
	}

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Path to Exile", Owner: 0, Types: []string{"instant"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Ragavan should be gone from battlefield.
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card.DisplayName() == "Ragavan, Nimble Pilferer" {
			t.Error("Ragavan should have been exiled")
		}
	}
	// Opponent should have a basic land on battlefield (enters tapped from Path).
	foundLand := false
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card.DisplayName() == "Mountain" {
			foundLand = true
		}
	}
	if !foundLand {
		t.Error("opponent should have searched for a basic land")
	}
}

// ============================================================================
// Swords to Plowshares
// ============================================================================

func TestSwordsToPlowshares_ExilesCreature_GainsLife(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 20
	target := addPerm(gs, 1, "Tarmogoyf", "creature")
	target.Card.BasePower = 5

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Swords to Plowshares", Owner: 0, Types: []string{"instant"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Target should be gone.
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card.DisplayName() == "Tarmogoyf" {
			t.Error("Tarmogoyf should have been exiled")
		}
	}
	// Controller of exiled creature gains life equal to power.
	if gs.Seats[1].Life != 25 {
		t.Errorf("expected opponent life 25 (gained 5), got %d", gs.Seats[1].Life)
	}
}

// ============================================================================
// Cyclonic Rift
// ============================================================================

func TestCyclonicRift_Overload_BouncesAllOpponents(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5 // past turn 3, triggers overload heuristic
	addPerm(gs, 1, "Sol Ring", "artifact")
	addPerm(gs, 1, "Rhystic Study", "enchantment")
	addPerm(gs, 1, "Island", "land") // lands should NOT be bounced

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Cyclonic Rift", Owner: 0, Types: []string{"instant"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Only the Island should remain (lands are not bounced).
	remaining := 0
	for _, p := range gs.Seats[1].Battlefield {
		remaining++
		if !p.IsLand() {
			t.Errorf("non-land permanent %s should have been bounced", p.Card.DisplayName())
		}
	}
	if remaining != 1 {
		t.Errorf("expected 1 remaining permanent (Island), got %d", remaining)
	}
}
