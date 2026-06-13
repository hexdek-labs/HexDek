package gameengine

import "testing"

// mod_tail_deal_damage_count_test.go — regressions for the generic
// count-scaled-damage parsed_tail handler (worker hex-dev-5).

func spellSrc(seat int) *Permanent {
	return &Permanent{Card: &Card{Name: "Burn Spell", Owner: seat}, Controller: seat}
}

func TestTailDamage_ToPlayerEqualCreatures(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[1].Life = 40
	addTestPerm(gs, 0, "Bear", "creature")
	addTestPerm(gs, 0, "Elf", "creature")
	addTestPerm(gs, 0, "Wolf", "creature")

	ok := resolveResidualByText(gs, spellSrc(0),
		"~ deals damage to any target equal to the number of creatures you control")
	if !ok {
		t.Fatalf("handler should recognize the clause")
	}
	if gs.Seats[1].Life != 37 {
		t.Errorf("opponent should take 3 (creatures you control), life=%d", gs.Seats[1].Life)
	}
}

func TestTailDamage_ToCreatureEqualCount(t *testing.T) {
	gs := newTestGame(t, 2)
	addTestPerm(gs, 0, "Bear", "creature")
	addTestPerm(gs, 0, "Elf", "creature")
	victim := addTestPerm(gs, 1, "Big Dummy", "creature")
	victim.Card.BasePower = 5
	victim.Card.BaseToughness = 5

	ok := resolveResidualByText(gs, spellSrc(0),
		"~ deals damage to target creature equal to the number of creatures you control")
	if !ok {
		t.Fatalf("handler should recognize the creature-target clause")
	}
	if victim.MarkedDamage != 2 {
		t.Errorf("victim should have 2 marked damage (creatures you control), got %d", victim.MarkedDamage)
	}
}

func TestTailDamage_EqualToThenToOrder(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[1].Life = 20
	addTestPerm(gs, 0, "Bear", "creature")

	// "deals damage equal to <count> to <target>" word order.
	ok := resolveResidualByText(gs, spellSrc(0),
		"~ deals damage equal to the number of creatures you control to any target")
	if !ok || gs.Seats[1].Life != 19 {
		t.Errorf("expected 1 damage to opponent (life 19), ok=%v life=%d", ok, gs.Seats[1].Life)
	}
}

func TestTailDamage_UnrecognizedCountSkipped(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[1].Life = 40
	ok := resolveResidualByText(gs, spellSrc(0),
		"~ deals damage to any target equal to the greatest mana value among permanents")
	if ok {
		t.Errorf("unrecognized count phrase must NOT be claimed (leave inert)")
	}
	if gs.Seats[1].Life != 40 {
		t.Errorf("no damage should be dealt for an unrecognized count")
	}
}

func TestTailDamage_NotADamageClauseIgnored(t *testing.T) {
	gs := newTestGame(t, 2)
	if resolveDealDamageEqualCount(gs, spellSrc(0), "draw a card") {
		t.Errorf("non-damage clause must not be claimed by the damage handler")
	}
}
