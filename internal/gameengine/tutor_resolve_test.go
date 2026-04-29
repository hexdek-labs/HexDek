package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// addLibraryTyped appends cards with specific types to a seat's library.
func addLibraryTyped(gs *GameState, seat int, cards ...struct {
	name  string
	types []string
	cmc   int
}) {
	for _, c := range cards {
		card := &Card{
			Name:  c.name,
			Owner: seat,
			Types: c.types,
			CMC:   c.cmc,
		}
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, card)
	}
}

// addLibraryColored appends cards with specific types and colors.
func addLibraryColored(gs *GameState, seat int, name string, types []string, colors []string) {
	card := &Card{
		Name:   name,
		Owner:  seat,
		Types:  types,
		Colors: colors,
	}
	gs.Seats[seat].Library = append(gs.Seats[seat].Library, card)
}

// ============================================================================
// Test 1: Basic tutor — "Search your library for a creature card, put it
//         into your hand, then shuffle." (Green Sun's Zenith style)
// ============================================================================

func TestTutorGeneric_CreatureToHand(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Lightning Bolt", []string{"instant"}, 1},
		struct {
			name  string
			types []string
			cmc   int
		}{"Grizzly Bears", []string{"creature"}, 2},
		struct {
			name  string
			types []string
			cmc   int
		}{"Counterspell", []string{"instant"}, 2},
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "creature"},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Fatalf("expected 1 card in hand, got %d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Hand[0].Name != "Grizzly Bears" {
		t.Errorf("expected Grizzly Bears in hand, got %s", gs.Seats[0].Hand[0].Name)
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 cards left in library, got %d", len(gs.Seats[0].Library))
	}
	if !hasEvent(gs, "tutor") {
		t.Error("expected tutor event")
	}
}

// ============================================================================
// Test 2: Tutor to top of library — "Search, shuffle, put on top."
//         (Vampiric Tutor pattern: shuffle FIRST, then place on top)
// ============================================================================

func TestTutorGeneric_ToTopOfLibrary(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Forest", []string{"land", "basic"}, 0},
		struct {
			name  string
			types []string
			cmc   int
		}{"Sol Ring", []string{"artifact"}, 1},
		struct {
			name  string
			types []string
			cmc   int
		}{"Swamp", []string{"land", "basic"}, 0},
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "artifact"},
		Destination:  "top_of_library",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	// Library: 3 original - 1 removed + 1 placed on top = 3 total.
	// (Sol Ring removed from library, shuffle, then placed back on top.)
	if len(gs.Seats[0].Library) != 3 {
		t.Fatalf("expected 3 cards in library (2 remaining + 1 on top), got %d", len(gs.Seats[0].Library))
	}
	// Sol Ring was put on top after shuffle.
	if gs.Seats[0].Library[0].Name != "Sol Ring" {
		t.Errorf("expected Sol Ring on top of library, got %s", gs.Seats[0].Library[0].Name)
	}
}

// ============================================================================
// Test 3: Tutor to battlefield — "Search for a basic land, put it onto
//         the battlefield tapped." (Rampant Growth style)
// ============================================================================

func TestTutorGeneric_LandToBattlefieldTapped(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Island", []string{"land", "basic"}, 0},
		struct {
			name  string
			types []string
			cmc   int
		}{"Grizzly Bears", []string{"creature"}, 2},
		struct {
			name  string
			types []string
			cmc   int
		}{"Forest", []string{"land", "basic"}, 0},
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "basic_land"},
		Destination:  "battlefield_tapped",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("expected 1 permanent on battlefield, got %d", len(gs.Seats[0].Battlefield))
	}
	perm := gs.Seats[0].Battlefield[0]
	if perm.Card.Name != "Island" {
		t.Errorf("expected Island on battlefield, got %s", perm.Card.Name)
	}
	if !perm.Tapped {
		t.Error("expected permanent to be tapped")
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 cards left in library, got %d", len(gs.Seats[0].Library))
	}
}

// ============================================================================
// Test 4: Tutor with no matching cards — fail to find (legal)
// ============================================================================

