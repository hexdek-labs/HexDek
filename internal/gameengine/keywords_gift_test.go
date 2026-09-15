package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Gift tests — CR §702.174
// ---------------------------------------------------------------------------

func newGiftGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(4, rand.New(rand.NewSource(59)), nil)
}

func newGiftGame2P(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(59)), nil)
}

// giftSpellCard builds a sorcery that prints the gift keyword with the
// given token type as its arg.
func giftSpellCard(name string, owner int, giftType string) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   3,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "gift", Args: []interface{}{giftType}},
			},
		},
	}
}

func nonGiftSpell(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

// ---------------------------------------------------------------------------
// (e) HasGift detector
// ---------------------------------------------------------------------------

func TestHasGift_Detects(t *testing.T) {
	if !HasGift(giftSpellCard("Bake into a Pie", 0, "Food")) {
		t.Fatal("HasGift should detect gift keyword")
	}
}

func TestHasGift_Negative(t *testing.T) {
	if HasGift(nonGiftSpell("Lightning Bolt", 0)) {
		t.Fatal("HasGift should be false for non-gift card")
	}
	if HasGift(nil) {
		t.Fatal("HasGift(nil) should be false")
	}
}

func TestGiftType_ReturnsTokenType(t *testing.T) {
	cases := []struct {
		arg, want string
	}{
		{"Treasure", "treasure"},
		{"Clue", "clue"},
		{"Food", "food"},
		{"Map", "map"},
		{"  Blood  ", "blood"},
	}
	for _, c := range cases {
		got := GiftType(giftSpellCard("X", 0, c.arg))
		if got != c.want {
			t.Errorf("GiftType(%q) = %q, want %q", c.arg, got, c.want)
		}
	}
}

func TestGiftType_NilOrNonGift(t *testing.T) {
	if GiftType(nil) != "" {
		t.Fatal("GiftType(nil) should be empty")
	}
	if GiftType(nonGiftSpell("X", 0)) != "" {
		t.Fatal("GiftType on non-gift card should be empty")
	}
}

// ---------------------------------------------------------------------------
// (a) Cast with gift promised stamps recipient + type
// ---------------------------------------------------------------------------

func TestCastWithGift_StampsRecipientAndType(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("Bake into a Pie", 0, "Food")

	if err := CastWithGift(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithGift failed: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	item := gs.Stack[0]
	if item.CostMeta == nil {
		t.Fatal("CostMeta must be populated for a gift cast")
	}
	if v, _ := item.CostMeta["gift_promised"].(bool); !v {
		t.Fatal("CostMeta[gift_promised] must be true")
	}
	if v, _ := item.CostMeta["gift_recipient"].(int); v != 2 {
		t.Fatalf("CostMeta[gift_recipient] = %v, want 2", item.CostMeta["gift_recipient"])
	}
	if v, _ := item.CostMeta["gift_type"].(string); v != "food" {
		t.Fatalf("CostMeta[gift_type] = %v, want \"food\"", item.CostMeta["gift_type"])
	}
}

func TestCastWithGift_EmitsAndFiresTrigger(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("Treasure-Gift Spell", 0, "Treasure")

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	sawCtx := map[string]interface{}{}
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "gift_promised" {
			for k, v := range ctx {
				sawCtx[k] = v
			}
		}
	}

	before := len(gs.EventLog)
	if err := CastWithGift(gs, 0, card, 1); err != nil {
		t.Fatalf("CastWithGift failed: %v", err)
	}

	// Trigger fan-out.
	if rec, _ := sawCtx["recipient_seat"].(int); rec != 1 {
		t.Fatalf("trigger ctx[recipient_seat] = %v, want 1", sawCtx["recipient_seat"])
	}
	if gt, _ := sawCtx["gift_type"].(string); gt != "treasure" {
		t.Fatalf("trigger ctx[gift_type] = %v, want \"treasure\"", sawCtx["gift_type"])
	}
	// Log event.
	sawEv := false
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "gift_promised" {
			sawEv = true
			if gs.EventLog[i].Target != 1 {
				t.Fatalf("event Target = %d, want 1 (recipient)", gs.EventLog[i].Target)
			}
		}
	}
	if !sawEv {
		t.Fatal("CastWithGift should emit a gift_promised log event")
	}
}

