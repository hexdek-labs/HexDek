package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Test helpers — Cleave (CR §702.148)
// ---------------------------------------------------------------------------

func newCleaveGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(31))
	return NewGameState(2, rng, nil)
}

// cleaveHandCard builds a sorcery with two effect variants on its AST:
//
//   - First Activated: "Draw `baseDraw` cards." (default / bracketed
//     variant, the more restricted one.)
//   - Second Activated: "Draw `cleaveDraw` cards." (cleave / brackets-
//     removed variant, the more permissive one.)
//
// The card also carries the cleave keyword printed at `cleaveCost`.
// `baseCMC` is the printed mana cost for a normal (non-cleave) cast;
// `cleaveCost` is the alt cost the cleave keyword charges.
func cleaveHandCard(gs *GameState, seat int, name string, baseCMC, cleaveCost, baseDraw, cleaveDraw int) *Card {
	c := &Card{
		Name:  name,
		Owner: seat,
		CMC:   baseCMC,
		Types: []string{"sorcery"},
		Colors: []string{"U"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				// Cleave keyword (declares the keyword and its arg).
				&gameast.Keyword{Name: "cleave", Args: []any{float64(cleaveCost)}},
				// Base / bracketed effect.
				&gameast.Activated{
					Effect: &gameast.Draw{
						Count:  *gameast.NumInt(baseDraw),
						Target: gameast.Filter{Base: "controller"},
					},
					Raw: "Draw a [restricted] card.",
				},
				// Cleave / brackets-removed effect.
				&gameast.Activated{
					Effect: &gameast.Draw{
						Count:  *gameast.NumInt(cleaveDraw),
						Target: gameast.Filter{Base: "controller"},
					},
					Raw: "Draw a card.",
				},
			},
		},
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

// fillLibrary tops up `seat`'s library so draws have cards to take.
func fillLibrary(gs *GameState, seat, n int) {
	for i := 0; i < n; i++ {
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, &Card{
			Name: "Filler", Owner: seat, Types: []string{"land"},
		})
	}
}

// ===========================================================================
// HasCleave / CleaveCost / CleaveEffect detection
// ===========================================================================

func TestHasCleave_Detects(t *testing.T) {
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "cleave", Args: []any{float64(2)}},
			},
		},
	}
	if !HasCleave(c) {
		t.Fatal("HasCleave should detect the keyword")
	}
	if HasCleave(nil) {
		t.Fatal("HasCleave(nil) should be false")
	}
	if HasCleave(&Card{AST: &gameast.CardAST{}}) {
		t.Fatal("HasCleave should be false for a card without the keyword")
	}
}

func TestCleaveCost_ParsesArg(t *testing.T) {
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "cleave", Args: []any{float64(3)}},
			},
		},
	}
	if got := CleaveCost(c); got != 3 {
		t.Errorf("CleaveCost: want 3, got %d", got)
	}
}

func TestBaseSpellEffect_ReturnsFirstActivated(t *testing.T) {
	gs := newCleaveGame(t)
	card := cleaveHandCard(gs, 0, "Dig Up the Body", 2, 3, 1, 3)
	eff := BaseSpellEffect(card)
	if eff == nil {
		t.Fatal("BaseSpellEffect returned nil")
	}
	draw, ok := eff.(*gameast.Draw)
	if !ok {
		t.Fatalf("BaseSpellEffect: want *gameast.Draw, got %T", eff)
	}
	if v := draw.Count.Int; v != 1 {
		t.Errorf("base draw count: want 1, got %d", v)
	}
}

func TestCleaveEffect_ReturnsSecondActivated(t *testing.T) {
	gs := newCleaveGame(t)
	card := cleaveHandCard(gs, 0, "Dig Up the Body", 2, 3, 1, 3)
	eff := CleaveEffect(card)
	if eff == nil {
		t.Fatal("CleaveEffect returned nil for a card with two Activated abilities")
	}
	draw, ok := eff.(*gameast.Draw)
	if !ok {
		t.Fatalf("CleaveEffect: want *gameast.Draw, got %T", eff)
	}
	if v := draw.Count.Int; v != 3 {
		t.Errorf("cleave draw count: want 3, got %d", v)
	}
}