func TestTutorGeneric_FailToFind(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Lightning Bolt", []string{"instant"}, 1},
		struct {
			name  string
			types []string
			cmc   int
		}{"Counterspell", []string{"instant"}, 2},
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "creature"},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
		Optional:     true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 0 {
		t.Fatalf("expected 0 cards found, got %d", found)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected empty hand, got %d cards", len(gs.Seats[0].Hand))
	}
	// Library should still be shuffled even on fail-to-find.
	if !hasEvent(gs, "tutor") {
		t.Error("expected tutor event even on fail-to-find")
	}
}

// ============================================================================
// Test 5: Unrestricted tutor — "Search for a card" (Demonic Tutor)
// ============================================================================

func TestTutorGeneric_UnrestrictedSearch(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Sol Ring", []string{"artifact"}, 1},
		struct {
			name  string
			types []string
			cmc   int
		}{"Mox Diamond", []string{"artifact"}, 0},
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "card"},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand, got %d", len(gs.Seats[0].Hand))
	}
}

// ============================================================================
// Test 6: Tutor for instant or sorcery (Mystical Tutor filter)
// ============================================================================

func TestTutorGeneric_InstantOrSorcery(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Grizzly Bears", []string{"creature"}, 2},
		struct {
			name  string
			types []string
			cmc   int
		}{"Demonic Tutor", []string{"sorcery"}, 2},
		struct {
			name  string
			types []string
			cmc   int
		}{"Brainstorm", []string{"instant"}, 1},
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "instant_or_sorcery"},
		Destination:  "top_of_library",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
		Reveal:       true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	// First matching instant or sorcery is Demonic Tutor (sorcery).
	if gs.Seats[0].Library[0].Name != "Demonic Tutor" {
		t.Errorf("expected Demonic Tutor on top, got %s", gs.Seats[0].Library[0].Name)
	}
}

// ============================================================================
// Test 7: Tutor with Extra filter — "nonland permanent" to hand
// ============================================================================

func TestTutorGeneric_NonlandPermanent(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Forest", []string{"land", "basic"}, 0},
		struct {
			name  string
			types []string
			cmc   int
		}{"Sol Ring", []string{"artifact"}, 1},
		struct {
			name  string
			types []string
			cmc   int
		}{"Island", []string{"land", "basic"}, 0},
	)

	tutor := &gameast.Tutor{
		Query: gameast.Filter{
			Base:  "permanent",
			Extra: []string{"nonland"},
		},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	if gs.Seats[0].Hand[0].Name != "Sol Ring" {
		t.Errorf("expected Sol Ring in hand, got %s", gs.Seats[0].Hand[0].Name)
	}
}

// ============================================================================
// Test 8: Tutor with color filter
// ============================================================================

