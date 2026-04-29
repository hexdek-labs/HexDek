package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ============================================================================
// Test helpers
// ============================================================================

func newP0Game(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return NewGameState(2, rng, nil)
}

func addP0Battlefield(gs *GameState, seat int, name string, pow, tough int, types ...string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         append([]string{}, types...),
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func addP0BattlefieldWithKeyword(gs *GameState, seat int, name string, pow, tough int, keyword string, types ...string) *Permanent {
	p := addP0Battlefield(gs, seat, name, pow, tough, types...)
	p.Card.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: keyword},
		},
	}
	return p
}

func addP0HandCard(gs *GameState, seat int, name string, cost int, types ...string) *Card {
	t := append([]string{}, types...)
	if cost > 0 {
		t = append(t, "cost:"+itoa(cost))
	}
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: t,
		CMC:   cost,
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

func addP0Library(gs *GameState, seat int, names ...string) {
	for _, n := range names {
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, &Card{
			Name:  n,
			Owner: seat,
			Types: []string{"creature"},
		})
	}
}

func addP0LibraryTyped(gs *GameState, seat int, name string, types ...string) {
	gs.Seats[seat].Library = append(gs.Seats[seat].Library, &Card{
		Name:  name,
		Owner: seat,
		Types: append([]string{}, types...),
	})
}

func p0CountEvents(gs *GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// ============================================================================
// 1. Cycling Tests — CR §702.29
// ============================================================================

func TestCycling_BasicDrawsCard(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 5

	// Create a card with cycling {2} in hand.
	card := addP0HandCard(gs, 0, "Renewed Faith", 2, "instant")
	card.AST = &gameast.CardAST{
		Name: "Renewed Faith",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "cycling", Args: []interface{}{float64(2)}},
		},
	}

	// Add cards to library so we can draw.
	addP0Library(gs, 0, "A", "B", "C")

	err := ActivateCycling(gs, 0, card)
	if err != nil {
		t.Fatalf("cycling should succeed: %v", err)
	}

	// Mana should be reduced by 2.
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("mana should be 3 after paying {2}, got %d", gs.Seats[0].ManaPool)
	}

	// Card should be in graveyard.
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Error("cycled card should be in graveyard")
	}

	// Card should NOT be in hand.
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Error("cycled card should not be in hand")
		}
	}

	// Should have drawn one card (library had A, B, C; hand should now have A).
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("should have 1 card in hand after drawing, got %d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Hand[0].Name != "A" {
		t.Errorf("drawn card should be A, got %s", gs.Seats[0].Hand[0].Name)
	}

	// Should have cycling event.
	if p0CountEvents(gs, "cycling") != 1 {
		t.Errorf("expected 1 cycling event, got %d", p0CountEvents(gs, "cycling"))
	}
}

func TestCycling_InsufficientMana(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 1

	card := addP0HandCard(gs, 0, "Shefet Monitor", 2, "creature")
	card.AST = &gameast.CardAST{
		Name: "Shefet Monitor",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "cycling", Args: []interface{}{float64(2)}},
		},
	}
	addP0Library(gs, 0, "X")

	err := ActivateCycling(gs, 0, card)
	if err == nil {
		t.Fatal("cycling with insufficient mana should fail")
	}

	// Card should still be in hand.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("card should remain in hand, hand size %d", len(gs.Seats[0].Hand))
	}
}

func TestCycling_NoCyclingKeyword(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 5

	card := addP0HandCard(gs, 0, "Lightning Bolt", 1, "instant")
	addP0Library(gs, 0, "X")

	err := ActivateCycling(gs, 0, card)
	if err == nil {
		t.Fatal("cycling on card without cycling should fail")
	}
}

