package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Buyback tests — CR §702.27
// ---------------------------------------------------------------------------

// newBuybackInstant builds an instant card with the buyback keyword for
// `buybackMana` (mana-string arg). normalCMC is the printed mana cost the
// spell would otherwise pay; the test runner pays normalCMC + buybackMana
// when casting.
func newBuybackInstant(name string, owner, normalCMC int, buybackMana string) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   normalCMC,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "buyback", Args: []any{buybackMana}},
			},
		},
	}
}

func newPlainInstant(name string, owner, cmc int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

func newBuybackGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return NewGameState(2, rng, nil)
}

// ---------------------------------------------------------------------------
// HasBuyback
// ---------------------------------------------------------------------------

func TestHasBuyback_Positive(t *testing.T) {
	card := newBuybackInstant("Capsize", 0, 3, "{3}")
	if !HasBuyback(card) {
		t.Fatal("HasBuyback returned false for a card with buyback keyword")
	}
}

func TestHasBuyback_Negative(t *testing.T) {
	card := newPlainInstant("Lightning Bolt", 0, 1)
	if HasBuyback(card) {
		t.Fatal("HasBuyback returned true for a card without buyback")
	}
}

func TestHasBuyback_NilCard(t *testing.T) {
	if HasBuyback(nil) {
		t.Fatal("HasBuyback(nil) should return false")
	}
}

// ---------------------------------------------------------------------------
// BuybackCost
// ---------------------------------------------------------------------------

func TestBuybackCost_ManaString(t *testing.T) {
	card := newBuybackInstant("Whispers of the Muse", 0, 1, "{3}")
	if got := BuybackCost(card); got != 3 {
		t.Fatalf("BuybackCost = %d, want 3", got)
	}
}

func TestBuybackCost_NumericArg(t *testing.T) {
	card := &Card{
		Name:  "Custom Buyback",
		Owner: 0,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Custom Buyback",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "buyback", Args: []any{4}},
			},
		},
	}
	if got := BuybackCost(card); got != 4 {
		t.Fatalf("BuybackCost = %d, want 4 (numeric arg)", got)
	}
}

func TestBuybackCost_MissingKeyword(t *testing.T) {
	if got := BuybackCost(newPlainInstant("Bolt", 0, 1)); got != 0 {
		t.Fatalf("BuybackCost on non-buyback card = %d, want 0", got)
	}
}

func TestBuybackCost_NilCard(t *testing.T) {
	if got := BuybackCost(nil); got != 0 {
		t.Fatalf("BuybackCost(nil) = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// CastBuyback — happy path
// ---------------------------------------------------------------------------

func TestCastBuyback_PaysBothCostsAndFlagsStack(t *testing.T) {
	gs := newBuybackGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 6 // 3 normal + 3 buyback

	card := newBuybackInstant("Capsize", 0, 3, "{3}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	result, err := CastBuyback(gs, 0, card, 3, 3)
	if err != nil {
		t.Fatalf("CastBuyback returned error: %v", err)
	}
	if result == nil {
		t.Fatal("CastBuyback returned nil CostPaymentResult on success")
	}

	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("expected 0 mana left (paid 6), got %d", gs.Seats[0].ManaPool)
	}
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Fatal("bought-back card should no longer be in hand at cast time")
		}
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	item := gs.Stack[0]
	if item.Card != card {
		t.Fatal("stack item Card should reference the cast card")
	}
	if item.Controller != 0 {
		t.Fatalf("stack item Controller=%d, want 0", item.Controller)
	}
	if v, ok := item.CostMeta["bought_back"]; !ok || v != true {
		t.Fatalf("CostMeta[\"bought_back\"] = %v, want true", item.CostMeta["bought_back"])
	}
	if v, ok := item.CostMeta["buyback_cost"]; !ok || v != 3 {
		t.Fatalf("CostMeta[\"buyback_cost\"] = %v, want 3", item.CostMeta["buyback_cost"])
	}
	if gs.Flags["spell_bought_back_this_turn:0"] != 1 {
		t.Fatal("seat-level spell_bought_back_this_turn:0 should be set")
	}
}

// ---------------------------------------------------------------------------
// CastBuyback — failure paths
// ---------------------------------------------------------------------------

func TestCastBuyback_NoKeyword(t *testing.T) {
	gs := newBuybackGame(t)
	gs.Seats[0].ManaPool = 6
	card := newPlainInstant("Bolt", 0, 1)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	if _, err := CastBuyback(gs, 0, card, 1, 3); err == nil {
		t.Fatal("CastBuyback should fail for a card without buyback")
	}
	if gs.Seats[0].ManaPool != 6 {
		t.Fatalf("mana should be unchanged, got %d", gs.Seats[0].ManaPool)
	}
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			return
		}
	}
	t.Fatal("card should remain in hand after failed CastBuyback")
}