func TestTutorGeneric_ColorFilter(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryColored(gs, 0, "Lightning Bolt", []string{"instant"}, []string{"R"})
	addLibraryColored(gs, 0, "Counterspell", []string{"instant"}, []string{"U"})
	addLibraryColored(gs, 0, "Swords to Plowshares", []string{"instant"}, []string{"W"})

	tutor := &gameast.Tutor{
		Query: gameast.Filter{
			Base:        "instant",
			ColorFilter: []string{"U"},
		},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	if gs.Seats[0].Hand[0].Name != "Counterspell" {
		t.Errorf("expected Counterspell in hand, got %s", gs.Seats[0].Hand[0].Name)
	}
}

// ============================================================================
// Test 9: Tutor with creature subtype filter
// ============================================================================

func TestTutorGeneric_CreatureSubtype(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Grizzly Bears", []string{"creature", "bear"}, 2},
		struct {
			name  string
			types []string
			cmc   int
		}{"Llanowar Elves", []string{"creature", "elf"}, 1},
		struct {
			name  string
			types []string
			cmc   int
		}{"Elvish Mystic", []string{"creature", "elf"}, 1},
	)

	tutor := &gameast.Tutor{
		Query: gameast.Filter{
			Base:          "creature",
			CreatureTypes: []string{"elf"},
		},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	if gs.Seats[0].Hand[0].Name != "Llanowar Elves" {
		t.Errorf("expected Llanowar Elves in hand, got %s", gs.Seats[0].Hand[0].Name)
	}
}

// ============================================================================
// Test 10: Tutor with CMC constraint — "creature with mana value <= 3"
// ============================================================================

func TestTutorGeneric_ManaValueConstraint(t *testing.T) {
	gs := newFixtureGame(t)

	mv := 3
	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Primeval Titan", []string{"creature"}, 6},
		struct {
			name  string
			types []string
			cmc   int
		}{"Birds of Paradise", []string{"creature"}, 1},
		struct {
			name  string
			types []string
			cmc   int
		}{"Craterhoof Behemoth", []string{"creature"}, 8},
	)

	tutor := &gameast.Tutor{
		Query: gameast.Filter{
			Base:        "creature",
			ManaValueOp: "<=",
			ManaValue:   &mv,
		},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	if gs.Seats[0].Hand[0].Name != "Birds of Paradise" {
		t.Errorf("expected Birds of Paradise in hand, got %s", gs.Seats[0].Hand[0].Name)
	}
}

// ============================================================================
// Test 11: Tutor to graveyard — "Search for a card, put it into your
//          graveyard." (Entomb style)
// ============================================================================

func TestTutorGeneric_ToGraveyard(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Griselbrand", []string{"creature"}, 8},
		struct {
			name  string
			types []string
			cmc   int
		}{"Lightning Bolt", []string{"instant"}, 1},
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "card"},
		Destination:  "graveyard",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Fatalf("expected 1 card in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
	if gs.Seats[0].Graveyard[0].Name != "Griselbrand" {
		t.Errorf("expected Griselbrand in graveyard, got %s", gs.Seats[0].Graveyard[0].Name)
	}
}

// ============================================================================
// Test 12: Opposition Agent interception
// ============================================================================

func TestTutorGeneric_OppositionAgent(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Flags["opposition_agent_seat_1"] = 1

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Grizzly Bears", []string{"creature"}, 2},
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "creature"},
		Destination:  "hand",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	// Card goes to opponent's exile, NOT our hand.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected empty hand (agent intercepted), got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[1].Exile) != 1 {
		t.Errorf("expected 1 card in opponent's exile, got %d", len(gs.Seats[1].Exile))
	}
	if !hasEvent(gs, "opposition_agent_exile") {
		t.Error("expected opposition_agent_exile event")
	}
}

// ============================================================================
// Test 13: Tutor to battlefield untapped — "Search for a land, put it onto
//          the battlefield." (Fetchland style)
// ============================================================================

func TestTutorGeneric_LandToBattlefieldUntapped(t *testing.T) {
	gs := newFixtureGame(t)

	addLibraryTyped(gs, 0,
		struct {
			name  string
			types []string
			cmc   int
		}{"Forest", []string{"land", "basic"}, 0},
		struct {
			name  string
			types []string
			cmc   int
		}{"Swamp", []string{"land", "basic"}, 0},
	)

	tutor := &gameast.Tutor{
		Query:        gameast.Filter{Base: "basic_land"},
		Destination:  "battlefield",
		Count:        *gameast.NumInt(1),
		ShuffleAfter: true,
	}

	found := ResolveTutorGeneric(gs, 0, tutor)
	if found != 1 {
		t.Fatalf("expected 1 card found, got %d", found)
	}
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("expected 1 permanent, got %d", len(gs.Seats[0].Battlefield))
	}
	perm := gs.Seats[0].Battlefield[0]
	if perm.Tapped {
		t.Error("expected permanent untapped for 'battlefield' destination")
	}
}

// ============================================================================
// Test 14: Nil/empty tutor safety
// ============================================================================