func TestCycling_Typecycling(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 5

	// Card with swampcycling {2}.
	card := addP0HandCard(gs, 0, "Twisted Abomination", 2, "creature")
	card.AST = &gameast.CardAST{
		Name: "Twisted Abomination",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "swampcycling", Args: []interface{}{float64(2)}},
		},
	}

	// Library has a creature and a swamp.
	addP0LibraryTyped(gs, 0, "Bear", "creature")
	addP0LibraryTyped(gs, 0, "Swamp", "land", "swamp")
	addP0LibraryTyped(gs, 0, "Mountain", "land", "mountain")

	err := ActivateCycling(gs, 0, card)
	if err != nil {
		t.Fatalf("typecycling should succeed: %v", err)
	}

	// Should have found the Swamp (it should be in hand).
	foundSwamp := false
	for _, c := range gs.Seats[0].Hand {
		if c.Name == "Swamp" {
			foundSwamp = true
			break
		}
	}
	if !foundSwamp {
		t.Error("typecycling should find a Swamp and put it in hand")
	}

	// Library should be missing the Swamp.
	for _, c := range gs.Seats[0].Library {
		if c.Name == "Swamp" {
			t.Error("Swamp should have been removed from library")
		}
	}

	if p0CountEvents(gs, "typecycling_search") != 1 {
		t.Error("expected typecycling_search event")
	}
}

func TestCyclingCost_Extraction(t *testing.T) {
	card := &Card{
		Name: "Akroma's Vengeance",
		AST: &gameast.CardAST{
			Name: "Akroma's Vengeance",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "cycling", Args: []interface{}{float64(3)}},
			},
		},
	}
	cost, ok := CyclingCost(card)
	if !ok {
		t.Fatal("should detect cycling")
	}
	if cost != 3 {
		t.Errorf("cycling cost should be 3, got %d", cost)
	}
}

func TestCyclingCost_NoCycling(t *testing.T) {
	card := &Card{Name: "Bear"}
	_, ok := CyclingCost(card)
	if ok {
		t.Fatal("should not detect cycling on a vanilla card")
	}
}

// ============================================================================
// 2. Crew Tests — CR §702.122
// ============================================================================

func TestCrew_BasicCrewSuccess(t *testing.T) {
	gs := newP0Game(t)

	// Create a vehicle with crew 3.
	vehicle := addP0Battlefield(gs, 0, "Heart of Kiran", 4, 4, "artifact", "vehicle")
	vehicle.Card.AST = &gameast.CardAST{
		Name: "Heart of Kiran",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "crew", Args: []interface{}{float64(3)}},
		},
	}

	// Create a 3/3 creature to crew with.
	crew1 := addP0Battlefield(gs, 0, "Bear", 3, 3, "creature")

	err := CrewVehicle(gs, 0, vehicle, []*Permanent{crew1})
	if err != nil {
		t.Fatalf("crew should succeed: %v", err)
	}

	// Crew creature should be tapped.
	if !crew1.Tapped {
		t.Error("crew creature should be tapped")
	}

	// Vehicle should be a creature now.
	if !vehicle.IsCreature() {
		t.Error("vehicle should be a creature after crewing")
	}

	// Should have crew event.
	if p0CountEvents(gs, "crew") != 1 {
		t.Error("expected crew event")
	}
}

func TestCrew_InsufficientPower(t *testing.T) {
	gs := newP0Game(t)

	vehicle := addP0Battlefield(gs, 0, "Smuggler's Copter", 3, 3, "artifact", "vehicle")
	vehicle.Card.AST = &gameast.CardAST{
		Name: "Smuggler's Copter",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "crew", Args: []interface{}{float64(3)}},
		},
	}

	// Only a 1/1.
	crew1 := addP0Battlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")

	err := CrewVehicle(gs, 0, vehicle, []*Permanent{crew1})
	if err == nil {
		t.Fatal("crew with insufficient power should fail")
	}

	// Creature should NOT be tapped.
	if crew1.Tapped {
		t.Error("crew creature should not be tapped on failure")
	}
}

func TestCrew_MultipleCreatures(t *testing.T) {
	gs := newP0Game(t)

	vehicle := addP0Battlefield(gs, 0, "Esika's Chariot", 4, 4, "artifact", "vehicle")
	vehicle.Card.AST = &gameast.CardAST{
		Name: "Esika's Chariot",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "crew", Args: []interface{}{float64(4)}},
		},
	}

	crew1 := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")
	crew2 := addP0Battlefield(gs, 0, "Wolf", 3, 3, "creature")

	err := CrewVehicle(gs, 0, vehicle, []*Permanent{crew1, crew2})
	if err != nil {
		t.Fatalf("crew with multiple creatures should succeed: %v", err)
	}

	if !crew1.Tapped || !crew2.Tapped {
		t.Error("all crew creatures should be tapped")
	}
	if !vehicle.IsCreature() {
		t.Error("vehicle should become creature")
	}
}

