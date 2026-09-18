package gameengine

import "testing"

func mvlPerm(power, value int) *Permanent {
	// encode value in a flag we read via the test valueFn; power via BasePower
	p := &Permanent{Card: &Card{BasePower: power, Types: []string{"creature"}}, Counters: map[string]int{}, Flags: map[string]int{"v": value}}
	return p
}

func TestChooseLeastValuableSet_CountBased(t *testing.T) {
	// "sacrifice 2": contrib=1 each, minimize value → the two cheapest.
	a := mvlPerm(0, 1)
	b := mvlPerm(0, 2)
	c := mvlPerm(0, 5)
	contrib := func(*Permanent) int { return 1 }
	value := func(p *Permanent) int { return p.Flags["v"] }
	got := ChooseLeastValuableSet([]*Permanent{c, b, a}, 2, contrib, value)
	if len(got) != 2 {
		t.Fatalf("need 2 → 2 chosen, got %d", len(got))
	}
	sel := map[*Permanent]bool{got[0]: true, got[1]: true}
	if !sel[a] || !sel[b] || sel[c] {
		t.Error("should pick the two cheapest (value 1 and 2), not the value-5 one")
	}
}

func TestChooseLeastValuableSet_PowerThreshold(t *testing.T) {
	// Teamwork 4: contrib=power. One 4-power (value 3) beats four 1-power
	// (value 1 each = 4) on total value — the efficient pick.
	big := mvlPerm(4, 3)
	s1 := mvlPerm(1, 1)
	s2 := mvlPerm(1, 1)
	s3 := mvlPerm(1, 1)
	s4 := mvlPerm(1, 1)
	contrib := func(p *Permanent) int { return p.Card.BasePower }
	value := func(p *Permanent) int { return p.Flags["v"] }
	got := ChooseLeastValuableSet([]*Permanent{s1, s2, s3, s4, big}, 4, contrib, value)
	if len(got) != 1 || got[0] != big {
		t.Errorf("Teamwork 4 should pick the single 4-power (value 3) over four 1/1s (value 4); got %d perms", len(got))
	}
}

func TestChooseLeastValuableSet_Unreachable(t *testing.T) {
	// total power 3 < need 5 → nil (fail-closed).
	got := ChooseLeastValuableSet([]*Permanent{mvlPerm(1, 1), mvlPerm(2, 1)}, 5,
		func(p *Permanent) int { return p.Card.BasePower },
		func(p *Permanent) int { return p.Flags["v"] })
	if got != nil {
		t.Errorf("unreachable requirement must return nil (fail-closed), got %d", len(got))
	}
}