func TestTutorGeneric_NilSafety(t *testing.T) {
	gs := newFixtureGame(t)

	// Nil tutor
	found := ResolveTutorGeneric(gs, 0, nil)
	if found != 0 {
		t.Errorf("expected 0 for nil tutor, got %d", found)
	}

	// Nil game state
	tutor := &gameast.Tutor{
		Query: gameast.Filter{Base: "creature"},
		Count: *gameast.NumInt(1),
	}
	found = ResolveTutorGeneric(nil, 0, tutor)
	if found != 0 {
		t.Errorf("expected 0 for nil gs, got %d", found)
	}

	// Invalid seat
	found = ResolveTutorGeneric(gs, 99, tutor)
	if found != 0 {
		t.Errorf("expected 0 for invalid seat, got %d", found)
	}
}

// ============================================================================
// VERIFICATION TESTS: Destroy resolves through DestroyPermanent properly
// ============================================================================

func TestDestroyGeneric_BoardWipe(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Wrath of God", 0, 0, "sorcery")
	_ = addBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")
	_ = addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")
	_ = addBattlefield(gs, 1, "Serra Angel", 4, 4, "creature")

	// Board wipe: "each" quantifier destroys all creatures.
	e := &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Quantifier: "each"},
	}
	ResolveEffect(gs, src, e)

	// All creatures destroyed. Only the sorcery "Wrath of God" remains
	// on seat 0 (it's not a creature).
	creatureCount := 0
	for _, seat := range gs.Seats {
		for _, p := range seat.Battlefield {
			if p.IsCreature() {
				creatureCount++
			}
		}
	}
	if creatureCount != 0 {
		t.Errorf("expected 0 creatures after board wipe, got %d", creatureCount)
	}
}

func TestDestroyGeneric_NonlandPermanent(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Vindicate", 0, 0, "sorcery")
	land := addBattlefield(gs, 1, "Forest", 0, 0, "land")
	artifact := addBattlefield(gs, 1, "Sol Ring", 0, 0, "artifact")

	// "destroy target nonland permanent" — should hit Sol Ring, not Forest.
	e := &gameast.Destroy{
		Target: gameast.Filter{
			Base:             "permanent",
			OpponentControls: true,
			Targeted:         true,
			Extra:            []string{"nonland"},
		},
	}
	ResolveEffect(gs, src, e)

	// Sol Ring should be destroyed, Forest should survive.
	landSurvived := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == land {
			landSurvived = true
		}
		if p == artifact {
			t.Error("Sol Ring should have been destroyed")
		}
	}
	if !landSurvived {
		t.Error("Forest should have survived (nonland filter)")
	}
}

func TestDestroyGeneric_IndestructibleSurvives(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Doom Blade", 0, 0, "instant")
	creature := addBattlefield(gs, 1, "Darksteel Colossus", 11, 11, "creature", "artifact")
	creature.GrantedAbilities = append(creature.GrantedAbilities, "indestructible")

	e := &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
	}
	ResolveEffect(gs, src, e)

	// Indestructible creature survives.
	if len(gs.Seats[1].Battlefield) != 1 {
		t.Errorf("expected indestructible creature to survive, got %d permanents", len(gs.Seats[1].Battlefield))
	}
}

// ============================================================================
// VERIFICATION TESTS: Bounce resolves through BouncePermanent properly
// ============================================================================

func TestBounceGeneric_ToLibraryTop(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Terminus", 0, 0, "sorcery")
	_ = addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	e := &gameast.Bounce{
		Target: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
		To:     "top_of_library",
	}
	ResolveEffect(gs, src, e)

	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("expected no creatures on battlefield, got %d", len(gs.Seats[1].Battlefield))
	}
	if len(gs.Seats[1].Library) != 1 {
		t.Errorf("expected 1 card on top of library, got %d", len(gs.Seats[1].Library))
	}
}

func TestBounceGeneric_ToLibraryBottom(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Condemn", 0, 0, "instant")
	_ = addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")
	addLibrary(gs, 1, "Filler Card")

	e := &gameast.Bounce{
		Target: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
		To:     "bottom_of_library",
	}
	ResolveEffect(gs, src, e)

	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("expected empty battlefield, got %d", len(gs.Seats[1].Battlefield))
	}
	// Library should have 2 cards, with the bounced one on bottom.
	if len(gs.Seats[1].Library) != 2 {
		t.Fatalf("expected 2 cards in library, got %d", len(gs.Seats[1].Library))
	}
	// First card should be the filler, last should be Grizzly Bears.
	if gs.Seats[1].Library[len(gs.Seats[1].Library)-1].Name != "Grizzly Bears" {
		t.Errorf("expected Grizzly Bears on bottom, got %s",
			gs.Seats[1].Library[len(gs.Seats[1].Library)-1].Name)
	}
}