func TestCastBuyback_NotInstantOrSorcery(t *testing.T) {
	gs := newBuybackGame(t)
	gs.Seats[0].ManaPool = 10
	// Buyback keyword on a creature — corpus mistype guard.
	card := &Card{
		Name:  "Bogus Buyback Creature",
		Owner: 0,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: "Bogus Buyback Creature",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "buyback", Args: []any{"{3}"}},
			},
		},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	if _, err := CastBuyback(gs, 0, card, 2, 3); err == nil {
		t.Fatal("CastBuyback should refuse non-instant/sorcery (§702.27a)")
	}
}

func TestCastBuyback_NotInHand(t *testing.T) {
	gs := newBuybackGame(t)
	gs.Seats[0].ManaPool = 6
	card := newBuybackInstant("Capsize", 0, 3, "{3}")
	// Card NOT in hand.
	if _, err := CastBuyback(gs, 0, card, 3, 3); err == nil {
		t.Fatal("CastBuyback should fail when card is not in hand")
	}
	if gs.Seats[0].ManaPool != 6 {
		t.Fatalf("mana should be unchanged, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastBuyback_InsufficientMana(t *testing.T) {
	gs := newBuybackGame(t)
	gs.Seats[0].ManaPool = 4 // need 6 (3 + 3)
	card := newBuybackInstant("Capsize", 0, 3, "{3}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	if _, err := CastBuyback(gs, 0, card, 3, 3); err == nil {
		t.Fatal("CastBuyback should fail with insufficient mana")
	}
	found := false
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("card should remain in hand after failed CastBuyback")
	}
	if gs.Seats[0].ManaPool != 4 {
		t.Fatalf("mana should be unchanged, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastBuyback_InvalidSeat(t *testing.T) {
	gs := newBuybackGame(t)
	card := newBuybackInstant("Capsize", 0, 3, "{3}")
	if _, err := CastBuyback(gs, 99, card, 3, 3); err == nil {
		t.Fatal("CastBuyback should fail on invalid seat index")
	}
}

// ---------------------------------------------------------------------------
// Stack predicates
// ---------------------------------------------------------------------------

func TestIsBoughtBack_TrueAndFalse(t *testing.T) {
	yes := &StackItem{CostMeta: map[string]interface{}{"bought_back": true}}
	no := &StackItem{CostMeta: map[string]interface{}{"bought_back": false}}
	missing := &StackItem{}
	if !IsBoughtBack(yes) {
		t.Fatal("IsBoughtBack should return true when bought_back=true")
	}
	if IsBoughtBack(no) {
		t.Fatal("IsBoughtBack should return false when bought_back=false")
	}
	if IsBoughtBack(missing) {
		t.Fatal("IsBoughtBack should return false when CostMeta missing")
	}
	if IsBoughtBack(nil) {
		t.Fatal("IsBoughtBack(nil) should return false")
	}
}

func TestShouldReturnToHandOnResolve_MirrorsIsBoughtBack(t *testing.T) {
	yes := &StackItem{CostMeta: map[string]interface{}{"bought_back": true}}
	if !ShouldReturnToHandOnResolve(yes) {
		t.Fatal("ShouldReturnToHandOnResolve should match IsBoughtBack")
	}
	if ShouldReturnToHandOnResolve(&StackItem{}) {
		t.Fatal("ShouldReturnToHandOnResolve should be false without flag")
	}
}

// ---------------------------------------------------------------------------
// End-to-end: bought-back spell resolves back to hand, not graveyard
// ---------------------------------------------------------------------------

func TestBuyback_CastResolveReturnsToHand_EndToEnd(t *testing.T) {
	gs := newBuybackGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 6

	card := newBuybackInstant("Capsize", 0, 3, "{3}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	// 1. Cast with buyback.
	if _, err := CastBuyback(gs, 0, card, 3, 3); err != nil {
		t.Fatalf("CastBuyback failed: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("stack should have 1 item, got %d", len(gs.Stack))
	}

	// 2. Resolve — ShouldReturnToHandOnResolve branch in ResolveStackTop
	//    should route the card to its owner's hand per §702.27a.
	ResolveStackTop(gs)

	// 3. Card must be in hand, NOT in graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("bought-back spell must not land in graveyard (§702.27a)")
		}
	}
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			foundInHand = true
			break
		}
	}
	if !foundInHand {
		t.Fatal("bought-back spell should return to owner's hand on resolve")
	}
	if len(gs.Stack) != 0 {
		t.Fatalf("stack should be empty after resolution, got %d items", len(gs.Stack))
	}
}

// Regression: without the buyback flag, the same Capsize-shaped card
// resolves to the graveyard (default §608.2g path) — confirms the new
// branch doesn't change behavior for ordinary instants.
func TestBuyback_NoFlagResolvesToGraveyard(t *testing.T) {
	gs := newBuybackGame(t)
	gs.Active = 0
	card := newPlainInstant("Lightning Bolt", 0, 1)
	gs.Stack = append(gs.Stack, &StackItem{
		Card:       card,
		Controller: 0,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
	})
	ResolveStackTop(gs)
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Fatal("non-buyback spell must not return to hand")
		}
	}
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("non-buyback spell should land in graveyard on resolve (§608.2g)")
	}
}
