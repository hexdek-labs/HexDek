package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Freerunning tests — CR §702.173 (OTJ Outlaws of Thunder Junction)
// ---------------------------------------------------------------------------

func newFreerunningGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(169))
	return NewGameState(2, rng, nil)
}

// newFreerunningInstant builds an instant card with the freerunning
// keyword for `freerunningMana` (mana-string arg). normalCMC is the
// printed mana cost the spell would otherwise pay if cast normally.
func newFreerunningInstant(name string, owner, normalCMC int, freerunningMana string) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   normalCMC,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "freerunning", Args: []any{freerunningMana}},
			},
		},
	}
}

// markCombatDamageDealtBy sets the seat flag the engine writes when a
// creature controlled by `seatIdx` deals combat damage to a player —
// the same key combat.go writes via
// gs.Seats[src.Controller].Flags["creature_dealt_combat_damage_to_player"]++.
func markCombatDamageDealtBy(gs *GameState, seatIdx int) {
	if gs.Seats[seatIdx].Flags == nil {
		gs.Seats[seatIdx].Flags = map[string]int{}
	}
	gs.Seats[seatIdx].Flags["creature_dealt_combat_damage_to_player"]++
}

// ---------------------------------------------------------------------------
// HasFreerunning / FreerunningCost
// ---------------------------------------------------------------------------

func TestHasFreerunning_Detects(t *testing.T) {
	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	if !HasFreerunning(card) {
		t.Fatal("HasFreerunning should be true for a card with freerunning keyword")
	}
}

func TestHasFreerunning_Negative(t *testing.T) {
	card := &Card{
		Name:  "Plain Bear",
		Types: []string{"creature"},
		AST:   &gameast.CardAST{Name: "Plain Bear", Abilities: []gameast.Ability{}},
	}
	if HasFreerunning(card) {
		t.Fatal("HasFreerunning must be false on vanilla card")
	}
}

func TestHasFreerunning_Nil(t *testing.T) {
	if HasFreerunning(nil) {
		t.Fatal("HasFreerunning(nil) must be false")
	}
}

func TestFreerunningCost_ParsesManaString(t *testing.T) {
	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	if got := FreerunningCost(card); got != 2 {
		t.Fatalf("FreerunningCost = %d, want 2 (for {1}{B})", got)
	}
}

func TestFreerunningCost_NumericArg(t *testing.T) {
	card := &Card{
		Name:  "Numeric",
		Owner: 0,
		Types: []string{"instant"},
		CMC:   4,
		AST: &gameast.CardAST{
			Name: "Numeric",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "freerunning", Args: []any{3}},
			},
		},
	}
	if got := FreerunningCost(card); got != 3 {
		t.Fatalf("FreerunningCost = %d, want 3 (numeric arg)", got)
	}
}

func TestFreerunningCost_NoKeyword(t *testing.T) {
	card := &Card{
		Name:  "Plain",
		Types: []string{"instant"},
		AST:   &gameast.CardAST{Name: "Plain", Abilities: []gameast.Ability{}},
	}
	if got := FreerunningCost(card); got != 0 {
		t.Fatalf("FreerunningCost on non-freerunning card = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// (a) succeeds after outlaw combat damage
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_HappyPath_AfterCombatDamage(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2

	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	markCombatDamageDealtBy(gs, 0)

	result, err := CastWithFreerunning(gs, 0, card, 2)
	if err != nil {
		t.Fatalf("CastWithFreerunning error: %v", err)
	}
	if result == nil {
		t.Fatal("CastWithFreerunning returned nil result")
	}

	// Card removed from hand.
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Fatal("card should have been removed from hand")
		}
	}
	// Stack item pushed.
	if len(gs.Stack) != 1 {
		t.Fatalf("Stack len = %d, want 1", len(gs.Stack))
	}
	if gs.Stack[0].Card != card {
		t.Errorf("top of stack card mismatch")
	}
}

// ---------------------------------------------------------------------------
// (b) rejected without prior outlaw combat damage
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_RejectedWithoutCombatDamage(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2

	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	// No combat-damage flag set — predicate fails.
	if CanCastForFreerunning(gs, 0) {
		t.Fatal("CanCastForFreerunning should be false without combat-damage flag (test scaffolding bug)")
	}

	_, err := CastWithFreerunning(gs, 0, card, 2)
	if err == nil {
		t.Fatal("CastWithFreerunning should fail when freerunning is not enabled")
	}
	// Card untouched.
	stillInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			stillInHand = true
		}
	}
	if !stillInHand {
		t.Error("card should remain in hand after rejected cast")
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("ManaPool = %d, want 2 (no mana spent on rejected cast)", gs.Seats[0].ManaPool)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("Stack len = %d, want 0 (nothing pushed)", len(gs.Stack))
	}
}

// ---------------------------------------------------------------------------
// (c) CostMeta stamped
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_StampsCostMeta(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2

	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	markCombatDamageDealtBy(gs, 0)

	if _, err := CastWithFreerunning(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithFreerunning error: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("Stack len = %d, want 1", len(gs.Stack))
	}
	item := gs.Stack[0]
	if !IsFreerunningCast(item) {
		t.Error("IsFreerunningCast should be true on the stack item")
	}
	if got, _ := item.CostMeta["freerunning_cast"].(bool); !got {
		t.Errorf("CostMeta[freerunning_cast] = %v, want true", item.CostMeta["freerunning_cast"])
	}
	if got, _ := item.CostMeta["freerunning_cost"].(int); got != 2 {
		t.Errorf("CostMeta[freerunning_cost] = %v, want 2", item.CostMeta["freerunning_cost"])
	}
	if got, _ := item.CostMeta["alt_cost_keyword"].(string); got != "freerunning" {
		t.Errorf("CostMeta[alt_cost_keyword] = %v, want \"freerunning\"", item.CostMeta["alt_cost_keyword"])
	}
}