func TestBounceGeneric_NonlandPermanent(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Cyclonic Rift", 0, 0, "instant")
	land := addBattlefield(gs, 1, "Forest", 0, 0, "land")
	creature := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	// Cyclonic Rift overload: "return all nonland permanents to their owners' hands"
	e := &gameast.Bounce{
		Target: gameast.Filter{
			Base:             "permanent",
			Quantifier:       "each",
			OpponentControls: true,
			Extra:            []string{"nonland"},
		},
		To: "owners_hand",
	}
	ResolveEffect(gs, src, e)

	// Land survives, creature bounced.
	landSurvived := false
	creatureSurvived := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == land {
			landSurvived = true
		}
		if p == creature {
			creatureSurvived = true
		}
	}
	if !landSurvived {
		t.Error("Forest should survive (nonland filter)")
	}
	if creatureSurvived {
		t.Error("Grizzly Bears should have been bounced")
	}
	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("expected 1 card in hand after bounce, got %d", len(gs.Seats[1].Hand))
	}
}

// ============================================================================
// VERIFICATION TESTS: Exile resolves through ExilePermanent properly
// ============================================================================

func TestExileGeneric_IgnoresIndestructible(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Path to Exile", 0, 0, "instant")
	creature := addBattlefield(gs, 1, "Darksteel Colossus", 11, 11, "creature")
	creature.GrantedAbilities = append(creature.GrantedAbilities, "indestructible")

	e := &gameast.Exile{
		Target: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
	}
	ResolveEffect(gs, src, e)

	// Exile ignores indestructible.
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Error("expected creature to be exiled despite indestructible")
	}
	if len(gs.Seats[1].Exile) != 1 {
		t.Errorf("expected 1 card in exile, got %d", len(gs.Seats[1].Exile))
	}
}

func TestExileGeneric_NonlandPermanent(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Generous Gift", 0, 0, "instant")
	land := addBattlefield(gs, 1, "Forest", 0, 0, "land")
	_ = addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	// "exile target nonland permanent"
	e := &gameast.Exile{
		Target: gameast.Filter{
			Base:             "permanent",
			OpponentControls: true,
			Targeted:         true,
			Extra:            []string{"nonland"},
		},
	}
	ResolveEffect(gs, src, e)

	// Land survives, creature exiled.
	landSurvived := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == land {
			landSurvived = true
		}
	}
	if !landSurvived {
		t.Error("Forest should survive (nonland filter)")
	}
	if len(gs.Seats[1].Exile) != 1 {
		t.Errorf("expected 1 card in exile, got %d", len(gs.Seats[1].Exile))
	}
}

// ============================================================================
// VERIFICATION TESTS: Draw handles all variants
// ============================================================================

func TestDrawGeneric_TargetsOpponent(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Braingeyser", 0, 0, "sorcery")
	addLibrary(gs, 1, "A", "B", "C")

	e := &gameast.Draw{
		Count:  *gameast.NumInt(2),
		Target: gameast.TargetOpponent(),
	}
	ResolveEffect(gs, src, e)

	if len(gs.Seats[1].Hand) != 2 {
		t.Errorf("expected opponent to draw 2, got %d", len(gs.Seats[1].Hand))
	}
}

func TestDrawGeneric_EachPlayer(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Windfall", 0, 0, "sorcery")
	addLibrary(gs, 0, "A", "B", "C")
	addLibrary(gs, 1, "X", "Y", "Z")

	e := &gameast.Draw{
		Count:  *gameast.NumInt(2),
		Target: gameast.EachPlayer(),
	}
	ResolveEffect(gs, src, e)

	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected seat 0 to draw 2, got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[1].Hand) != 2 {
		t.Errorf("expected seat 1 to draw 2, got %d", len(gs.Seats[1].Hand))
	}
}

