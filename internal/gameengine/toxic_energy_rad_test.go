package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// newTestGame creates a minimal GameState with the given seat count.
func newTestGame(t *testing.T, seats int) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return NewGameState(seats, rng, nil)
}

// addTestPerm creates a test permanent on the battlefield.
func addTestPerm(gs *GameState, seat int, name string, types ...string) *Permanent {
	card := &Card{
		Name:  name,
		Owner: seat,
		Types: append([]string{}, types...),
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

// countTestEvents counts events with the given kind.
func countTestEvents(gs *GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// ============================================================================
// Toxic tests
// ============================================================================

func TestToxic_DealsCombatDamageAddsPoisonPlusNormalDamage(t *testing.T) {
	gs := newTestGame(t, 2)
	// Create a 2/2 creature with toxic 2.
	attacker := addTestPerm(gs, 0, "Venomous Attacker", "creature")
	attacker.Card.BasePower = 2
	attacker.Card.BaseToughness = 2
	attacker.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "toxic", Args: []interface{}{float64(2)}},
		},
	}

	startingLife := gs.Seats[1].Life

	// Apply combat damage to player 1.
	applyCombatDamageToPlayer(gs, attacker, 2, 1)

	// Player 1 should have lost 2 life (normal damage).
	if gs.Seats[1].Life != startingLife-2 {
		t.Errorf("expected life %d, got %d", startingLife-2, gs.Seats[1].Life)
	}
	// Player 1 should also have 2 poison counters (toxic).
	if gs.Seats[1].PoisonCounters != 2 {
		t.Errorf("expected 2 poison counters, got %d", gs.Seats[1].PoisonCounters)
	}
}

func TestToxic_FlagBasedDetection(t *testing.T) {
	gs := newTestGame(t, 2)
	attacker := addTestPerm(gs, 0, "Toxic Creature", "creature")
	attacker.Card.BasePower = 1
	attacker.Card.BaseToughness = 1
	// Set toxic via flag (N = 3).
	attacker.Flags["kw:toxic"] = 3

	applyCombatDamageToPlayer(gs, attacker, 1, 1)

	// Should have 3 poison counters from toxic flag.
	if gs.Seats[1].PoisonCounters != 3 {
		t.Errorf("expected 3 poison counters from toxic flag, got %d", gs.Seats[1].PoisonCounters)
	}
}

func TestHasToxic_DetectsASTKeyword(t *testing.T) {
	p := &Permanent{
		Card: &Card{
			AST: &gameast.CardAST{
				Abilities: []gameast.Ability{
					&gameast.Keyword{Name: "toxic", Args: []interface{}{float64(1)}},
				},
			},
		},
		Flags: map[string]int{},
	}
	hasToxic, n := HasToxic(p)
	if !hasToxic {
		t.Fatal("expected HasToxic to return true")
	}
	if n != 1 {
		t.Errorf("expected toxic N=1, got %d", n)
	}
}

func TestHasToxic_NoToxic(t *testing.T) {
	p := &Permanent{
		Card: &Card{
			AST: &gameast.CardAST{
				Abilities: []gameast.Ability{
					&gameast.Keyword{Name: "flying"},
				},
			},
		},
		Flags: map[string]int{},
	}
	hasToxic, _ := HasToxic(p)
	if hasToxic {
		t.Fatal("expected HasToxic to return false for non-toxic creature")
	}
}

// ============================================================================
// Infect tests (verify infect replaces damage, unlike toxic)
// ============================================================================

func TestInfect_DealsPoisonInsteadOfDamage(t *testing.T) {
	gs := newTestGame(t, 2)
	attacker := addTestPerm(gs, 0, "Infect Creature", "creature")
	attacker.Card.BasePower = 3
	attacker.Card.BaseToughness = 3
	attacker.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "infect"},
		},
	}

	startingLife := gs.Seats[1].Life

	applyCombatDamageToPlayer(gs, attacker, 3, 1)

	// Infect: life should NOT change.
	if gs.Seats[1].Life != startingLife {
		t.Errorf("infect should not reduce life; expected %d, got %d", startingLife, gs.Seats[1].Life)
	}
	// Should have 3 poison counters.
	if gs.Seats[1].PoisonCounters != 3 {
		t.Errorf("expected 3 poison counters from infect, got %d", gs.Seats[1].PoisonCounters)
	}
}

