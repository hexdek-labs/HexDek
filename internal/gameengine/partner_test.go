package gameengine

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// Partner-commander tests — CR §702.124 / §903.3c.
//
// Covers:
//   - Setup with 2 commanders places both in command zone, 2 names, 40 life.
//   - Independent tax tracking: casting Kraum 3× doesn't tax Tymna.
//   - Independent damage tracking: 15 Kraum + 10 Tymna = survival; 21 Kraum
//     alone = loss (CR §704.6c).
//   - Partner legality validator: bare Partner, Partner with X, Friends
//     Forever, Doctor+Companion, Background+chooser, illegal pairs.
// -----------------------------------------------------------------------------

// makePartnerCard builds a synthetic legendary creature with the bare
// "Partner" keyword parsed as gameast.Keyword{Raw: "partner"}. Owner is
// deferred to SetupCommanderGame.
func makePartnerCard(name string) *Card {
	ast := &gameast.CardAST{
		Name:        name,
		FullyParsed: true,
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "partner", Raw: "partner"},
		},
	}
	return &Card{
		Name:          name,
		AST:           ast,
		BasePower:     2,
		BaseToughness: 2,
		Types:         []string{"legendary", "creature"},
	}
}

// makePartnerWithCard builds a card with the targeted "Partner with X"
// keyword (CR §702.124g).
func makePartnerWithCard(name, partnerWithName string) *Card {
	ast := &gameast.CardAST{
		Name:        name,
		FullyParsed: true,
		Abilities: []gameast.Ability{
			&gameast.Keyword{
				Name: "partner",
				Raw:  "partner with " + partnerWithName,
			},
		},
	}
	return &Card{
		Name:          name,
		AST:           ast,
		BasePower:     2,
		BaseToughness: 2,
		Types:         []string{"legendary", "creature"},
	}
}

// makeKeywordCard builds a card with a single bare keyword (Friends
// Forever, Doctor's Companion, Choose a Background).
func makeKeywordCard(name, rawKeyword string, types ...string) *Card {
	ast := &gameast.CardAST{
		Name:        name,
		FullyParsed: true,
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: strings.Fields(rawKeyword)[0], Raw: rawKeyword},
		},
	}
	if len(types) == 0 {
		types = []string{"legendary", "creature"}
	}
	return &Card{
		Name:          name,
		AST:           ast,
		BasePower:     2,
		BaseToughness: 2,
		Types:         types,
	}
}

// -----------------------------------------------------------------------------
// Setup: two commanders in command zone, names populated, life = 40
// -----------------------------------------------------------------------------

func TestPartner_SetupPlacesBothInCommandZone(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	kraum := makePartnerCard("Kraum, Ludevic's Opus")
	tymna := makePartnerCard("Tymna the Weaver")
	decks := []*CommanderDeck{
		{CommanderCards: []*Card{kraum, tymna}},
		{CommanderCards: []*Card{makePartnerCard("C")}},
		{CommanderCards: []*Card{makePartnerCard("D")}},
		{CommanderCards: []*Card{makePartnerCard("E")}},
	}
	SetupCommanderGame(gs, decks)

	if gs.Seats[0].Life != 40 {
		t.Fatalf("seat 0 life should be 40, got %d", gs.Seats[0].Life)
	}
	if len(gs.Seats[0].CommandZone) != 2 {
		t.Fatalf("partner deck should have 2 commanders in CZ, got %d", len(gs.Seats[0].CommandZone))
	}
	if len(gs.Seats[0].CommanderNames) != 2 {
		t.Fatalf("partner deck should have 2 commander names, got %d", len(gs.Seats[0].CommanderNames))
	}
	names := gs.Seats[0].CommanderNames
	if names[0] != "Kraum, Ludevic's Opus" || names[1] != "Tymna the Weaver" {
		t.Fatalf("commander names wrong: %v", names)
	}
	// Single-commander seats unaffected.
	if len(gs.Seats[1].CommandZone) != 1 {
		t.Fatalf("single-commander seat 1 CZ size should be 1, got %d", len(gs.Seats[1].CommandZone))
	}
}

