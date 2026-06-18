package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// gift_cast_r63_test.go — r63 mechanic-probe (CR §702.192, Gift / WOE-OTJ).
//
// The resolution layer (ResolveGift + the 6 per_card gift_consumers) was
// fully built and tested, but NOTHING in the live cast path stamped
// CostMeta["gift_promised"], so every printed gift card resolved
// permanently in no-gift mode: the opponent never got the gift and the
// gift-gated bonus never applied. Compounding it, printed gift cards parse
// the "Gift a <x>" preamble to a Static scaffold node (NOT a Keyword), so
// the keyword-based HasGift/GiftType missed every real card. This pins the
// cast-side wiring: CastSpell now offers the optional promise (Hat-gated),
// picks a recipient, and stamps the promise + recipient + gift type.

// giftSpellCardAST builds an instant whose gift preamble is the REAL
// parsed shape: a Static ability whose Raw is "gift a <thing>".
func giftSpellCardAST(name, giftRaw string) *Card {
	return &Card{
		Name:  name,
		Owner: 0,
		Types: []string{"instant", "cost:1"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: giftRaw},
			},
		},
	}
}

func giftPromisedEvents(gs *GameState) []Event {
	var out []Event
	for _, ev := range gs.EventLog {
		if ev.Kind == "gift_promised" {
			out = append(out, ev)
		}
	}
	return out
}

// (1)+(2) Casting a gift card with a willing Hat stamps the promise:
// gift_promised=true, a valid opponent recipient, the canonical token type.
func TestGift_CastWithPromiseStampsCostMeta(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{} // promise the gift
	gs.Seats[0].ManaPool = 10

	card := giftSpellCardAST("Blooming Blast", "gift a treasure")
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	evs := giftPromisedEvents(gs)
	if len(evs) != 1 {
		t.Fatalf("expected exactly one gift_promised event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.Seat != 0 {
		t.Errorf("gift caster should be seat 0, got %d", ev.Seat)
	}
	if ev.Target != 1 {
		t.Errorf("gift recipient should be the opponent (seat 1), got %d", ev.Target)
	}
	if gt, _ := ev.Details["gift_type"].(string); gt != "treasure" {
		t.Errorf("gift_type should be treasure, got %q", gt)
	}
}

// (3) Declining the gift stamps nothing — the spell resolves in its
// lesser / no-bonus mode.
func TestGift_CastDeclinedStampsNothing(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &declineHat{} // decline the gift
	gs.Seats[0].ManaPool = 10

	card := giftSpellCardAST("Blooming Blast", "gift a treasure")
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	if evs := giftPromisedEvents(gs); len(evs) != 0 {
		t.Fatalf("declining the gift must not stamp a promise, got %d gift_promised events", len(evs))
	}
}

// (4) The recipient must be a LIVING opponent (CR §702.192b / §800.4):
// an eliminated seat is skipped.
func TestGift_RecipientSkipsEliminatedOpponent(t *testing.T) {
	gs := newKWCombatGame4P(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 10
	gs.Seats[1].Lost = true // nearest opponent eliminated

	card := giftSpellCardAST("Crumb and Get It", "gift a food")
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	evs := giftPromisedEvents(gs)
	if len(evs) != 1 {
		t.Fatalf("expected one gift_promised event, got %d", len(evs))
	}
	if evs[0].Target == 1 {
		t.Error("gift recipient must skip the eliminated seat 1")
	}
	if evs[0].Target != 2 {
		t.Errorf("gift recipient should be the next living opponent (seat 2), got %d", evs[0].Target)
	}
}

// DetectGift recognizes the real parsed shape and extracts the canonical
// token type; non-token gifts ("gift a card") are still gifts with type "".
func TestDetectGift_ParsedASTForm(t *testing.T) {
	cases := []struct {
		raw      string
		wantGift bool
		wantType string
	}{
		{"gift a treasure", true, "treasure"},
		{"gift a food", true, "food"},
		{"gift a clue", true, "clue"},
		{"gift a map", true, "map"},
		{"gift an octopus", true, ""}, // non-token gift, still a gift
		{"gift a card", true, ""},     // draw — non-token
		{"deal 2 damage to target creature", false, ""},
	}
	for _, c := range cases {
		card := giftSpellCardAST("X", c.raw)
		has, ty := DetectGift(card)
		if has != c.wantGift {
			t.Errorf("DetectGift(%q) has=%v, want %v", c.raw, has, c.wantGift)
		}
		if ty != c.wantType {
			t.Errorf("DetectGift(%q) type=%q, want %q", c.raw, ty, c.wantType)
		}
	}
	// Non-gift card with no gift Static node.
	if has, _ := DetectGift(&Card{Name: "Lightning Bolt"}); has {
		t.Error("DetectGift must be false for a card with no AST / no gift node")
	}
}

// chooseGiftRecipient returns -1 when the caster has no living opponent.
func TestChooseGiftRecipient_NoLivingOpponent(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[1].Lost = true
	if r := chooseGiftRecipient(gs, 0); r != -1 {
		t.Errorf("expected -1 when no living opponent, got %d", r)
	}
}