func TestCrew_NotVehicle(t *testing.T) {
	gs := newP0Game(t)
	nonVehicle := addP0Battlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	crew1 := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")

	err := CrewVehicle(gs, 0, nonVehicle, []*Permanent{crew1})
	if err == nil {
		t.Fatal("should fail crewing a non-vehicle")
	}
}

func TestCrew_UncrewAtEOT(t *testing.T) {
	gs := newP0Game(t)

	vehicle := addP0Battlefield(gs, 0, "Heart of Kiran", 4, 4, "artifact", "vehicle")
	vehicle.Card.AST = &gameast.CardAST{
		Name: "Heart of Kiran",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "crew", Args: []interface{}{float64(3)}},
		},
	}
	crew1 := addP0Battlefield(gs, 0, "Bear", 3, 3, "creature")

	_ = CrewVehicle(gs, 0, vehicle, []*Permanent{crew1})
	if !vehicle.IsCreature() {
		t.Fatal("vehicle should be creature after crewing")
	}

	UncrewVehiclesAtEOT(gs)

	if vehicle.IsCreature() {
		t.Error("vehicle should lose creature type at EOT")
	}
}

func TestIsVehicle(t *testing.T) {
	p := &Permanent{
		Card: &Card{Name: "Copter", Types: []string{"artifact", "vehicle"}},
	}
	if !IsVehicle(p) {
		t.Error("should detect vehicle")
	}

	p2 := &Permanent{
		Card: &Card{Name: "Sol Ring", Types: []string{"artifact"}},
	}
	if IsVehicle(p2) {
		t.Error("should not detect non-vehicle")
	}
}

// ============================================================================
// 3. Convoke Tests — CR §702.51
// ============================================================================

func TestConvoke_CostReduction(t *testing.T) {
	gs := newP0Game(t)

	// Add 3 untapped creatures.
	addP0Battlefield(gs, 0, "A", 1, 1, "creature")
	addP0Battlefield(gs, 0, "B", 1, 1, "creature")
	addP0Battlefield(gs, 0, "C", 1, 1, "creature")

	// One tapped creature (should not count).
	tapped := addP0Battlefield(gs, 0, "D", 1, 1, "creature")
	tapped.Tapped = true

	reduction := ConvokeCostReduction(gs, 0)
	if reduction != 3 {
		t.Errorf("convoke reduction should be 3 (3 untapped creatures), got %d", reduction)
	}
}

func TestConvoke_WiredIntoCostModifiers(t *testing.T) {
	gs := newP0Game(t)

	// Add 2 untapped creatures for convoke.
	addP0Battlefield(gs, 0, "A", 1, 1, "creature")
	addP0Battlefield(gs, 0, "B", 1, 1, "creature")

	// Card with convoke and CMC 5.
	card := &Card{
		Name:  "Stoke the Flames",
		Owner: 0,
		Types: []string{"instant", "cost:5"},
		CMC:   5,
		AST: &gameast.CardAST{
			Name: "Stoke the Flames",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "convoke"},
			},
		},
	}

	totalCost := CalculateTotalCost(gs, card, 0)
	// Base cost 5, convoke reduces by 2 (2 untapped creatures), net = 3.
	if totalCost != 3 {
		t.Errorf("convoke should reduce cost from 5 to 3, got %d", totalCost)
	}
}

func TestHasConvoke(t *testing.T) {
	card := &Card{
		Name: "Venerated Loxodon",
		AST: &gameast.CardAST{
			Name: "Venerated Loxodon",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "convoke"},
			},
		},
	}
	if !HasConvoke(card) {
		t.Error("should detect convoke")
	}

	noConvoke := &Card{Name: "Bear"}
	if HasConvoke(noConvoke) {
		t.Error("should not detect convoke on vanilla card")
	}
}