// -----------------------------------------------------------------------------
// Independent tax
// -----------------------------------------------------------------------------

// Casting Kraum from the command zone three times should tax Kraum
// (0, 2, 4) but leave Tymna's tax at 0.
func TestPartner_IndependentCastTax(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	kraum := makePartnerCard("Kraum, Ludevic's Opus") // base CMC 4
	tymna := makePartnerCard("Tymna the Weaver")      // base CMC 3
	other := makePartnerCard("Sidekick")
	SetupCommanderGame(gs, []*CommanderDeck{
		{CommanderCards: []*Card{kraum, tymna}},
		{CommanderCards: []*Card{other}},
	})
	gs.Seats[0].ManaPool = 100

	// First Kraum cast: pay 4, tax(Kraum) → 1, tax(Tymna) still 0.
	if err := CastCommanderFromCommandZone(gs, 0, "Kraum, Ludevic's Opus", 4); err != nil {
		t.Fatalf("first Kraum cast failed: %v", err)
	}
	if gs.Seats[0].CommanderCastCounts["Kraum, Ludevic's Opus"] != 1 {
		t.Fatalf("Kraum tax after 1 cast: want 1, got %d",
			gs.Seats[0].CommanderCastCounts["Kraum, Ludevic's Opus"])
	}
	if gs.Seats[0].CommanderCastCounts["Tymna the Weaver"] != 0 {
		t.Fatalf("Tymna tax should still be 0, got %d",
			gs.Seats[0].CommanderCastCounts["Tymna the Weaver"])
	}
	// Simulate Kraum returning to command zone.
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone,
		gs.Stack[len(gs.Stack)-1].Card)
	gs.Stack = gs.Stack[:len(gs.Stack)-1]

	// Second Kraum cast: pay 4+2*1 = 6, tax(Kraum) → 2.
	if err := CastCommanderFromCommandZone(gs, 0, "Kraum, Ludevic's Opus", 4); err != nil {
		t.Fatalf("second Kraum cast failed: %v", err)
	}
	if gs.Seats[0].CommanderCastCounts["Kraum, Ludevic's Opus"] != 2 {
		t.Fatalf("Kraum tax after 2 casts: want 2, got %d",
			gs.Seats[0].CommanderCastCounts["Kraum, Ludevic's Opus"])
	}
	if gs.Seats[0].CommanderCastCounts["Tymna the Weaver"] != 0 {
		t.Fatal("Tymna tax should STILL be 0 even after Kraum has been cast twice")
	}
	// Mana: 100 - 4 - 6 = 90.
	if gs.Seats[0].ManaPool != 90 {
		t.Fatalf("mana after 2 Kraum casts: want 90, got %d", gs.Seats[0].ManaPool)
	}

	// Now cast Tymna first time: pay base 3, tax(Tymna) → 1.
	if err := CastCommanderFromCommandZone(gs, 0, "Tymna the Weaver", 3); err != nil {
		t.Fatalf("first Tymna cast failed: %v", err)
	}
	if gs.Seats[0].CommanderCastCounts["Tymna the Weaver"] != 1 {
		t.Fatalf("Tymna tax after 1 cast: want 1, got %d",
			gs.Seats[0].CommanderCastCounts["Tymna the Weaver"])
	}
	// Kraum tax should still be 2, untouched by Tymna's cast.
	if gs.Seats[0].CommanderCastCounts["Kraum, Ludevic's Opus"] != 2 {
		t.Fatal("Kraum tax should stay at 2 after a Tymna cast")
	}
	// Mana: 90 - 3 = 87.
	if gs.Seats[0].ManaPool != 87 {
		t.Fatalf("mana after 2 Kraum + 1 Tymna: want 87, got %d", gs.Seats[0].ManaPool)
	}
}

// -----------------------------------------------------------------------------
// Independent damage
// -----------------------------------------------------------------------------