// ============================================================================
// Proliferate tests
// ============================================================================

func TestProliferate_AllPlayerCounterTypes(t *testing.T) {
	gs := newTestGame(t, 2)

	// Set up: seat 0 has energy + experience, seat 1 has poison + rad.
	gs.Seats[0].Flags = map[string]int{
		"energy_counters":     3,
		"experience_counters": 2,
	}
	gs.Seats[1].PoisonCounters = 5
	gs.Seats[1].Flags = map[string]int{
		"rad_counters": 4,
	}

	// Create a source permanent for seat 0.
	src := addTestPerm(gs, 0, "Proliferator", "creature")

	// Call proliferate via ResolveEffect with a ModificationEffect.
	ResolveEffect(gs, src, &gameast.ModificationEffect{ModKind: "proliferate"})

	// Seat 0's energy should be 4 (3 + 1 proliferated).
	if gs.Seats[0].Flags["energy_counters"] != 4 {
		t.Errorf("expected energy 4, got %d", gs.Seats[0].Flags["energy_counters"])
	}
	// Seat 0's experience should be 3 (2 + 1 proliferated).
	if gs.Seats[0].Flags["experience_counters"] != 3 {
		t.Errorf("expected experience 3, got %d", gs.Seats[0].Flags["experience_counters"])
	}
	// Seat 1's poison should be 6 (5 + 1 proliferated).
	if gs.Seats[1].PoisonCounters != 6 {
		t.Errorf("expected poison 6, got %d", gs.Seats[1].PoisonCounters)
	}
	// Seat 1's rad should be 5 (4 + 1 proliferated).
	if gs.Seats[1].Flags["rad_counters"] != 5 {
		t.Errorf("expected rad 5, got %d", gs.Seats[1].Flags["rad_counters"])
	}
}

func TestProliferate_SkipsSelfPoison(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].PoisonCounters = 3

	src := addTestPerm(gs, 0, "Proliferator", "creature")
	ResolveEffect(gs, src, &gameast.ModificationEffect{ModKind: "proliferate"})

	// Self-poison should NOT be proliferated (GreedyHat policy).
	if gs.Seats[0].PoisonCounters != 3 {
		t.Errorf("expected self-poison to stay at 3, got %d", gs.Seats[0].PoisonCounters)
	}
}

// ============================================================================
// Rad counter trigger tests
// ============================================================================

