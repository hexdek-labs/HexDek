package mana

import "testing"

func TestPoolEmptyCannotPay(t *testing.T) {
	var p Pool
	cost, _ := Parse("{U}")
	if p.CanPay(cost, 0) {
		t.Error("empty pool should not pay {U}")
	}
}

func TestPoolPaysExactColor(t *testing.T) {
	p := Pool{U: 1}
	cost, _ := Parse("{U}")
	if !p.CanPay(cost, 0) {
		t.Error("pool U:1 should pay {U}")
	}
	if err := p.Pay(cost, 0); err != nil {
		t.Errorf("pay error: %v", err)
	}
	if p.U != 0 {
		t.Errorf("U after pay: got %d, want 0", p.U)
	}
}

func TestPoolPaysGenericFromAnyColor(t *testing.T) {
	cost, _ := Parse("{2}{U}{B}") // 4 CMC: 2 generic + UB

	// 3 mana total can't pay 4 CMC
	insufficient := Pool{U: 2, B: 1}
	if insufficient.CanPay(cost, 0) {
		t.Error("pool U:2 B:1 (3 mana) should NOT pay {2}{U}{B} (needs 4)")
	}

	// 4 mana with both required colors: should pay (2 generic from R, then UB)
	enough := Pool{U: 1, B: 1, R: 2}
	if !enough.CanPay(cost, 0) {
		t.Error("pool U:1 B:1 R:2 should pay {2}{U}{B} (4 mana available, all colors covered)")
	}
}

func TestPoolDoomsdayThreshold(t *testing.T) {
	cost, _ := Parse("{B}{B}{B}")
	if (Pool{B: 2}).CanPay(cost, 0) {
		t.Error("pool B:2 should NOT pay {B}{B}{B}")
	}
	if !(Pool{B: 3}).CanPay(cost, 0) {
		t.Error("pool B:3 should pay {B}{B}{B}")
	}
	if !(Pool{B: 5}).CanPay(cost, 0) {
		t.Error("pool B:5 should pay {B}{B}{B} (extra mana ignored)")
	}
}

func TestPoolColorMissing(t *testing.T) {
	p := Pool{U: 5, R: 5} // 10 mana but no black
	cost, _ := Parse("{B}{B}")
	if p.CanPay(cost, 0) {
		t.Error("pool with no black should not pay {B}{B}")
	}
}

func TestPoolXCost(t *testing.T) {
	p := Pool{U: 2, B: 5} // 7 mana
	cost, _ := Parse("{X}{B}{B}")
	// X=4 → costs 4 + 2 (BB) = 6 mana; pool has 7 → can pay
	if !p.CanPay(cost, 4) {
		t.Error("pool U:2 B:5 should pay {4}{B}{B}")
	}
	// X=10 → costs 10 + 2 = 12 mana; only 7 → can't pay
	if p.CanPay(cost, 10) {
		t.Error("pool U:2 B:5 should NOT pay {10}{B}{B}")
	}
}

func TestPoolEmptyFunc(t *testing.T) {
	p := Pool{U: 3, B: 2}
	p.Empty()
	if p.Total() != 0 {
		t.Errorf("pool after Empty() total: got %d, want 0", p.Total())
	}
}

func TestPoolAdd(t *testing.T) {
	p1 := Pool{U: 2, B: 1}
	p2 := Pool{U: 1, R: 3}
	p1.Add(p2)
	if p1.U != 3 {
		t.Errorf("U after add: got %d, want 3", p1.U)
	}
	if p1.B != 1 {
		t.Errorf("B after add: got %d, want 1", p1.B)
	}
	if p1.R != 3 {
		t.Errorf("R after add: got %d, want 3", p1.R)
	}
}

func TestPayDeductsCorrectly(t *testing.T) {
	p := Pool{U: 2, B: 3}
	cost, _ := Parse("{1}{B}{B}") // 3 CMC: 1 generic + BB
	if err := p.Pay(cost, 0); err != nil {
		t.Fatalf("pay error: %v", err)
	}
	// After: BB used (B 3→1), then 1 generic from any (W priority but 0, then U priority → U 2→1)
	if p.B != 1 {
		t.Errorf("B after pay: got %d, want 1", p.B)
	}
	if p.U != 1 {
		t.Errorf("U after pay (took 1 generic from U): got %d, want 1", p.U)
	}
}

func TestPayInsufficientReturnsError(t *testing.T) {
	p := Pool{U: 1}
	cost, _ := Parse("{U}{U}")
	if err := p.Pay(cost, 0); err == nil {
		t.Error("expected error paying {U}{U} from U:1 pool")
	}
}