// 15 damage from Kraum + 10 from Tymna on the same pilot = NO loss, because
// neither single commander crossed 21 (CR §704.6c "the same commander").
// This is the marquee partner invariant.
func TestPartner_DamageFromEachAccruesIndependently(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	kraum := makePartnerCard("Kraum, Ludevic's Opus")
	tymna := makePartnerCard("Tymna the Weaver")
	SetupCommanderGame(gs, []*CommanderDeck{
		{CommanderCards: []*Card{kraum, tymna}},
		{CommanderCards: []*Card{makePartnerCard("C")}},
		{CommanderCards: []*Card{makePartnerCard("D")}},
		{CommanderCards: []*Card{makePartnerCard("E")}},
	})
	// Both Kraum and Tymna belong to seat 0. Seat 1 is the victim pilot.
	AccumulateCommanderDamage(gs, 1, 0, "Kraum, Ludevic's Opus", 15)
	AccumulateCommanderDamage(gs, 1, 0, "Tymna the Weaver", 10)
	StateBasedActions(gs)
	if gs.Seats[1].Lost {
		t.Fatalf("seat 1 should SURVIVE: 15 Kraum + 10 Tymna < 21 each. loss=%q",
			gs.Seats[1].LossReason)
	}
	// Verify totals are in the right buckets.
	if got := CommanderDamageFrom(gs.Seats[1], 0, "Kraum, Ludevic's Opus"); got != 15 {
		t.Fatalf("Kraum→seat1 bucket: want 15, got %d", got)
	}
	if got := CommanderDamageFrom(gs.Seats[1], 0, "Tymna the Weaver"); got != 10 {
		t.Fatalf("Tymna→seat1 bucket: want 10, got %d", got)
	}
}

// 21 damage from ONLY Kraum = loss, regardless of Tymna's 0.
func TestPartner_DamageFromOneCommanderCan21(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	kraum := makePartnerCard("Kraum, Ludevic's Opus")
	tymna := makePartnerCard("Tymna the Weaver")
	SetupCommanderGame(gs, []*CommanderDeck{
		{CommanderCards: []*Card{kraum, tymna}},
		{CommanderCards: []*Card{makePartnerCard("C")}},
		{CommanderCards: []*Card{makePartnerCard("D")}},
		{CommanderCards: []*Card{makePartnerCard("E")}},
	})
	AccumulateCommanderDamage(gs, 1, 0, "Kraum, Ludevic's Opus", 21)
	StateBasedActions(gs)
	if !gs.Seats[1].Lost {
		t.Fatal("seat 1 should lose: 21 Kraum damage crosses §704.6c threshold")
	}
	if !strings.Contains(gs.Seats[1].LossReason, "Kraum") {
		t.Fatalf("loss reason should cite Kraum, got %q", gs.Seats[1].LossReason)
	}
}

// -----------------------------------------------------------------------------
// ValidatePartnerPair legality matrix
// -----------------------------------------------------------------------------

func TestValidatePartnerPair_BothBarePartner(t *testing.T) {
	a := makePartnerCard("Kraum, Ludevic's Opus")
	b := makePartnerCard("Tymna the Weaver")
	if err := ValidatePartnerPair([]*Card{a, b}); err != nil {
		t.Fatalf("Kraum+Tymna should be legal (both Partner), got %v", err)
	}
}

func TestValidatePartnerPair_SingleCommanderOK(t *testing.T) {
	a := makePartnerCard("Edgar Markov")
	if err := ValidatePartnerPair([]*Card{a}); err != nil {
		t.Fatalf("single commander should be legal, got %v", err)
	}
}

func TestValidatePartnerPair_OnlyOneHasPartner(t *testing.T) {
	a := makePartnerCard("Kraum, Ludevic's Opus")
	b := &Card{
		Name: "Edgar Markov", Types: []string{"legendary", "creature"},
		AST: &gameast.CardAST{Name: "Edgar Markov", FullyParsed: true},
	}
	if err := ValidatePartnerPair([]*Card{a, b}); err == nil {
		t.Fatal("Kraum + non-partner should be illegal")
	}
}