func TestCleaveEffect_NilWhenNoSecondVariant(t *testing.T) {
	// Card has the keyword but only ONE Activated — there's no
	// brackets-removed variant to swap to.
	c := &Card{
		Name:  "Single Effect",
		Owner: 0,
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "cleave", Args: []any{float64(2)}},
				&gameast.Activated{
					Effect: &gameast.Draw{Count: *gameast.NumInt(1), Target: gameast.Filter{Base: "controller"}},
				},
			},
		},
	}
	if CleaveEffect(c) != nil {
		t.Fatal("CleaveEffect should be nil when no second Activated effect exists")
	}
	if CleaveEffect(nil) != nil {
		t.Fatal("CleaveEffect(nil) should be nil")
	}
}

// ===========================================================================
// (a) Cleave-cast stamps CostMeta["cleave_cast"]=true
// (c-prep) item.CleaveActive flag + item.Effect swapped to cleave variant
// ===========================================================================

func TestCastWithCleave_StampsFlagAndSwapsEffect(t *testing.T) {
	gs := newCleaveGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3 // cleave cost is 3
	card := cleaveHandCard(gs, 0, "Dig Up the Body", 2, 3, 1, 3)

	res, err := CastWithCleave(gs, 0, card, 3)
	if err != nil {
		t.Fatalf("CastWithCleave: %v", err)
	}
	if res == nil {
		t.Fatal("CastWithCleave returned nil result on success")
	}

	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	item := gs.Stack[0]

	if !item.CleaveActive {
		t.Error("StackItem.CleaveActive should be true after a cleave cast")
	}
	if item.CostMeta == nil {
		t.Fatal("StackItem.CostMeta should not be nil")
	}
	if v, _ := item.CostMeta["cleave_cast"].(bool); !v {
		t.Errorf("CostMeta[cleave_cast] = %v, want true", item.CostMeta["cleave_cast"])
	}
	if v, _ := item.CostMeta["alt_cost"].(string); v != "cleave" {
		t.Errorf("CostMeta[alt_cost] = %v, want \"cleave\"", item.CostMeta["alt_cost"])
	}
	if v, _ := item.CostMeta["cleave_cost"].(int); v != 3 {
		t.Errorf("CostMeta[cleave_cost] = %v, want 3", item.CostMeta["cleave_cost"])
	}
	if item.CastZone != ZoneHand {
		t.Errorf("CastZone = %v, want ZoneHand", item.CastZone)
	}

	// Crucially: item.Effect points at the CLEAVE variant (Draw 3), not
	// the base variant (Draw 1).
	if item.Effect == nil {
		t.Fatal("StackItem.Effect should be set to the cleave variant")
	}
	draw, ok := item.Effect.(*gameast.Draw)
	if !ok {
		t.Fatalf("item.Effect: want *gameast.Draw, got %T", item.Effect)
	}
	if v := draw.Count.Int; v != 3 {
		t.Errorf("item.Effect.Count = %d, want 3 (cleave variant)", v)
	}

	// Cleave events.
	sawCleaveCast := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "cleave_cast" && ev.Amount == 3 {
			if rule, _ := ev.Details["rule"].(string); rule == "702.148a" {
				sawCleaveCast = true
				break
			}
		}
	}
	if !sawCleaveCast {
		t.Error("expected a cleave_cast event with rule 702.148a and amount 3")
	}
	if !SpellCleaveThisTurn(gs, 0) {
		t.Error("SpellCleaveThisTurn should be true after a cleave cast")
	}
}

// ===========================================================================
// (b) Base cast — collectSpellEffect picks the bracketed variant
// ===========================================================================