// ============================================================================
// 4. Infect Tests — CR §702.90
// ============================================================================

func TestInfect_DealsPoisonToPlayer(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	src := addP0BattlefieldWithKeyword(gs, 0, "Glistener Elf", 1, 1, "infect", "creature")
	setPermFlag(src, flagAttacking, true)

	applyCombatDamageToPlayer(gs, src, 1, 1)

	// Should gain poison, not lose life.
	if gs.Seats[1].PoisonCounters != 1 {
		t.Errorf("defender should have 1 poison counter, got %d", gs.Seats[1].PoisonCounters)
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("defender life should remain 20 (infect = poison, not life loss), got %d", gs.Seats[1].Life)
	}

	// Should have poison event.
	if p0CountEvents(gs, "poison") != 1 {
		t.Errorf("expected 1 poison event, got %d", p0CountEvents(gs, "poison"))
	}
}

func TestInfect_DealsMinusCountersToCreature(t *testing.T) {
	gs := newP0Game(t)

	src := addP0BattlefieldWithKeyword(gs, 0, "Phyrexian Crusader", 2, 2, "infect", "creature")
	target := addP0Battlefield(gs, 1, "Bear", 3, 3, "creature")

	applyCombatDamageToCreature(gs, src, 2, target)

	// Should have -1/-1 counters, NOT marked damage.
	if target.Counters["-1/-1"] != 2 {
		t.Errorf("target should have 2 -1/-1 counters, got %d", target.Counters["-1/-1"])
	}
	if target.MarkedDamage != 0 {
		t.Errorf("infect should not deal marked damage, got %d", target.MarkedDamage)
	}

	// Target's effective toughness should be 1 (3 - 2).
	if target.Toughness() != 1 {
		t.Errorf("target toughness should be 1, got %d", target.Toughness())
	}

	if p0CountEvents(gs, "infect_counters") != 1 {
		t.Error("expected infect_counters event")
	}
}

func TestInfect_NormalDamageStillWorks(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[1].Life = 20

	// Non-infect creature.
	src := addP0Battlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	applyCombatDamageToPlayer(gs, src, 2, 1)

	if gs.Seats[1].Life != 18 {
		t.Errorf("normal damage should reduce life, got %d", gs.Seats[1].Life)
	}
	if gs.Seats[1].PoisonCounters != 0 {
		t.Error("non-infect should not give poison counters")
	}
}

func TestInfect_PoisonSBAKills(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[1].PoisonCounters = 10

	StateBasedActions(gs)

	if !gs.Seats[1].Lost {
		t.Error("player with 10 poison counters should lose (SBA §704.5c)")
	}
}

// ============================================================================
// 5. Shroud Tests — CR §702.18
// ============================================================================

func TestShroud_CantBeTargeted(t *testing.T) {
	gs := newP0Game(t)

	shrouded := addP0BattlefieldWithKeyword(gs, 1, "Argothian Enchantress", 0, 1, "shroud", "creature")
	_ = shrouded

	// Try targeting from opponent — should be filtered out.
	f := gameast.Filter{Base: "creature", Targeted: true}
	targets := pickPermanentTarget(gs, f, 0, nil)
	for _, tgt := range targets {
		if tgt.Permanent != nil && tgt.Permanent.Card.Name == "Argothian Enchantress" {
			t.Error("shrouded creature should not be targetable by opponent")
		}
	}

	// Also can't be targeted by controller.
	targets = pickPermanentTarget(gs, f, 1, nil)
	for _, tgt := range targets {
		if tgt.Permanent != nil && tgt.Permanent.Card.Name == "Argothian Enchantress" {
			t.Error("shrouded creature should not be targetable by controller either")
		}
	}
}

func TestShroud_UntargetedBypassesShroud(t *testing.T) {
	gs := newP0Game(t)

	shrouded := addP0BattlefieldWithKeyword(gs, 1, "Troll Ascetic", 3, 2, "shroud", "creature")

	// Untargeted filter — should include shrouded creature.
	f := gameast.Filter{Base: "creature", Quantifier: "each", Targeted: false}
	targets := allPermanentTargets(gs, f, 0)
	found := false
	for _, tgt := range targets {
		if tgt.Permanent == shrouded {
			found = true
		}
	}
	if !found {
		t.Error("untargeted 'each creature' should include shrouded creatures")
	}
}