// ---------------------------------------------------------------------------
// (b) Cast without gift leaves CostMeta unset
// ---------------------------------------------------------------------------

func TestCastWithoutGift_CostMetaUnset(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("Spell", 0, "Treasure")
	// Caller skips CastWithGift and uses the normal cast path — the
	// stack item it pushes carries no gift metadata.
	item := &StackItem{
		Card:       card,
		Controller: 0,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
	}
	PushStackItem(gs, item)

	if item.CostMeta != nil {
		if promised, ok := item.CostMeta["gift_promised"].(bool); ok && promised {
			t.Fatal("cast without gift must NOT set CostMeta[gift_promised]=true")
		}
	}
	// ResolveGift on such an item must be a no-op.
	if ResolveGift(gs, item) {
		t.Fatal("ResolveGift on a non-gift cast should return false")
	}
}

// ---------------------------------------------------------------------------
// (c) Gift token actually created on recipient's battlefield on resolve
// ---------------------------------------------------------------------------

func TestResolveGift_CreatesTokenOnRecipientBattlefield(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("Food-Gift Spell", 0, "Food")

	if err := CastWithGift(gs, 0, card, 1); err != nil {
		t.Fatalf("CastWithGift failed: %v", err)
	}
	item := gs.Stack[0]

	beforeCount := len(gs.Seats[1].Battlefield)
	delivered := ResolveGift(gs, item)
	if !delivered {
		t.Fatal("ResolveGift should return true after creating the token")
	}
	if len(gs.Seats[1].Battlefield) != beforeCount+1 {
		t.Fatalf("recipient's battlefield grew by %d, want 1",
			len(gs.Seats[1].Battlefield)-beforeCount)
	}
	// The token should be a Food artifact on seat 1.
	newPerm := gs.Seats[1].Battlefield[len(gs.Seats[1].Battlefield)-1]
	if newPerm.Controller != 1 {
		t.Fatalf("new token Controller = %d, want 1 (the recipient)", newPerm.Controller)
	}
	if !newPerm.IsArtifact() {
		t.Fatal("food gift should be an artifact token")
	}
}

func TestResolveGift_DispatchesAllCanonicalTokenTypes(t *testing.T) {
	canonical := []string{"treasure", "clue", "food", "blood", "map", "gold", "powerstone", "junk"}
	for _, ty := range canonical {
		gs := newGiftGame(t)
		card := giftSpellCard("X", 0, ty)
		if err := CastWithGift(gs, 0, card, 1); err != nil {
			t.Fatalf("CastWithGift(%s) failed: %v", ty, err)
		}
		item := gs.Stack[0]
		before := len(gs.Seats[1].Battlefield)
		if !ResolveGift(gs, item) {
			t.Fatalf("ResolveGift(%s) should deliver a token", ty)
		}
		if len(gs.Seats[1].Battlefield) != before+1 {
			t.Fatalf("%s gift did not land a token on recipient", ty)
		}
	}
}

func TestResolveGift_UnknownTokenTypeReturnsFalse(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("Custom-Gift", 0, "ZombieKnight")
	if err := CastWithGift(gs, 0, card, 1); err != nil {
		t.Fatalf("CastWithGift failed: %v", err)
	}
	item := gs.Stack[0]
	before := len(gs.Seats[1].Battlefield)
	if ResolveGift(gs, item) {
		t.Fatal("ResolveGift should return false for non-canonical token types (per-card handler covers those)")
	}
	if len(gs.Seats[1].Battlefield) != before {
		t.Fatal("non-canonical gift must not create a token via the engine surface")
	}
}

func TestResolveGift_RecipientEliminatedNoOp(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("Spell", 0, "Treasure")
	if err := CastWithGift(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithGift failed: %v", err)
	}
	item := gs.Stack[0]
	// Recipient gets eliminated between cast and resolve.
	gs.Seats[2].Lost = true
	before := len(gs.Seats[2].Battlefield)
	if ResolveGift(gs, item) {
		t.Fatal("eliminated recipient should not receive the gift")
	}
	if len(gs.Seats[2].Battlefield) != before {
		t.Fatal("no token should land on an eliminated recipient")
	}
}