func TestBaseCast_UsesBracketedFirstActivated(t *testing.T) {
	// The "base cast" semantics: collectSpellEffect (what the normal
	// cast path uses to populate StackItem.Effect) returns the FIRST
	// Activated effect — the bracketed / restricted variant. The
	// cleave variant is NOT touched.
	gs := newCleaveGame(t)
	card := cleaveHandCard(gs, 0, "Dig Up the Body", 2, 3, 1, 3)

	eff := collectSpellEffect(card)
	if eff == nil {
		t.Fatal("collectSpellEffect returned nil")
	}
	draw, ok := eff.(*gameast.Draw)
	if !ok {
		t.Fatalf("base effect: want *gameast.Draw, got %T", eff)
	}
	if v := draw.Count.Int; v != 1 {
		t.Errorf("base cast should use the bracketed (smaller) variant: want draw=1, got draw=%d", v)
	}

	// Build a base-cast stack item the same way CastSpellWithCosts would
	// — Effect from collectSpellEffect, no CleaveActive, no cleave_cast
	// CostMeta — and confirm the shape.
	item := &StackItem{
		Card:       card,
		Controller: 0,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
	}
	if item.CleaveActive {
		t.Error("base-cast StackItem must not have CleaveActive=true")
	}
	d, _ := item.Effect.(*gameast.Draw)
	if d == nil || d.Count.Int != 1 {
		t.Error("base-cast StackItem must carry the bracketed draw-1 effect")
	}
}

// ===========================================================================
// (c) Cleave-cast applies the non-bracketed variant on resolve
// ===========================================================================

func TestCastWithCleave_ResolvesIntoNonBracketedVariant(t *testing.T) {
	// End-to-end: cleave-cast a card whose base draws 1 and cleave
	// draws 3, then resolve the stack. The controller should end up
	// with 3 cards in hand (proving the cleave variant ran, not the
	// base variant).
	gs := newCleaveGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3 // cleave cost
	fillLibrary(gs, 0, 10)   // plenty of cards to draw
	card := cleaveHandCard(gs, 0, "Dig Up the Body", 2, 3, 1, 3)

	if _, err := CastWithCleave(gs, 0, card, 3); err != nil {
		t.Fatalf("CastWithCleave: %v", err)
	}

	handBefore := len(gs.Seats[0].Hand)
	ResolveStackTop(gs)
	handAfter := len(gs.Seats[0].Hand)

	drawn := handAfter - handBefore
	if drawn != 3 {
		t.Fatalf("cleave variant should have drawn 3 cards (base draws 1); drew %d", drawn)
	}
}

func TestBaseCastShape_ResolvesIntoBracketedVariant(t *testing.T) {
	// Mirror of the cleave-resolve test using a hand-rolled base-cast
	// StackItem. Confirms the bracketed variant draws ONLY 1 (and not
	// the cleave 3), so the divergence in (c) above is genuinely from
	// the cleave swap and not from some other test-fixture quirk.
	gs := newCleaveGame(t)
	gs.Active = 0
	fillLibrary(gs, 0, 10)
	card := cleaveHandCard(gs, 0, "Dig Up the Body", 2, 3, 1, 3)
	// Remove from hand to mimic post-cast state.
	gs.Seats[0].Hand = gs.Seats[0].Hand[:0]

	item := &StackItem{
		Card:       card,
		Controller: 0,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
	}
	PushStackItem(gs, item)

	handBefore := len(gs.Seats[0].Hand)
	ResolveStackTop(gs)
	handAfter := len(gs.Seats[0].Hand)

	if drawn := handAfter - handBefore; drawn != 1 {
		t.Fatalf("base (bracketed) variant should draw 1 card, drew %d", drawn)
	}
}

// ===========================================================================
// (d) Cleave cost actually charged (not the printed mana cost)
// ===========================================================================

func TestCastWithCleave_PaysCleaveCostNotPrintedCMC(t *testing.T) {
	// Printed CMC is 6 (the bracketed "premium" cost); cleave cost is 2.
	// With only 4 mana, a normal cast would fail — but a cleave cast
	// must succeed and consume only 2.
	gs := newCleaveGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 4
	card := cleaveHandCard(gs, 0, "Expensive Bracketed Spell", 6, 2, 1, 3)

	if _, err := CastWithCleave(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithCleave: %v", err)
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("expected mana=2 (paid {2} cleave, not {6} CMC), got %d", gs.Seats[0].ManaPool)
	}

	// pay_mana event should reference the cleave reason + keyword.
	sawPay := false
	for _, ev := range gs.EventLog {
		if ev.Kind != "pay_mana" {
			continue
		}
		reason, _ := ev.Details["reason"].(string)
		kw, _ := ev.Details["keyword"].(string)
		if reason == "cleave_cast" && kw == "cleave" && ev.Amount == 2 {
			sawPay = true
			break
		}
	}
	if !sawPay {
		t.Error("expected a pay_mana event with reason=cleave_cast keyword=cleave amount=2")
	}
	// And no event records the printed CMC of 6.
	for _, ev := range gs.EventLog {
		if ev.Kind == "pay_mana" && ev.Amount == 6 {
			t.Error("should not have paid the printed CMC of 6")
		}
	}
}