func TestHexproof_OppCantTarget_ControllerCan(t *testing.T) {
	gs := newP0Game(t)

	hexproofed := addP0BattlefieldWithKeyword(gs, 0, "Geist of Saint Traft", 2, 2, "hexproof", "creature")

	// Opponent can't target.
	if CanBeTargetedBy(hexproofed, 1) {
		t.Error("hexproof creature should not be targetable by opponent")
	}

	// Controller CAN target.
	if !CanBeTargetedBy(hexproofed, 0) {
		t.Error("hexproof creature should be targetable by controller")
	}
}

// ============================================================================
// 6. Affinity for Artifacts Tests — CR §702.41
// ============================================================================

func TestAffinity_ReducesCostByArtifactCount(t *testing.T) {
	gs := newP0Game(t)

	// Put 3 artifacts on the battlefield.
	addP0Battlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	addP0Battlefield(gs, 0, "Mana Vault", 0, 0, "artifact")
	addP0Battlefield(gs, 0, "Mox Opal", 0, 0, "artifact")

	card := &Card{
		Name:  "Frogmite",
		Owner: 0,
		Types: []string{"artifact", "creature", "cost:4"},
		CMC:   4,
		AST: &gameast.CardAST{
			Name: "Frogmite",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "affinity for artifacts", Raw: "affinity for artifacts"},
			},
		},
	}

	cost := CalculateTotalCost(gs, card, 0)
	// Base 4, minus 3 artifacts = 1.
	if cost != 1 {
		t.Errorf("affinity should reduce cost from 4 to 1, got %d", cost)
	}
}

func TestAffinity_CantGoBelowZero(t *testing.T) {
	gs := newP0Game(t)

	// Put 5 artifacts on the battlefield.
	for i := 0; i < 5; i++ {
		addP0Battlefield(gs, 0, "Artifact "+itoa(i), 0, 0, "artifact")
	}

	card := &Card{
		Name:  "Frogmite",
		Owner: 0,
		Types: []string{"artifact", "creature", "cost:4"},
		CMC:   4,
		AST: &gameast.CardAST{
			Name: "Frogmite",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "affinity for artifacts", Raw: "affinity for artifacts"},
			},
		},
	}

	cost := CalculateTotalCost(gs, card, 0)
	// Base 4, minus 5 (floor 0) = 0.
	if cost != 0 {
		t.Errorf("affinity cost should floor at 0, got %d", cost)
	}
}

func TestHasAffinityForArtifacts(t *testing.T) {
	card := &Card{
		Name: "Myr Enforcer",
		AST: &gameast.CardAST{
			Name: "Myr Enforcer",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "affinity for artifacts", Raw: "affinity for artifacts"},
			},
		},
	}
	if !HasAffinityForArtifacts(card) {
		t.Error("should detect affinity for artifacts")
	}

	// Also test "affinity" with raw containing "artifact".
	card2 := &Card{
		Name: "Myr Enforcer2",
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "affinity", Raw: "affinity for artifacts"},
			},
		},
	}
	if !HasAffinityForArtifacts(card2) {
		t.Error("should detect affinity via raw text")
	}
}

func TestCountArtifacts(t *testing.T) {
	gs := newP0Game(t)
	addP0Battlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	addP0Battlefield(gs, 0, "Mana Crypt", 0, 0, "artifact")
	addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")

	if CountArtifacts(gs, 0) != 2 {
		t.Errorf("should count 2 artifacts, got %d", CountArtifacts(gs, 0))
	}
	if CountArtifacts(gs, 1) != 0 {
		t.Errorf("seat 1 should have 0 artifacts, got %d", CountArtifacts(gs, 1))
	}
}

// ============================================================================
// 7. Exalted Tests — CR §702.83
// ============================================================================

