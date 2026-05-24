package gameengine

// CR §613.4 layer-7 sublayer-ordering audit (R60).
//
// Bug: GetEffectiveCharacteristics applied Permanent.Counters and
// Permanent.Modifications as a POST-PASS after every layer 7 sublayer
// (including 7d switch). Per CR §613.4c, counters AND P/T modifications
// belong in sublayer 7c — alongside anthems — and must therefore apply
// BEFORE 7d switch. For symmetric pumps (+1/+1 counters, +3/+3 Brute
// Force) the ordering is mathematically irrelevant (adding equal bonuses
// to both P and T commutes with switch). For asymmetric pumps it
// matters: a 3/2 with Berserk-style +3/+0 then Hands-of-Binding switch
// should read 2/6 (CR), but the engine produced 5/3 (apply switch to
// 3/2, then post-pass +3/+0).
//
// Worked example (3/2 Tarmogoyf-stand-in):
//
//   Berserk-style modification : Modification{Power: 3, Toughness: 0}
//   Switch (Hands of Binding)  : RegisterPTSwitch sublayer "d"
//
//   CR §613.4 expected order:
//     base   3/2
//     7c +3/+0  →  6/2
//     7d switch  →  2/6      ✓
//
//   Pre-fix engine order (counters+mods post-pass):
//     base   3/2
//     7d switch  →  2/3
//     post +3/+0 →  5/3      ✗
//
// Fix: move applyCountersAndMods from "after every sublayer" to "after
// sublayer c, before sublayer d" in GetEffectiveCharacteristics. The
// post-pass effectively becomes the canonical 7c counter/modification
// step.

import (
	"testing"
)

func TestLayer7c_AsymmetricModBeforeSwitch_R60(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	card := &Card{
		Name:          "Test Goyf",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     3,
		BaseToughness: 2,
	}
	perm := &Permanent{
		Card:       card,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	// Berserk-style +P/+0 pump (uneven Modification).
	perm.Modifications = append(perm.Modifications, Modification{
		Power:     3,
		Toughness: 0,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()

	// Sanity: post-pump pre-switch should read 6/2.
	if p := gs.PowerOf(perm); p != 6 {
		t.Fatalf("pre-switch power: want 6, got %d", p)
	}
	if t0 := gs.ToughnessOf(perm); t0 != 2 {
		t.Fatalf("pre-switch toughness: want 2, got %d", t0)
	}

	// Hands of Binding-style switch (sublayer "d").
	RegisterPTSwitch(gs, perm, DurationEndOfTurn, func(_ *GameState, p *Permanent) bool {
		return p == perm
	})
	gs.InvalidateCharacteristicsCache()

	got := struct{ P, T int }{gs.PowerOf(perm), gs.ToughnessOf(perm)}
	want := struct{ P, T int }{2, 6}
	if got != want {
		t.Fatalf("CR §613.4c/d ordering: want %+v (Berserk +3/+0 then switch on 3/2 → 6/2 → 2/6), got %+v", want, got)
	}
}

// Symmetric +1/+1 counters must commute with switch — pin existing
// behavior so the fix doesn't regress the easy case.
func TestLayer7c_PlusOneCounterCommutesWithSwitch_R60(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	card := &Card{
		Name:          "Test 3/2",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     3,
		BaseToughness: 2,
	}
	perm := &Permanent{
		Card:       card,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{"+1/+1": 2},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	RegisterPTSwitch(gs, perm, DurationEndOfTurn, func(_ *GameState, p *Permanent) bool {
		return p == perm
	})
	gs.InvalidateCharacteristicsCache()

	// 3/2 + two +1/+1 counters (7c) = 5/4; switch (7d) = 4/5. Either ordering
	// produces the same number because the bonus is symmetric.
	if p, tg := gs.PowerOf(perm), gs.ToughnessOf(perm); p != 4 || tg != 5 {
		t.Fatalf("3/2 + 2 +1/+1 counters then switch: want 4/5, got %d/%d", p, tg)
	}
}

// Humility (sets base to 1/1 at 7b) + +1/+1 counter must still read 2/2
// post-fix. Pins the Humility-counter interaction the existing post-pass
// comment specifically calls out.
func TestLayer7b_HumilityPlusOneOneCounter_R60(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Humility source — a 1/1 enchantment stand-in registered with the
	// canonical RegisterHumility helper.
	humCard := &Card{Name: "Humility", Owner: 0, Types: []string{"enchantment"}}
	humility := &Permanent{
		Card: humCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, humility)
	RegisterHumility(gs, humility)

	// Target creature: 3/3 with a +1/+1 counter.
	targetCard := &Card{
		Name: "Beefy Beast", Owner: 1,
		Types:         []string{"creature"},
		BasePower:     3,
		BaseToughness: 3,
	}
	target := &Permanent{
		Card: targetCard, Controller: 1, Owner: 1,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{"+1/+1": 1},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, target)
	gs.InvalidateCharacteristicsCache()

	if p, tg := gs.PowerOf(target), gs.ToughnessOf(target); p != 2 || tg != 2 {
		t.Fatalf("Humility 1/1 + one +1/+1 counter: want 2/2, got %d/%d", p, tg)
	}
}
