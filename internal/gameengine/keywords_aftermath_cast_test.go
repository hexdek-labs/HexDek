package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Aftermath cast helper tests — CR §702.127 (Amonkhet 2017)
// ---------------------------------------------------------------------------

func newAftermathGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(2128))
	gs := NewGameState(2, rng, nil)
	// Sorcery-speed timing: active player seat 0 in precombat_main with
	// an empty stack.
	gs.Active = 0
	gs.Step = "precombat_main"
	return gs
}

func newAftermathBackHalf(name string, owner, cmc int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "aftermath"},
			},
		},
	}
}

func newPlainSorceryNoAftermath(name string, owner, cmc int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

// ---------------------------------------------------------------------------
// (a) Cast from grave succeeds for an aftermath back-half
// ---------------------------------------------------------------------------

func TestCastWithAftermath_HappyPath(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 5

	card := newAftermathBackHalf("Driven // Despair (Despair half)", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	result, err := CastWithAftermath(gs, 0, card, 3)
	if err != nil {
		t.Fatalf("CastWithAftermath error: %v", err)
	}
	if result == nil {
		t.Fatal("CastWithAftermath returned nil result")
	}

	// Removed from graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Error("card should have been removed from graveyard")
		}
	}
	// Pushed onto stack.
	if len(gs.Stack) != 1 {
		t.Fatalf("Stack len = %d, want 1", len(gs.Stack))
	}
	if gs.Stack[0].Card != card {
		t.Error("top of stack should be the aftermath card")
	}
	// Mana paid.
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("ManaPool after cast = %d, want 2 (5 - 3 aftermath)", gs.Seats[0].ManaPool)
	}
}