func TestExalted_SingleAttackerGetsBonus(t *testing.T) {
	gs := newP0Game(t)

	// Attacker with exalted.
	attacker := addP0BattlefieldWithKeyword(gs, 0, "Noble Hierarch", 0, 1, "exalted", "creature")
	_ = attacker

	// Lone attacker (different creature).
	fighter := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")

	// Simulate exalted: 1 instance of exalted, 1 lone attacker.
	ApplyExalted(gs, 0, fighter)

	// Fighter should get +1/+1.
	if fighter.Power() != 3 {
		t.Errorf("exalted should give +1 power, got %d", fighter.Power())
	}
	if fighter.Toughness() != 3 {
		t.Errorf("exalted should give +1 toughness, got %d", fighter.Toughness())
	}

	if p0CountEvents(gs, "exalted") != 1 {
		t.Error("expected exalted event")
	}
}

func TestExalted_MultipleInstances(t *testing.T) {
	gs := newP0Game(t)

	// Three permanents with exalted.
	addP0BattlefieldWithKeyword(gs, 0, "Noble Hierarch", 0, 1, "exalted", "creature")
	addP0BattlefieldWithKeyword(gs, 0, "Qasali Pridemage", 2, 2, "exalted", "creature")
	addP0BattlefieldWithKeyword(gs, 0, "Rafiq of the Many", 3, 3, "exalted", "creature")

	fighter := addP0Battlefield(gs, 0, "Soldier Token", 1, 1, "creature")

	ApplyExalted(gs, 0, fighter)

	// Fighter should get +3/+3.
	if fighter.Power() != 4 {
		t.Errorf("3 exalted should give +3 power, got %d", fighter.Power())
	}
	if fighter.Toughness() != 4 {
		t.Errorf("3 exalted should give +3 toughness, got %d", fighter.Toughness())
	}
}

func TestExalted_NoExaltedNoBonus(t *testing.T) {
	gs := newP0Game(t)

	fighter := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")

	ApplyExalted(gs, 0, fighter)

	// No exalted permanents, no bonus.
	if fighter.Power() != 2 {
		t.Errorf("no exalted should mean no bonus, power=%d", fighter.Power())
	}

	if p0CountEvents(gs, "exalted") != 0 {
		t.Error("no exalted events when count is 0")
	}
}

func TestCountExalted(t *testing.T) {
	gs := newP0Game(t)
	addP0BattlefieldWithKeyword(gs, 0, "A", 1, 1, "exalted", "creature")
	addP0BattlefieldWithKeyword(gs, 0, "B", 1, 1, "exalted", "creature")
	addP0Battlefield(gs, 0, "C", 1, 1, "creature") // no exalted

	if CountExalted(gs, 0) != 2 {
		t.Errorf("should count 2 exalted, got %d", CountExalted(gs, 0))
	}
}

// ============================================================================
// 8. Landwalk Tests — CR §702.14
// ============================================================================

func TestLandwalk_SwampwalkUnblockable(t *testing.T) {
	gs := newP0Game(t)

	attacker := addP0BattlefieldWithKeyword(gs, 0, "Filth", 1, 1, "swampwalk", "creature")
	blocker := addP0Battlefield(gs, 1, "Bear", 2, 2, "creature")

	// Defender controls a Swamp.
	addP0Battlefield(gs, 1, "Swamp", 0, 0, "land", "swamp")

	// Should not be blockable.
	if canBlockGS(gs, attacker, blocker) {
		t.Error("swampwalk creature should be unblockable when defender controls a Swamp")
	}
}

func TestLandwalk_NoMatchingLand(t *testing.T) {
	gs := newP0Game(t)

	attacker := addP0BattlefieldWithKeyword(gs, 0, "Filth", 1, 1, "swampwalk", "creature")
	blocker := addP0Battlefield(gs, 1, "Bear", 2, 2, "creature")

	// Defender controls a Mountain, not a Swamp.
	addP0Battlefield(gs, 1, "Mountain", 0, 0, "land", "mountain")

	// Should be blockable.
	if !canBlockGS(gs, attacker, blocker) {
		t.Error("swampwalk creature should be blockable when defender has no Swamp")
	}
}

