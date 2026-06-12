package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r62 — phase-2 legality finding #2 (seed 1050043, turn 33, seat 2,
// Martyr's Cause "over-paid: announced 3, spent 4"): Esper Sentinel's
// tax handler deducted the caster's mana inside their own cast window
// (the noncreature_spell_cast trigger fires mid-CastSpell, between the
// validator's BeginCast and FinishCast) via ManaPool -= x +
// SyncManaAfterSpend, but never credited it through
// gs.Legality.NoteManaSpend — so the cost-paid check (CR 601.2f-h)
// read every Sentinel-taxed cast as a +1 over-pay. Confirmed by
// instrumentation: the tax fired at exactly the violation's
// seat/turn/amount, once (NOT a double-tax — the deduction itself was
// always correct; only the validator accounting was missing).

func esperLegalityFixture(t *testing.T) (*gameengine.GameState, *gameengine.LegalityValidator, *gameengine.Card) {
	t.Helper()
	gs := gameengine.NewGameState(2, nil, nil)
	v := gameengine.NewLegalityValidator(1050043)
	gs.Legality = v

	// Seat 1 controls Esper Sentinel (1/1 — tax X = 1).
	sentinel := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:          "Esper Sentinel",
			Owner:         1,
			Types:         []string{"artifact", "creature"},
			BasePower:     1,
			BaseToughness: 4,
		},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, sentinel)

	// Seat 0 casts a noncreature spell for 3 with 5 in pool — the
	// Martyr's Cause shape (enchantment, CMC 3, pool 5).
	spell := &gameengine.Card{
		Name:  "Martyr's Cause",
		Owner: 0,
		Types: []string{"enchantment", "cost:3"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)
	gs.Seats[0].ManaPool = 5
	gameengine.EnsureTypedPool(gs.Seats[0])
	return gs, v, spell
}

func TestEsperSentinel_TaxedCast_NoCostPaidViolation(t *testing.T) {
	gs, v, spell := esperLegalityFixture(t)
	gs.Active = 0
	gs.Phase = "main"

	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// The tax must have actually been paid: 5 - 3 (spell) - 1 (Sentinel
	// tax, greedily paid) = 1.
	if got := gameengine.EnsureTypedPool(gs.Seats[0]).Total(); got != 1 {
		t.Fatalf("expected pool 1 after spell (3) + Sentinel tax (1), got %d — the tax handler regressed", got)
	}

	// And the validator must see a clean cast: the tax is an auxiliary
	// in-window payment, credited via NoteManaSpend, NOT part of the
	// spell's announced total. Pre-fix this produced exactly one
	// 601.2f-h "over-paid (double-deduction)" violation.
	for _, viol := range v.Violations {
		t.Errorf("taxed honest cast flagged: %s", viol.String())
	}
}

// Control: with no Sentinel on the battlefield the same cast spends
// exactly the announced 3 and stays clean — pins that the fixture
// itself isn't masking anything.
func TestEsperSentinel_ControlWithoutSentinel_Clean(t *testing.T) {
	gs, v, spell := esperLegalityFixture(t)
	gs.Active = 0
	gs.Phase = "main"
	gs.Seats[1].Battlefield = nil // remove the Sentinel

	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if got := gameengine.EnsureTypedPool(gs.Seats[0]).Total(); got != 2 {
		t.Fatalf("expected pool 2 (5-3, no tax), got %d", got)
	}
	if len(v.Violations) != 0 {
		t.Fatalf("untaxed cast flagged: %v", v.Violations)
	}
}
