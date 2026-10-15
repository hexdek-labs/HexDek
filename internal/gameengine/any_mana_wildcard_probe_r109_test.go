package gameengine

import "testing"

// TestAnyManaWildcardProbe_r109 asks the question r108 never asked:
// is the `Any` bucket the add_mana_effect fallback produces NARROWER
// or WIDER than a correctly-typed colored mana?
//
// r108 filed "Vivi yields Any:1 instead of at least the right colour"
// and ranked emitting the colours first as "near-risk-free". That is
// only true if Any is narrower. Measure it.
func TestAnyManaWildcardProbe_r109(t *testing.T) {
	pips := []string{"W", "U", "B", "R", "G", "C"}

	anyPool := func() *ColoredManaPool { return &ColoredManaPool{Any: 1} }
	uPool := func() *ColoredManaPool { return &ColoredManaPool{U: 1} }

	for _, pip := range pips {
		a := anyPool().CanPayColored(pip, 1, "activated")
		u := uPool().CanPayColored(pip, 1, "activated")
		t.Logf("pip {%s}:  Any:1 pays=%v   U:1 pays=%v", pip, a, u)
		if !a {
			t.Errorf("Any:1 failed to pay {%s} — Any is NOT a universal wildcard", pip)
		}
	}

	// And the spend path, not just the predicate.
	for _, pip := range pips {
		p := anyPool()
		got := PayColoredPip(p, pip, "activated")
		t.Logf("PayColoredPip({%s}) against Any:1 -> %q, pool now %+v", pip, got, *p)
		if got == "" {
			t.Errorf("Any:1 could not actually pay {%s}", pip)
		}
	}

	// Positive control: an EMPTY pool must fail, or the probe proves nothing.
	empty := &ColoredManaPool{}
	if empty.CanPayColored("U", 1, "activated") {
		t.Fatal("control: empty pool claims it can pay {U} — probe is broken")
	}
	if got := PayColoredPip(empty, "U", "activated"); got != "" {
		t.Fatalf("control: empty pool paid {U} as %q — probe is broken", got)
	}
	t.Log("CONTROL empty pool: cannot pay {U} (predicate and spend both refuse)")
}