func TestLandwalk_Islandwalk(t *testing.T) {
	gs := newP0Game(t)

	attacker := addP0BattlefieldWithKeyword(gs, 0, "Wonder", 2, 2, "islandwalk", "creature")
	blocker := addP0Battlefield(gs, 1, "Bear", 2, 2, "creature")

	addP0Battlefield(gs, 1, "Island", 0, 0, "land", "island")

	if canBlockGS(gs, attacker, blocker) {
		t.Error("islandwalk should be unblockable when defender controls an Island")
	}
}

func TestLandwalkType_Detection(t *testing.T) {
	p := &Permanent{
		Card: &Card{Name: "Filth"},
		Flags: map[string]int{},
	}
	p.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "swampwalk"},
		},
	}

	lt := LandwalkType(p)
	if lt != "swamp" {
		t.Errorf("should detect swampwalk land type 'swamp', got '%s'", lt)
	}
}

func TestLandwalkType_NoLandwalk(t *testing.T) {
	p := &Permanent{
		Card: &Card{Name: "Bear"},
	}
	lt := LandwalkType(p)
	if lt != "" {
		t.Errorf("should return empty for non-landwalk, got '%s'", lt)
	}
}

func TestDefenderControlsLandType(t *testing.T) {
	gs := newP0Game(t)
	addP0Battlefield(gs, 1, "Swamp", 0, 0, "land", "swamp")

	if !DefenderControlsLandType(gs, 1, "swamp") {
		t.Error("should detect swamp on defender's battlefield")
	}
	if DefenderControlsLandType(gs, 1, "island") {
		t.Error("should not detect island when only swamp present")
	}
}

// ============================================================================
// 9. Bestow Tests — CR §702.103
// ============================================================================

func TestBestow_CastAsAura(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 10

	card := &Card{
		Name:          "Boon Satyr",
		Owner:         0,
		Types:         []string{"creature", "enchantment"},
		BasePower:     4,
		BaseToughness: 2,
		CMC:           5,
		AST: &gameast.CardAST{
			Name: "Boon Satyr",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "bestow", Args: []interface{}{float64(5)}},
			},
		},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	target := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")

	err := CastWithBestow(gs, 0, card, target)
	if err != nil {
		t.Fatalf("bestow should succeed: %v", err)
	}

	// Mana should be reduced.
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("mana should be 5 after paying 5, got %d", gs.Seats[0].ManaPool)
	}

	// Card should not be in hand.
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Error("card should be removed from hand")
		}
	}

	// Target should have +4/+2 from bestow.
	if target.Power() != 6 {
		t.Errorf("target should have power 6 (2+4), got %d", target.Power())
	}
	if target.Toughness() != 4 {
		t.Errorf("target should have toughness 4 (2+2), got %d", target.Toughness())
	}

	if p0CountEvents(gs, "bestow") != 1 {
		t.Error("expected bestow event")
	}
}

func TestBestow_Falloff(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 10

	card := &Card{
		Name:          "Nighthowler",
		Owner:         0,
		Types:         []string{"creature", "enchantment"},
		BasePower:     3,
		BaseToughness: 3,
		CMC:           4,
		AST: &gameast.CardAST{
			Name: "Nighthowler",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "bestow", Args: []interface{}{float64(4)}},
			},
		},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	target := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")
	_ = CastWithBestow(gs, 0, card, target)

	// Find the bestowed permanent.
	var bestowPerm *Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == card {
			bestowPerm = p
			break
		}
	}
	if bestowPerm == nil {
		t.Fatal("bestowed permanent should be on battlefield")
	}

	// Simulate target leaving — remove from battlefield.
	gs.Seats[0].Battlefield = []*Permanent{bestowPerm}

	// Run bestow falloff check.
	changed := CheckBestowFalloffs(gs)
	if !changed {
		t.Error("bestow falloff should trigger when target leaves")
	}

	// Bestowed permanent should now be a creature, not an aura.
	if bestowPerm.AttachedTo != nil {
		t.Error("bestow falloff should clear AttachedTo")
	}
	if bestowPerm.Flags["bestowed"] != 0 {
		t.Error("bestow falloff should clear bestowed flag")
	}

	// Should have creature type.
	if !bestowPerm.IsCreature() {
		t.Error("bestowed permanent should become a creature after falloff")
	}

	if p0CountEvents(gs, "bestow_falloff") != 1 {
		t.Error("expected bestow_falloff event")
	}
}

