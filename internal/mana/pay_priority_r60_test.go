package mana

import "testing"

// pay_priority_r60_test.go — R60 internal/mana precision sweep.
//
// Bug: Pool.Pay deducted colorless mana LAST when paying the generic
// portion of a cost, iterating slot order W→U→B→R→G→C. Colorless is the
// LEAST flexible resource in the pool: it can only pay generic costs or
// {C} pips. Colored mana can pay generic OR its matching color. Spending
// colored mana on generic when colorless is available strands the colored
// mana for no benefit.
//
// Concrete failure: pool {W:1, C:2} paying {1} then {W} — buggy order
// burned W on the {1}, leaving {W:0, C:2}, which then fails to pay the
// {W} cast despite the pool having 2 mana total.
//
// Fix: generic-payment slot order is now C→W→U→B→R→G. These regressions
// pin the priority + sequence-payability invariant.

func TestPayGeneric_PrefersColorlessOverColored(t *testing.T) {
	// W:1 + C:2 paying {1}: must burn C, not W.
	p := Pool{W: 1, C: 2}
	cost, _ := Parse("{1}")
	if err := p.Pay(cost, 0); err != nil {
		t.Fatalf("pay {1}: %v", err)
	}
	if p.W != 1 {
		t.Errorf("W after paying {1}: got %d, want 1 (colorless should be spent first)", p.W)
	}
	if p.C != 1 {
		t.Errorf("C after paying {1}: got %d, want 1 (one colorless burned for generic)", p.C)
	}
}

func TestPayGeneric_PreservesColoredForFutureCast(t *testing.T) {
	// Sequence: pay {1}, then pay {W}. Pool W:1 + C:2 (3 mana total).
	// Both casts MUST succeed; the buggy slot order stranded W on the
	// first cast and failed the second.
	p := Pool{W: 1, C: 2}
	one, _ := Parse("{1}")
	w, _ := Parse("{W}")
	if err := p.Pay(one, 0); err != nil {
		t.Fatalf("pay {1}: %v", err)
	}
	if err := p.Pay(w, 0); err != nil {
		t.Fatalf("pay {W} after {1}: %v — colored mana was stranded by generic-pay order", err)
	}
	// Final state: W gone (paid the {W}), C reduced by 1 (paid the {1}).
	if p.W != 0 || p.C != 1 {
		t.Errorf("final pool after {1}+{W}: got W:%d C:%d, want W:0 C:1", p.W, p.C)
	}
}

func TestPayGeneric_ColorlessFirstWithXSpell(t *testing.T) {
	// X spells route through the same generic-pay slot loop. Pool
	// U:1 + C:5 paying {X}{U} with X=4: U pip consumes U:1, then 4
	// generic must come from C (not the now-empty U), leaving C:1.
	p := Pool{U: 1, C: 5}
	cost, _ := Parse("{X}{U}")
	if err := p.Pay(cost, 4); err != nil {
		t.Fatalf("pay {X}{U} x=4: %v", err)
	}
	if p.U != 0 {
		t.Errorf("U after paying {X}{U} x=4: got %d, want 0", p.U)
	}
	if p.C != 1 {
		t.Errorf("C after paying {X}{U} x=4: got %d, want 1 (4 generic from C)", p.C)
	}
}

func TestPayGeneric_FallsThroughWhenColorlessExhausted(t *testing.T) {
	// Pool W:2 + C:1 paying {2}: spend C:1 first, then need 1 more from
	// W (next in slot order after C). Final W:1, C:0.
	p := Pool{W: 2, C: 1}
	cost, _ := Parse("{2}")
	if err := p.Pay(cost, 0); err != nil {
		t.Fatalf("pay {2}: %v", err)
	}
	if p.C != 0 {
		t.Errorf("C after paying {2}: got %d, want 0 (colorless burned first)", p.C)
	}
	if p.W != 1 {
		t.Errorf("W after paying {2}: got %d, want 1 (fallback after colorless exhausted)", p.W)
	}
}

func TestPayGeneric_ColorlessPipStillRequiresC(t *testing.T) {
	// Defense-in-depth: ensure the {C} pip (Eldrazi cost, colorless
	// requirement) still drains p.C BEFORE the generic-pay loop, so
	// the new priority order doesn't accidentally double-spend the
	// same colorless mana.
	p := Pool{W: 3, C: 1}
	cost, _ := Parse("{C}{1}") // 1 colorless pip + 1 generic
	if err := p.Pay(cost, 0); err != nil {
		t.Fatalf("pay {C}{1}: %v", err)
	}
	// {C} drains C:1→0; then {1} generic falls through to W:3→2.
	if p.C != 0 {
		t.Errorf("C after paying {C}{1}: got %d, want 0", p.C)
	}
	if p.W != 2 {
		t.Errorf("W after paying {C}{1}: got %d, want 2 (generic fell through to W)", p.W)
	}
}