// ===========================================================================
// Rejection paths + side-effect guards
// ===========================================================================

func TestCastWithCleave_RejectsCardWithoutKeyword(t *testing.T) {
	gs := newCleaveGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	c := &Card{Name: "Plain", Owner: 0, Types: []string{"sorcery"},
		AST: &gameast.CardAST{Name: "Plain"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)

	res, err := CastWithCleave(gs, 0, c, 2)
	if err == nil || res != nil {
		t.Fatal("CastWithCleave should reject a card without the cleave keyword")
	}
	if ce, ok := err.(*CastError); !ok || ce.Reason != "no_cleave_keyword" {
		t.Errorf("expected CastError(no_cleave_keyword), got %T %v", err, err)
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("mana should be untouched, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastWithCleave_RejectsCardWithoutCleaveVariant(t *testing.T) {
	gs := newCleaveGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	// Has the keyword, but only ONE Activated — no cleave variant to swap.
	c := &Card{
		Name: "Bracket-less", Owner: 0, Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name: "Bracket-less",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "cleave", Args: []any{float64(2)}},
				&gameast.Activated{Effect: &gameast.Draw{Count: *gameast.NumInt(1), Target: gameast.Filter{Base: "controller"}}},
			},
		},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)

	res, err := CastWithCleave(gs, 0, c, 2)
	if err == nil || res != nil {
		t.Fatal("CastWithCleave should reject when there's no cleave variant")
	}
	if ce, ok := err.(*CastError); !ok || ce.Reason != "no_cleave_variant" {
		t.Errorf("expected CastError(no_cleave_variant), got %T %v", err, err)
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("mana untouched check, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("card should remain in hand, hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestCastWithCleave_RejectsInsufficientMana(t *testing.T) {
	gs := newCleaveGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1
	card := cleaveHandCard(gs, 0, "Dig Up the Body", 2, 3, 1, 3)

	res, err := CastWithCleave(gs, 0, card, 3)
	if err == nil || res != nil {
		t.Fatal("CastWithCleave should reject when mana < cleave cost")
	}
	if ce, ok := err.(*CastError); !ok || ce.Reason != "insufficient_mana" {
		t.Errorf("expected CastError(insufficient_mana), got %T %v", err, err)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("card should remain in hand, hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("mana untouched check, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("stack should be empty after rejection, got %d", len(gs.Stack))
	}
}

func TestCastWithCleave_DefaultsToKeywordArgWhenNeg1(t *testing.T) {
	gs := newCleaveGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3
	card := cleaveHandCard(gs, 0, "Dig Up the Body", 2, 3, 1, 3)

	if _, err := CastWithCleave(gs, 0, card, -1); err != nil {
		t.Fatalf("CastWithCleave with -1 fallback: %v", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected 0 mana after paying default cleave 3; got %d", gs.Seats[0].ManaPool)
	}
	if v, _ := gs.Stack[0].CostMeta["cleave_cost"].(int); v != 3 {
		t.Errorf("CostMeta[cleave_cost] should resolve to default 3, got %d", v)
	}
}

func TestCastWithCleave_NilSafety(t *testing.T) {
	if _, err := CastWithCleave(nil, 0, nil, 0); err == nil {
		t.Fatal("CastWithCleave(nil...) should error")
	}
	gs := newCleaveGame(t)
	if _, err := CastWithCleave(gs, -1, nil, 0); err == nil {
		t.Fatal("CastWithCleave(invalid seat) should error")
	}
	if _, err := CastWithCleave(gs, 0, nil, 0); err == nil {
		t.Fatal("CastWithCleave(nil card) should error")
	}
}

func TestSpellCleaveThisTurn_FalseBeforeCast(t *testing.T) {
	gs := newCleaveGame(t)
	if SpellCleaveThisTurn(gs, 0) {
		t.Fatal("SpellCleaveThisTurn should be false before any cast")
	}
}