func TestResolveGift_FiresDeliveredTrigger(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("Spell", 0, "Treasure")
	if err := CastWithGift(gs, 0, card, 1); err != nil {
		t.Fatalf("CastWithGift failed: %v", err)
	}
	item := gs.Stack[0]

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	saw := false
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "gift_delivered" {
			saw = true
		}
	}
	ResolveGift(gs, item)
	if !saw {
		t.Fatal("ResolveGift should fire gift_delivered trigger")
	}
}

// ---------------------------------------------------------------------------
// (d) Gift cannot be self (recipient must be opponent)
// ---------------------------------------------------------------------------

func TestCastWithGift_SelfRecipientRejected(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("Spell", 0, "Treasure")
	err := CastWithGift(gs, 0, card, 0) // self
	if err == nil {
		t.Fatal("§702.174b: gift recipient must be an opponent (not self)")
	}
	if len(gs.Stack) != 0 {
		t.Fatal("rejected gift cast must not push a stack item")
	}
}

func TestCastWithGift_EliminatedRecipientRejected(t *testing.T) {
	gs := newGiftGame(t)
	gs.Seats[1].Lost = true
	card := giftSpellCard("Spell", 0, "Treasure")
	if err := CastWithGift(gs, 0, card, 1); err == nil {
		t.Fatal("§800.4: can't gift a Lost seat")
	}
}

func TestCastWithGift_InvalidRecipientRejected(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("Spell", 0, "Treasure")
	if err := CastWithGift(gs, 0, card, -1); err == nil {
		t.Fatal("negative recipient seat must reject")
	}
	if err := CastWithGift(gs, 0, card, 99); err == nil {
		t.Fatal("out-of-range recipient must reject")
	}
}

func TestCastWithGift_2PTableValidOpponent(t *testing.T) {
	gs := newGiftGame2P(t)
	card := giftSpellCard("Spell", 0, "Treasure")
	if err := CastWithGift(gs, 0, card, 1); err != nil {
		t.Fatalf("2P table: gift to seat 1 (opponent) should succeed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Negative paths
// ---------------------------------------------------------------------------

func TestCastWithGift_NoKeywordRejected(t *testing.T) {
	gs := newGiftGame(t)
	card := nonGiftSpell("Lightning Bolt", 0)
	if err := CastWithGift(gs, 0, card, 1); err == nil {
		t.Fatal("cards without gift must not run the gift cast path")
	}
}

func TestCastWithGift_NoGiftTypeRejected(t *testing.T) {
	gs := newGiftGame(t)
	// Gift keyword but no arg.
	card := &Card{
		Name:  "Gift-No-Type",
		Owner: 0,
		Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name: "X",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "gift"}, // no args
			},
		},
	}
	if err := CastWithGift(gs, 0, card, 1); err == nil {
		t.Fatal("gift keyword without a token-type arg must reject")
	}
}

// ---------------------------------------------------------------------------
// Nil / invalid-input safety
// ---------------------------------------------------------------------------

func TestCastWithGift_NilSafe(t *testing.T) {
	gs := newGiftGame(t)
	card := giftSpellCard("X", 0, "Treasure")
	if err := CastWithGift(nil, 0, card, 1); err == nil {
		t.Fatal("nil game must error")
	}
	if err := CastWithGift(gs, -1, card, 1); err == nil {
		t.Fatal("invalid caster must error")
	}
	if err := CastWithGift(gs, 99, card, 1); err == nil {
		t.Fatal("out-of-range caster must error")
	}
	if err := CastWithGift(gs, 0, nil, 1); err == nil {
		t.Fatal("nil card must error")
	}
}

func TestResolveGift_NilSafe(t *testing.T) {
	gs := newGiftGame(t)
	if ResolveGift(nil, &StackItem{}) {
		t.Fatal("nil game ResolveGift must be false")
	}
	if ResolveGift(gs, nil) {
		t.Fatal("nil item ResolveGift must be false")
	}
	// Item with no CostMeta (regular spell, no gift promised).
	if ResolveGift(gs, &StackItem{Controller: 0}) {
		t.Fatal("item without CostMeta ResolveGift must be false")
	}
}