// ---------------------------------------------------------------------------
// (d) reduced cost actually charged (not the printed normal CMC)
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_ChargesFreerunningCostNotNormalCost(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	// Mana pool exactly enough for the freerunning cost, NOT for the
	// normal cost — proves the helper charged the alt cost, not the
	// printed mana cost.
	gs.Seats[0].ManaPool = 2

	// normalCMC=4, freerunning cost {1}{B} = 2.
	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	markCombatDamageDealtBy(gs, 0)

	if _, err := CastWithFreerunning(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithFreerunning error: %v", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("ManaPool after cast = %d, want 0 (2 paid for freerunning)", gs.Seats[0].ManaPool)
	}
	// Verify the pay_mana event recorded the freerunning amount, NOT the
	// printed CMC.
	foundFreerunningPay := false
	for _, e := range gs.EventLog {
		if e.Kind != "pay_mana" {
			continue
		}
		if e.Details == nil {
			continue
		}
		if r, _ := e.Details["reason"].(string); r != "freerunning_cast" {
			continue
		}
		if e.Amount != 2 {
			t.Errorf("pay_mana for freerunning_cast Amount = %d, want 2", e.Amount)
		}
		foundFreerunningPay = true
	}
	if !foundFreerunningPay {
		t.Error("expected pay_mana event with reason=freerunning_cast")
	}
}

// ---------------------------------------------------------------------------
// Auto-resolve printed cost via -1 sentinel
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_PrintedCostSentinel(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{2}{B}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	markCombatDamageDealtBy(gs, 0)

	// -1 means "use the printed FreerunningCost" — should resolve to 3.
	if _, err := CastWithFreerunning(gs, 0, card, -1); err != nil {
		t.Fatalf("CastWithFreerunning error: %v", err)
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("ManaPool = %d, want 2 (5 - 3 freerunning)", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// Insufficient mana
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_RejectsInsufficientMana(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1

	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	markCombatDamageDealtBy(gs, 0)

	if _, err := CastWithFreerunning(gs, 0, card, 2); err == nil {
		t.Fatal("CastWithFreerunning should reject when mana pool < freerunning cost")
	}
	stillInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			stillInHand = true
		}
	}
	if !stillInHand {
		t.Error("card should remain in hand on failed cast")
	}
}

// ---------------------------------------------------------------------------
// Missing keyword
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_RejectsMissingKeyword(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	card := &Card{
		Name:  "Plain Instant",
		Types: []string{"instant"},
		CMC:   2,
		AST:   &gameast.CardAST{Name: "Plain Instant", Abilities: []gameast.Ability{}},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	markCombatDamageDealtBy(gs, 0)

	if _, err := CastWithFreerunning(gs, 0, card, 2); err == nil {
		t.Fatal("CastWithFreerunning should reject a card without freerunning")
	}
}

// ---------------------------------------------------------------------------
// Card not in hand
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_RejectsCardNotInHand(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	// NOT added to hand.
	markCombatDamageDealtBy(gs, 0)

	if _, err := CastWithFreerunning(gs, 0, card, 2); err == nil {
		t.Fatal("CastWithFreerunning should reject when card is not in hand")
	}
}

// ---------------------------------------------------------------------------
// Seat-level "freerunning cast this turn" marker
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_SetsThisTurnFlag(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2

	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	markCombatDamageDealtBy(gs, 0)

	if SpellFreerunningCastThisTurn(gs, 0) {
		t.Fatal("SpellFreerunningCastThisTurn should be false before any cast")
	}
	if _, err := CastWithFreerunning(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithFreerunning error: %v", err)
	}
	if !SpellFreerunningCastThisTurn(gs, 0) {
		t.Error("SpellFreerunningCastThisTurn should be true after successful cast")
	}
}

// ---------------------------------------------------------------------------
// Predicate dependence — CanCastForFreerunning gate is the authority
// ---------------------------------------------------------------------------

func TestCastWithFreerunning_UsesExistingPredicate(t *testing.T) {
	gs := newFreerunningGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2

	card := newFreerunningInstant("Caustic Bronco", 0, 4, "{1}{B}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	// Predicate must be false initially.
	if CanCastForFreerunning(gs, 0) {
		t.Fatal("predicate should be false before any combat damage")
	}
	if _, err := CastWithFreerunning(gs, 0, card, 2); err == nil {
		t.Fatal("cast should fail when predicate is false")
	}

	// Flip the predicate via the existing seat-flag convention.
	markCombatDamageDealtBy(gs, 0)
	if !CanCastForFreerunning(gs, 0) {
		t.Fatal("predicate should be true after combat-damage flag set")
	}
	if _, err := CastWithFreerunning(gs, 0, card, 2); err != nil {
		t.Fatalf("cast should succeed when predicate is true: %v", err)
	}
}