func TestCastWithAftermath_PrintedCostSentinel(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 5

	// CMC=4 → pay 4 with cost=-1.
	card := newAftermathBackHalf("Aftermath", 0, 4)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithAftermath(gs, 0, card, -1); err != nil {
		t.Fatalf("CastWithAftermath error: %v", err)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("ManaPool = %d, want 1 (5 - 4 printed CMC)", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// (b) Cast at instant speed REJECTED (sorcery-only)
// ---------------------------------------------------------------------------

func TestCastWithAftermath_RejectsInstantSpeed_OpponentsTurn(t *testing.T) {
	gs := newAftermathGame(t)
	// Switch active to seat 1 — it's the opponent's turn now.
	gs.Active = 1
	gs.Step = "precombat_main"

	gs.Seats[0].ManaPool = 5
	card := newAftermathBackHalf("Aftermath", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	_, err := CastWithAftermath(gs, 0, card, 3)
	if err == nil {
		t.Fatal("CastWithAftermath should reject when it's not the caster's turn")
	}
	// State must be unchanged.
	stillInGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			stillInGrave = true
		}
	}
	if !stillInGrave {
		t.Error("card should remain in graveyard on rejected cast")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("ManaPool = %d, want 5 (no mana spent on rejected cast)", gs.Seats[0].ManaPool)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("Stack len = %d, want 0", len(gs.Stack))
	}
}

func TestCastWithAftermath_RejectsInstantSpeed_StackNotEmpty(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 5
	// Park something on the stack — sorcery-speed is blocked while stack
	// is non-empty.
	gs.Stack = append(gs.Stack, &StackItem{Card: &Card{Name: "X"}, Controller: 1})

	card := newAftermathBackHalf("Aftermath", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithAftermath(gs, 0, card, 3); err == nil {
		t.Fatal("CastWithAftermath should reject when stack is non-empty (sorcery-speed only)")
	}
}

func TestCastWithAftermath_RejectsInstantSpeed_NonMainPhase(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 5
	gs.Step = "declare_attackers" // combat step, not a main phase
	card := newAftermathBackHalf("Aftermath", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithAftermath(gs, 0, card, 3); err == nil {
		t.Fatal("CastWithAftermath should reject during a non-main phase")
	}
}

// ---------------------------------------------------------------------------
// (c) Cast for a non-aftermath card REJECTED
// ---------------------------------------------------------------------------

func TestCastWithAftermath_RejectsNonAftermathCard(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 5
	card := newPlainSorceryNoAftermath("Plain Sorcery", 0, 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	_, err := CastWithAftermath(gs, 0, card, 2)
	if err == nil {
		t.Fatal("CastWithAftermath should reject cards without the aftermath keyword")
	}
}

// ---------------------------------------------------------------------------
// (d) CostMeta stamped
// ---------------------------------------------------------------------------

func TestCastWithAftermath_StampsCostMeta(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 5
	card := newAftermathBackHalf("Driven", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithAftermath(gs, 0, card, 3); err != nil {
		t.Fatalf("CastWithAftermath error: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("Stack len = %d, want 1", len(gs.Stack))
	}
	item := gs.Stack[0]

	if !IsAftermathCast(item) {
		t.Error("IsAftermathCast should be true on the stack item")
	}
	if got, _ := item.CostMeta["aftermath_cast"].(bool); !got {
		t.Errorf("CostMeta[aftermath_cast] = %v, want true", item.CostMeta["aftermath_cast"])
	}
	if got, _ := item.CostMeta["zone_cast_keyword"].(string); got != "aftermath" {
		t.Errorf("CostMeta[zone_cast_keyword] = %v, want \"aftermath\"", item.CostMeta["zone_cast_keyword"])
	}
	if got, _ := item.CostMeta["aftermath_cost"].(int); got != 3 {
		t.Errorf("CostMeta[aftermath_cost] = %v, want 3", item.CostMeta["aftermath_cost"])
	}
	// Critical: the exile-on-resolve handoff.
	if got, _ := item.CostMeta["exile_on_resolve"].(bool); !got {
		t.Errorf("CostMeta[exile_on_resolve] = %v, want true", item.CostMeta["exile_on_resolve"])
	}
	if !ShouldExileOnResolve(item) {
		t.Error("ShouldExileOnResolve should be true for an aftermath cast")
	}
}

// ---------------------------------------------------------------------------
// (e) On resolve, the card goes to exile (NOT graveyard)
// ---------------------------------------------------------------------------

func TestCastWithAftermath_ResolveRoutesCardToExile(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 5
	card := newAftermathBackHalf("Despair", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithAftermath(gs, 0, card, 3); err != nil {
		t.Fatalf("CastWithAftermath error: %v", err)
	}
	// Top of stack — resolve it.
	if len(gs.Stack) != 1 {
		t.Fatalf("Stack len = %d, want 1", len(gs.Stack))
	}
	ResolveStackTop(gs)

	// Card should now be in EXILE, not in graveyard, not on battlefield.
	inExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			inExile = true
		}
	}
	if !inExile {
		t.Errorf("after resolution, aftermath card should be in exile (§702.127a); exile=%v graveyard=%v",
			gs.Seats[0].Exile, gs.Seats[0].Graveyard)
	}
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Error("aftermath card must NOT return to graveyard after resolution")
		}
	}
}

// ---------------------------------------------------------------------------
// Rejection paths
// ---------------------------------------------------------------------------

func TestCastWithAftermath_RejectsInsufficientMana(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 1
	card := newAftermathBackHalf("Aftermath", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithAftermath(gs, 0, card, 3); err == nil {
		t.Fatal("CastWithAftermath should reject when mana pool < cost")
	}
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			return // expected: card stayed in grave
		}
	}
	t.Error("card should remain in graveyard on insufficient-mana rejection")
}

func TestCastWithAftermath_RejectsCardNotInGraveyard(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 5
	card := newAftermathBackHalf("Aftermath", 0, 3)
	// NOT added to graveyard.

	if _, err := CastWithAftermath(gs, 0, card, 3); err == nil {
		t.Fatal("CastWithAftermath should reject when card is not in graveyard")
	}
}

func TestCastWithAftermath_NilSafe(t *testing.T) {
	if _, err := CastWithAftermath(nil, 0, nil, 3); err == nil {
		t.Error("nil gs should return error")
	}
	gs := newAftermathGame(t)
	if _, err := CastWithAftermath(gs, 0, nil, 3); err == nil {
		t.Error("nil card should return error")
	}
	if _, err := CastWithAftermath(gs, 99, &Card{}, 3); err == nil {
		t.Error("invalid seat should return error")
	}
}

// ---------------------------------------------------------------------------
// This-turn marker
// ---------------------------------------------------------------------------

func TestCastWithAftermath_SetsThisTurnFlag(t *testing.T) {
	gs := newAftermathGame(t)
	gs.Seats[0].ManaPool = 5
	card := newAftermathBackHalf("Aftermath", 0, 3)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if SpellAftermathCastThisTurn(gs, 0) {
		t.Fatal("SpellAftermathCastThisTurn should be false before any cast")
	}
	if _, err := CastWithAftermath(gs, 0, card, 3); err != nil {
		t.Fatalf("CastWithAftermath error: %v", err)
	}
	if !SpellAftermathCastThisTurn(gs, 0) {
		t.Error("SpellAftermathCastThisTurn should be true after a successful cast")
	}
}

func TestIsAftermathCast_NegativePaths(t *testing.T) {
	if IsAftermathCast(nil) {
		t.Error("IsAftermathCast(nil) must be false")
	}
	if IsAftermathCast(&StackItem{}) {
		t.Error("IsAftermathCast on empty item must be false")
	}
}