// ============================================================================
// VERIFICATION TESTS: GainLife handles all variants
// ============================================================================

func TestGainLifeGeneric_EachPlayer(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Congregate", 0, 0, "instant")

	e := &gameast.GainLife{
		Amount: *gameast.NumInt(4),
		Target: gameast.EachPlayer(),
	}
	ResolveEffect(gs, src, e)

	if gs.Seats[0].Life != 24 {
		t.Errorf("expected seat 0 life 24, got %d", gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != 24 {
		t.Errorf("expected seat 1 life 24, got %d", gs.Seats[1].Life)
	}
}

func TestGainLifeGeneric_DefaultsToController(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Heroes' Reunion", 0, 0, "instant")

	// AST emits "you" or "controller" for self-targeting life gain.
	e := &gameast.GainLife{
		Amount: *gameast.NumInt(7),
		Target: gameast.Filter{Base: "you"},
	}
	ResolveEffect(gs, src, e)

	if gs.Seats[0].Life != 27 {
		t.Errorf("expected seat 0 life 27, got %d", gs.Seats[0].Life)
	}
}

func TestGainLifeGeneric_NoTargetDefaultsToController(t *testing.T) {
	gs := newFixtureGame(t)
	// Use a source with no other permanents on the battlefield,
	// so PickTarget returns nil and the default kicks in.
	gs.Seats[0].Battlefield = nil
	src := &Permanent{
		Card:       &Card{Name: "Healing Salve", Types: []string{"instant"}},
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}

	e := &gameast.GainLife{
		Amount: *gameast.NumInt(3),
		Target: gameast.Filter{}, // truly empty → defaults to controller
	}
	ResolveEffect(gs, src, e)

	if gs.Seats[0].Life != 23 {
		t.Errorf("expected seat 0 life 23, got %d", gs.Seats[0].Life)
	}
}

// ============================================================================
// VERIFICATION TESTS: Damage handles all variants
// ============================================================================

func TestDamageGeneric_EachOpponent(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Gray Merchant", 2, 4, "creature")

	e := &gameast.Damage{
		Amount: *gameast.NumInt(5),
		Target: gameast.EachOpponent(),
	}
	ResolveEffect(gs, src, e)

	if gs.Seats[1].Life != 15 {
		t.Errorf("expected opponent life 15, got %d", gs.Seats[1].Life)
	}
}

func TestDamageGeneric_EachCreature(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Pyroclasm", 0, 0, "sorcery")
	a := addBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")
	b := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	e := &gameast.Damage{
		Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "creature", Quantifier: "each"},
	}
	ResolveEffect(gs, src, e)

	if a.MarkedDamage != 2 {
		t.Errorf("expected Llanowar Elves marked damage 2, got %d", a.MarkedDamage)
	}
	if b.MarkedDamage != 2 {
		t.Errorf("expected Grizzly Bears marked damage 2, got %d", b.MarkedDamage)
	}
}

// ============================================================================
// VERIFICATION TESTS: cardMatchesTutorFilter edge cases
// ============================================================================

func TestCardMatchesTutorFilter_BasicLand(t *testing.T) {
	forest := &Card{Name: "Forest", Types: []string{"land", "basic"}}
	island := &Card{Name: "Island", Types: []string{"land", "basic"}}
	shockland := &Card{Name: "Stomping Ground", Types: []string{"land"}}

	basicLandFilter := gameast.Filter{Base: "basic_land"}
	if !cardMatchesTutorFilter(forest, basicLandFilter) {
		t.Error("Forest should match basic_land filter")
	}
	if !cardMatchesTutorFilter(island, basicLandFilter) {
		t.Error("Island should match basic_land filter")
	}
	if cardMatchesTutorFilter(shockland, basicLandFilter) {
		t.Error("Stomping Ground should NOT match basic_land filter")
	}
}

func TestCardMatchesTutorFilter_ArtifactOrEnchantment(t *testing.T) {
	solRing := &Card{Name: "Sol Ring", Types: []string{"artifact"}}
	smothering := &Card{Name: "Smothering Tithe", Types: []string{"enchantment"}}
	creature := &Card{Name: "Grizzly Bears", Types: []string{"creature"}}

	filter := gameast.Filter{Base: "artifact_or_enchantment"}
	if !cardMatchesTutorFilter(solRing, filter) {
		t.Error("Sol Ring should match artifact_or_enchantment")
	}
	if !cardMatchesTutorFilter(smothering, filter) {
		t.Error("Smothering Tithe should match artifact_or_enchantment")
	}
	if cardMatchesTutorFilter(creature, filter) {
		t.Error("Grizzly Bears should NOT match artifact_or_enchantment")
	}
}

func TestCardMatchesTutorFilter_NilCard(t *testing.T) {
	filter := gameast.Filter{Base: "creature"}
	if cardMatchesTutorFilter(nil, filter) {
		t.Error("nil card should not match any filter")
	}
}

// ============================================================================
// VERIFICATION TESTS: matchesPermanent with Extra filters
// ============================================================================

func TestMatchesPermanent_NonlandExtra(t *testing.T) {
	gs := newFixtureGame(t)
	land := addBattlefield(gs, 0, "Forest", 0, 0, "land")
	creature := addBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")

	filter := gameast.Filter{
		Base:  "permanent",
		Extra: []string{"nonland"},
	}

	if matchesPermanent(filter, land) {
		t.Error("land should NOT match nonland permanent filter")
	}
	if !matchesPermanent(filter, creature) {
		t.Error("creature should match nonland permanent filter")
	}
}

func TestMatchesPermanent_TappedExtra(t *testing.T) {
	gs := newFixtureGame(t)
	tapped := addBattlefield(gs, 0, "Tapped Creature", 2, 2, "creature")
	tapped.Tapped = true
	untapped := addBattlefield(gs, 0, "Untapped Creature", 2, 2, "creature")

	filter := gameast.Filter{
		Base:  "creature",
		Extra: []string{"tapped"},
	}

	if !matchesPermanent(filter, tapped) {
		t.Error("tapped creature should match tapped filter")
	}
	if matchesPermanent(filter, untapped) {
		t.Error("untapped creature should NOT match tapped filter")
	}
}

func TestMatchesPermanent_ColorFilter(t *testing.T) {
	gs := newFixtureGame(t)
	_ = gs
	blackCreature := &Permanent{
		Card:     &Card{Name: "Nantuko Shade", Types: []string{"creature"}, Colors: []string{"B"}},
		Counters: map[string]int{},
		Flags:    map[string]int{},
	}
	whiteCreature := &Permanent{
		Card:     &Card{Name: "Serra Angel", Types: []string{"creature"}, Colors: []string{"W"}},
		Counters: map[string]int{},
		Flags:    map[string]int{},
	}

	filter := gameast.Filter{
		Base:        "creature",
		ColorFilter: []string{"B"},
	}

	if !matchesPermanent(filter, blackCreature) {
		t.Error("black creature should match black color filter")
	}
	if matchesPermanent(filter, whiteCreature) {
		t.Error("white creature should NOT match black color filter")
	}
}

func TestMatchesPermanent_ManaValueConstraint(t *testing.T) {
	gs := newFixtureGame(t)
	_ = gs
	mv := 3
	cheapCreature := &Permanent{
		Card:     &Card{Name: "Llanowar Elves", Types: []string{"creature"}, CMC: 1},
		Counters: map[string]int{},
		Flags:    map[string]int{},
	}
	expensiveCreature := &Permanent{
		Card:     &Card{Name: "Primeval Titan", Types: []string{"creature"}, CMC: 6},
		Counters: map[string]int{},
		Flags:    map[string]int{},
	}

	filter := gameast.Filter{
		Base:        "creature",
		ManaValueOp: "<=",
		ManaValue:   &mv,
	}

	if !matchesPermanent(filter, cheapCreature) {
		t.Error("1-CMC creature should match <= 3 filter")
	}
	if matchesPermanent(filter, expensiveCreature) {
		t.Error("6-CMC creature should NOT match <= 3 filter")
	}
}