func TestValidatePartnerPair_PartnerWithMatched(t *testing.T) {
	a := makePartnerWithCard("Will Kenrith", "Rowan Kenrith")
	b := makePartnerWithCard("Rowan Kenrith", "Will Kenrith")
	if err := ValidatePartnerPair([]*Card{a, b}); err != nil {
		t.Fatalf("Will+Rowan (Partner with) should be legal, got %v", err)
	}
}

func TestValidatePartnerPair_PartnerWithMismatched(t *testing.T) {
	a := makePartnerWithCard("Will Kenrith", "Rowan Kenrith")
	b := makePartnerWithCard("Pia Nalaar", "Kiran Nalaar")
	if err := ValidatePartnerPair([]*Card{a, b}); err == nil {
		t.Fatal("Will Kenrith + Pia Nalaar should be illegal (named partners differ)")
	}
}

func TestValidatePartnerPair_FriendsForever(t *testing.T) {
	a := makeKeywordCard("Commander A", "friends forever")
	b := makeKeywordCard("Commander B", "friends forever")
	if err := ValidatePartnerPair([]*Card{a, b}); err != nil {
		t.Fatalf("Friends Forever pair should be legal, got %v", err)
	}
}

func TestValidatePartnerPair_BackgroundAndChooser(t *testing.T) {
	// "Choose a Background" legendary creature.
	chooser := makeKeywordCard("Wilson, Refined Grizzly", "choose a background")
	// Background-typed enchantment.
	bg := &Card{
		Name:  "Raised by Giants",
		Types: []string{"legendary", "enchantment", "background"},
		AST:   &gameast.CardAST{Name: "Raised by Giants", FullyParsed: true},
	}
	if err := ValidatePartnerPair([]*Card{chooser, bg}); err != nil {
		t.Fatalf("Choose-a-Background + Background should be legal, got %v", err)
	}
}

func TestValidatePartnerPair_TooMany(t *testing.T) {
	a := makePartnerCard("A")
	b := makePartnerCard("B")
	c := makePartnerCard("C")
	if err := ValidatePartnerPair([]*Card{a, b, c}); err == nil {
		t.Fatal("3 commanders should be illegal")
	}
}

func TestValidatePartnerPair_Empty(t *testing.T) {
	if err := ValidatePartnerPair(nil); err == nil {
		t.Fatal("no commanders should error")
	}
}

// -----------------------------------------------------------------------------
// §903.9b replacement: each partner registers independently
// -----------------------------------------------------------------------------

// Kraum dying should put Kraum back in the command zone without touching
// Tymna's entry.
func TestPartner_Kraum903_9bIndependentOfTymna(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	kraum := makePartnerCard("Kraum, Ludevic's Opus")
	tymna := makePartnerCard("Tymna the Weaver")
	SetupCommanderGame(gs, []*CommanderDeck{
		{CommanderCards: []*Card{kraum, tymna}},
		{CommanderCards: []*Card{makePartnerCard("X")}},
	})
	// Pretend Kraum was on battlefield — strip Kraum from CZ, keep Tymna.
	filtered := gs.Seats[0].CommandZone[:0]
	for _, c := range gs.Seats[0].CommandZone {
		if c != kraum {
			filtered = append(filtered, c)
		}
	}
	gs.Seats[0].CommandZone = filtered
	dest := FireZoneChange(gs, nil, kraum, 0, "battlefield", "hand")
	if dest != "command_zone" {
		t.Fatalf("Kraum's §903.9b redirect should route to command_zone, got %s", dest)
	}
	// Command zone should contain Tymna (index 0) + Kraum (index 1).
	if len(gs.Seats[0].CommandZone) != 2 {
		t.Fatalf("CZ should have 2 after Kraum returns, got %d", len(gs.Seats[0].CommandZone))
	}
	names := map[string]bool{}
	for _, c := range gs.Seats[0].CommandZone {
		names[c.DisplayName()] = true
	}
	if !names["Kraum, Ludevic's Opus"] || !names["Tymna the Weaver"] {
		t.Fatalf("CZ should contain both Kraum + Tymna: %v", names)
	}
}