func TestHasBestow(t *testing.T) {
	card := &Card{
		Name: "Boon Satyr",
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "bestow", Args: []interface{}{float64(5)}},
			},
		},
	}
	if !HasBestow(card) {
		t.Error("should detect bestow")
	}

	vanilla := &Card{Name: "Bear"}
	if HasBestow(vanilla) {
		t.Error("should not detect bestow on vanilla card")
	}
}

func TestBestow_InsufficientMana(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 2

	card := &Card{
		Name:          "Boon Satyr",
		Owner:         0,
		Types:         []string{"creature", "enchantment"},
		BasePower:     4,
		BaseToughness: 2,
		CMC:           5,
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "bestow", Args: []interface{}{float64(5)}},
			},
		},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	target := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")

	err := CastWithBestow(gs, 0, card, target)
	if err == nil {
		t.Fatal("bestow with insufficient mana should fail")
	}
}

// ============================================================================
// 10. Devoid Tests — CR §702.114
// ============================================================================

func TestDevoid_CardLevel(t *testing.T) {
	card := &Card{
		Name:   "Eldrazi Scion",
		Colors: []string{"R"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "devoid"},
			},
		},
	}

	if !CardHasDevoid(card) {
		t.Error("should detect devoid")
	}

	ApplyDevoid(card)
	if len(card.Colors) != 0 {
		t.Errorf("devoid should clear colors, got %v", card.Colors)
	}
}

func TestDevoid_NoDevoid(t *testing.T) {
	card := &Card{
		Name:   "Bear",
		Colors: []string{"G"},
	}

	if CardHasDevoid(card) {
		t.Error("should not detect devoid on vanilla card")
	}

	ApplyDevoid(card)
	if len(card.Colors) != 1 || card.Colors[0] != "G" {
		t.Error("non-devoid card should keep its colors")
	}
}

// ============================================================================
// Integration test: Exalted fires during DeclareAttackers
// ============================================================================

func TestExalted_FiresDuringDeclareAttackers(t *testing.T) {
	gs := newP0Game(t)

	// A single attacker that is not exalted.
	fighter := addP0Battlefield(gs, 0, "Wild Nacatl", 3, 3, "creature")
	_ = fighter

	// An exalted permanent.
	addP0BattlefieldWithKeyword(gs, 0, "Noble Hierarch", 0, 1, "exalted", "creature")

	// Need an opponent with life.
	gs.Seats[1].Life = 20

	attackers := DeclareAttackers(gs, 0)

	if len(attackers) != 1 {
		t.Fatalf("expected 1 attacker, got %d", len(attackers))
	}

	// The lone attacker should have exalted bonus.
	atk := attackers[0]
	if atk.Card.Name == "Wild Nacatl" {
		if atk.Power() != 4 {
			t.Errorf("exalted should boost Wild Nacatl from 3 to 4 power, got %d", atk.Power())
		}
	}

	if p0CountEvents(gs, "exalted") != 1 {
		t.Errorf("expected exalted event, got %d", p0CountEvents(gs, "exalted"))
	}
}

func TestExalted_MultipleAttackersNoExalted(t *testing.T) {
	gs := newP0Game(t)

	addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")
	addP0Battlefield(gs, 0, "Wolf", 3, 3, "creature")
	addP0BattlefieldWithKeyword(gs, 0, "Noble Hierarch", 0, 1, "exalted", "creature")

	gs.Seats[1].Life = 20

	attackers := DeclareAttackers(gs, 0)

	// Multiple attackers — exalted should NOT fire.
	// (Noble Hierarch also attacks, so 3 creatures attack.)
	if p0CountEvents(gs, "exalted") != 0 {
		t.Error("exalted should not fire when multiple creatures attack")
	}
	_ = attackers
}