func TestRadTrigger_MillsAndDamages(t *testing.T) {
	gs := newTestGame(t, 2)

	// Give seat 0 three rad counters.
	gs.Seats[0].Flags = map[string]int{"rad_counters": 3}

	// Add 3 cards to library: 2 nonlands, 1 land.
	gs.Seats[0].Library = []*Card{
		{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}},
		{Name: "Mountain", Owner: 0, Types: []string{"land"}},
		{Name: "Giant Growth", Owner: 0, Types: []string{"instant"}},
	}

	startingLife := gs.Seats[0].Life
	FireRadCounterTriggers(gs)

	// Should have milled all 3 cards.
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("expected library empty, got %d cards", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Graveyard) != 3 {
		t.Errorf("expected 3 cards in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}

	// Lost 2 life (2 nonland cards milled).
	if gs.Seats[0].Life != startingLife-2 {
		t.Errorf("expected life %d, got %d", startingLife-2, gs.Seats[0].Life)
	}

	// Rad counters: only nonland cards remove rad counters.
	// 3 milled, 2 nonland → 2 rad removed → 3-2 = 1 remaining.
	if gs.Seats[0].Flags["rad_counters"] != 1 {
		t.Errorf("expected rad counters 1 (3-2 nonland), got %d", gs.Seats[0].Flags["rad_counters"])
	}
}

func TestRadTrigger_PartialMill(t *testing.T) {
	gs := newTestGame(t, 2)

	// 5 rad counters but only 2 cards in library.
	gs.Seats[0].Flags = map[string]int{"rad_counters": 5}
	gs.Seats[0].Library = []*Card{
		{Name: "Spell 1", Owner: 0, Types: []string{"instant"}},
		{Name: "Spell 2", Owner: 0, Types: []string{"sorcery"}},
	}

	FireRadCounterTriggers(gs)

	// Only 2 cards milled (library exhausted).
	if len(gs.Seats[0].Graveyard) != 2 {
		t.Errorf("expected 2 cards in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}

	// Rad counters: 5 - 2 milled = 3 remaining.
	if gs.Seats[0].Flags["rad_counters"] != 3 {
		t.Errorf("expected rad counters 3, got %d", gs.Seats[0].Flags["rad_counters"])
	}
}

func TestRadTrigger_SkipsLostPlayers(t *testing.T) {
	gs := newTestGame(t, 2)

	gs.Seats[0].Lost = true
	gs.Seats[0].Flags = map[string]int{"rad_counters": 3}
	gs.Seats[0].Library = []*Card{
		{Name: "Card", Owner: 0, Types: []string{"instant"}},
	}

	FireRadCounterTriggers(gs)

	// Lost player should not be processed.
	if len(gs.Seats[0].Library) != 1 {
		t.Error("lost player's library should not be milled")
	}
}

// ============================================================================
// Energy payment system tests
// ============================================================================

func TestEnergy_GainAndPay(t *testing.T) {
	gs := newTestGame(t, 2)

	GainEnergy(gs, 0, 5)
	if GetEnergy(gs, 0) != 5 {
		t.Errorf("expected 5 energy, got %d", GetEnergy(gs, 0))
	}

	ok := PayEnergy(gs, 0, 3)
	if !ok {
		t.Fatal("expected PayEnergy to succeed")
	}
	if GetEnergy(gs, 0) != 2 {
		t.Errorf("expected 2 energy after paying 3, got %d", GetEnergy(gs, 0))
	}
}

func TestEnergy_InsufficientRejected(t *testing.T) {
	gs := newTestGame(t, 2)

	GainEnergy(gs, 0, 2)
	ok := PayEnergy(gs, 0, 5)
	if ok {
		t.Fatal("expected PayEnergy to fail with insufficient energy")
	}
	// Energy should be unchanged.
	if GetEnergy(gs, 0) != 2 {
		t.Errorf("expected energy unchanged at 2, got %d", GetEnergy(gs, 0))
	}
}

func TestEnergy_ZeroEnergy(t *testing.T) {
	gs := newTestGame(t, 2)

	ok := PayEnergy(gs, 0, 1)
	if ok {
		t.Fatal("expected PayEnergy to fail with zero energy")
	}
}

// ============================================================================
// Experience counter scaling tests
// ============================================================================

func TestExperienceCounterScaling(t *testing.T) {
	gs := newTestGame(t, 2)

	// Set up experience counters.
	gs.Seats[0].Flags = map[string]int{"experience_counters": 7}

	src := addTestPerm(gs, 0, "Experience Commander", "creature")

	// Test via evalScaling.
	sa := &gameast.ScalingAmount{ScalingKind: "experience_counters"}
	val, ok := evalScaling(gs, src, sa)
	if !ok {
		t.Fatal("expected evalScaling to succeed for experience_counters")
	}
	if val != 7 {
		t.Errorf("expected experience counter value 7, got %d", val)
	}
}

func TestEnergyCounterScaling(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Flags = map[string]int{"energy_counters": 4}
	src := addTestPerm(gs, 0, "Energy Card", "creature")

	sa := &gameast.ScalingAmount{ScalingKind: "energy_counters"}
	val, ok := evalScaling(gs, src, sa)
	if !ok {
		t.Fatal("expected evalScaling to succeed for energy_counters")
	}
	if val != 4 {
		t.Errorf("expected energy counter value 4, got %d", val)
	}
}

func TestRadCounterScaling(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Flags = map[string]int{"rad_counters": 6}
	src := addTestPerm(gs, 0, "Rad Card", "creature")

	sa := &gameast.ScalingAmount{ScalingKind: "rad_counters"}
	val, ok := evalScaling(gs, src, sa)
	if !ok {
		t.Fatal("expected evalScaling to succeed for rad_counters")
	}
	if val != 6 {
		t.Errorf("expected rad counter value 6, got %d", val)
	}
}

func TestExperienceCounterScaling_StrVal(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Flags = map[string]int{"experience_counters": 5}
	src := addTestPerm(gs, 0, "Mizzix", "creature")

	// Test via evalNumber with StrVal reference.
	n := gameast.NumStr("experience_counters")
	val, ok := evalNumber(gs, src, n)
	if !ok {
		t.Fatal("expected evalNumber to succeed for experience_counters string ref")
	}
	if val != 5 {
		t.Errorf("expected 5, got %d", val)
	}
}
