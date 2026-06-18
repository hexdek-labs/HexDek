package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// squad_r63_test.go — r63 mechanic-probe (CR §702.157, squad).
//
// Squad was an UNWIRED stub (only a comment block in keywords_batch3.go): the
// repeatable additional cost was never paid at cast and the ETB token copies
// were never created. Fix: wire squad into CastSpell's optional-cost stage as
// a repeatable mana cost (count stamped onto CostMeta), mirror the count onto
// the entering permanent, and at ETB create N faithful token copies through
// the canonical CreateDoubledTokens + MintTokenAsCopyOf chokepoint (token
// doublers double them, each copy a fresh *Card, copy ETBs fire).
//
// Pins: squad paid N times → exactly N copies; faithful copy (printed P/T,
// fresh *Card no aliasing); squad 0 times → zero copies; copy ETBs fire;
// token-doubler doubles the squad copies.

// squadCreatureCard builds a vanilla creature spell with printed Squad {cost}.
func squadCreatureCard(name string, baseCost, pow, tough int, squadCostArg interface{}) *Card {
	return &Card{
		Name:          name,
		Owner:         0,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         []string{"creature", "cost:" + itoa(baseCost)},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "squad", Args: []interface{}{squadCostArg}},
			},
		},
	}
}

// squadTokensOf returns the squad token copies the seat controls (minted with
// the squad_token flag), excluding the original cast creature.
func squadTokensOf(gs *GameState, seat int) []*Permanent {
	var out []*Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Flags == nil {
			continue
		}
		if p.Flags["squad_token"] == 1 {
			out = append(out, p)
		}
	}
	return out
}

// payNHat pays squad exactly n times (and every other optional cost to max).
type payNHat struct {
	GreedyHatStub
	n int
}

func (h *payNHat) ChooseOptionalCost(gs *GameState, seatIdx int, card *Card, kind string, cost, max int) int {
	if kind == "squad" {
		if h.n > max {
			return max
		}
		return h.n
	}
	return max
}

// (4)+(faithful) Squad paid N times makes exactly N copies; each copy is a
// faithful copy (printed P/T) and a distinct *Card (no aliasing).
func TestSquad_PaidNMakesNFaithfulCopies(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payNHat{n: 2}
	gs.Seats[0].ManaPool = 8 // 2 base + 2 × squad{2}
	card := squadCreatureCard("Squad Sergeant", 2, 3, 4, float64(2))
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if len(gs.Stack) != 0 {
		t.Fatalf("stack should drain after cast, got %d", len(gs.Stack))
	}

	tokens := squadTokensOf(gs, 0)
	if len(tokens) != 2 {
		t.Fatalf("squad paid 2× should make exactly 2 token copies, got %d", len(tokens))
	}

	// Original + 2 copies = 3 creatures named "Squad Sergeant".
	named := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Squad Sergeant" {
			named++
		}
	}
	if named != 3 {
		t.Errorf("expected original + 2 squad copies = 3 'Squad Sergeant', got %d", named)
	}

	for _, tok := range tokens {
		// Faithful copy: printed P/T preserved.
		if tok.Card.BasePower != 3 || tok.Card.BaseToughness != 4 {
			t.Errorf("squad copy must copy printed P/T 3/4, got %d/%d", tok.Card.BasePower, tok.Card.BaseToughness)
		}
		// Fresh *Card, no aliasing of the source.
		if tok.Card == card {
			t.Error("CardIdentity: squad copy aliases the source *Card — must be a distinct DeepCopy")
		}
		// It's a token.
		if !tok.IsToken() {
			t.Error("squad copy must be a token")
		}
	}

	// All squad mana paid (8 - 2 base - 2×2 squad = 2 left).
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected 2 mana left (8 - 2 base - 4 squad), got %d", gs.Seats[0].ManaPool)
	}
}

// (4) Squad paid 0 times → zero copies (baseline unchanged).
func TestSquad_PaidZeroMakesNoCopies(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payNHat{n: 0}
	gs.Seats[0].ManaPool = 8 // could afford, but pay 0
	card := squadCreatureCard("Squad Sergeant", 2, 3, 4, float64(2))
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if got := len(squadTokensOf(gs, 0)); got != 0 {
		t.Errorf("squad paid 0× must make 0 copies, got %d", got)
	}
	// Only the base cost was paid.
	if gs.Seats[0].ManaPool != 6 {
		t.Errorf("expected only 2 base mana paid (pool 6), got %d", gs.Seats[0].ManaPool)
	}
}

// (5) The copy tokens' own ETB triggers fire (they're entering the battlefield).
func TestSquad_CopyETBTriggersFire(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payNHat{n: 2}
	gs.Seats[0].ManaPool = 8

	var etbCount int
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	TriggerHook = func(hgs *GameState, event string, ctx map[string]interface{}) {
		if event != "permanent_etb" {
			return
		}
		// Count only the squad-token entries (flag set on the token perm).
		if p, ok := ctx["perm"].(*Permanent); ok && p != nil && p.Flags != nil && p.Flags["squad_token"] == 1 {
			etbCount++
		}
	}

	card := squadCreatureCard("Squad Sergeant", 2, 3, 4, float64(2))
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if etbCount != 2 {
		t.Errorf("each of the 2 squad copies must fire its ETB (permanent_etb), got %d", etbCount)
	}
}

// (doubler) A token-doubler (Doubling Season) doubles the squad copies — squad
// copies are created, so doublers apply (a real ruling). Also exercises the
// mana-pip squad cost arg ("{1}{R}" → CMC 2) to pin SquadCost pip parsing.
func TestSquad_TokenDoublerDoublesCopies(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payNHat{n: 1}
	gs.Seats[0].ManaPool = 8

	ds := addKWCombatBattlefield(gs, 0, "Doubling Season", 0, 0, "enchantment")
	RegisterDoublingSeason(gs, ds)

	var tokenCreated int
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	TriggerHook = func(hgs *GameState, event string, ctx map[string]interface{}) {
		if event != "token_created" {
			return
		}
		if c, ok := ctx["count"].(int); ok {
			tokenCreated += c
		}
	}

	// Squad {1}{R} as a mana-pip string (the parser's usual shape).
	card := squadCreatureCard("Doubled Squad", 2, 2, 2, "{1}{R}")
	if sc, ok := SquadCost(card); !ok || sc != 2 {
		t.Fatalf("SquadCost should parse {1}{R} → 2, got %d ok=%v", sc, ok)
	}
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// Squad paid 1× → 1 copy, doubled by Doubling Season → 2 copies.
	tokens := squadTokensOf(gs, 0)
	if len(tokens) != 2 {
		t.Errorf("Doubling Season should double 1 squad copy to 2, got %d", len(tokens))
	}
	if tokenCreated != 2 {
		t.Errorf("token_created should report 2 doubled squad copies, got %d", tokenCreated)
	}
}
