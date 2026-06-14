package gameengine

// r63 — layer 7 P/T sublayer ordering audit (CR §613.4c): 7a CDA → 7b set base
// → 7c counters → 7d modifications (anthems/pumps) → 7e switch.
//
// Bug found: "switch power and toughness" (Inside Out / Twisted Image /
// Fluxcharger) was implemented by mutating Card.BasePower/BaseToughness
// directly — a permanent base swap, ignoring the until-EOT duration AND landing
// in the wrong sublayer so a later pump was not re-switched. Fixed to a 7e
// continuous switch (RegisterPTSwitch, now correctly sublayer "e").

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func l7Game() *GameState {
	gs := NewGameState(2, rand.New(rand.NewSource(3)), nil)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	return gs
}

func l7Creature(gs *GameState, seat int, name string, pow, tough int) *Permanent {
	p := &Permanent{
		Card: &Card{Name: name, Owner: seat, Types: []string{"creature"},
			TypeLine: "Creature", BasePower: pow, BaseToughness: tough},
		Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func ptOf(gs *GameState, p *Permanent) (int, int) {
	ch := GetEffectiveCharacteristics(gs, p)
	return ch.Power, ch.Toughness
}

// (a) 7b set 1/1 + 7c +1/+1 counter = 2/2 (the set must not swallow the counter).
func TestL7_SetThenCounter(t *testing.T) {
	gs := l7Game()
	c := l7Creature(gs, 0, "Goyf", 4, 5)
	RegisterSetPT(gs, c, 1, 1, DurationPermanent, "Humility", "test-set")
	c.Counters["+1/+1"] = 1
	gs.InvalidateCharacteristicsCache()
	if p, tg := ptOf(gs, c); p != 2 || tg != 2 {
		t.Fatalf("set-1/1 + counter should be 2/2; got %d/%d", p, tg)
	}
}

// (b) 7a CDA base + 7c counter + 7d anthem compose in order.
func TestL7_CDA_Counter_Anthem(t *testing.T) {
	gs := l7Game()
	c := l7Creature(gs, 0, "Tarmogoyf", 0, 1)
	RegisterCDA(gs, c, "test-cda", func(_ *GameState, p *Permanent) bool { return p == c },
		func(_ *GameState, _ *Permanent) (int, int) { return 3, 4 }) // CDA base 3/4
	c.Counters["+1/+1"] = 2          // 7c → +2/+2
	src := c
	registerAnthemPT(gs, c, 1, 1, "test-anthem", func(_ *GameState, t *Permanent) bool { return t.Controller == src.Controller })
	gs.InvalidateCharacteristicsCache()
	// 3/4 → +2/+2 (counter) → +1/+1 (anthem) = 6/7
	if p, tg := ptOf(gs, c); p != 6 || tg != 7 {
		t.Fatalf("CDA 3/4 + 2 counters + anthem should be 6/7; got %d/%d", p, tg)
	}
}

// (c) +1/+1 and -1/-1 net in the layer calc; SBA 704.5q annihilates the COUNTERS
// afterward (not before).
func TestL7_CounterNettingThenSBA(t *testing.T) {
	gs := l7Game()
	c := l7Creature(gs, 0, "Beast", 2, 2)
	c.Counters["+1/+1"] = 3
	c.Counters["-1/-1"] = 1
	gs.InvalidateCharacteristicsCache()
	// layer nets +3-1 = +2 → 4/4 (before SBA removes a pair)
	if p, tg := ptOf(gs, c); p != 4 || tg != 4 {
		t.Fatalf("layer should net counters to 4/4; got %d/%d", p, tg)
	}
	StateBasedActions(gs) // §704.5q pair-removal
	if c.Counters["+1/+1"] != 2 || c.Counters["-1/-1"] != 0 {
		t.Fatalf("SBA should annihilate a +1/-1 pair → 2/0; got +%d/-%d", c.Counters["+1/+1"], c.Counters["-1/-1"])
	}
	if p, tg := ptOf(gs, c); p != 4 || tg != 4 {
		t.Fatalf("post-SBA P/T unchanged at 4/4; got %d/%d", p, tg)
	}
}

// (e) two base-set effects: last timestamp wins; an anthem applies on top even
// when the base-set has a LATER timestamp than the anthem.
func TestL7_BaseSetLastWins_AnthemOnTop(t *testing.T) {
	gs := l7Game()
	c := l7Creature(gs, 0, "Shapeshifter", 5, 5)
	src := c
	registerAnthemPT(gs, c, 2, 2, "test-anthem-early", func(_ *GameState, t *Permanent) bool { return t.Controller == src.Controller })
	RegisterSetPT(gs, c, 1, 1, DurationPermanent, "SetA", "test-setA") // earlier
	RegisterSetPT(gs, c, 0, 3, DurationPermanent, "SetB", "test-setB") // later → wins
	gs.InvalidateCharacteristicsCache()
	// base-set last (0/3) → anthem +2/+2 = 2/5
	if p, tg := ptOf(gs, c); p != 2 || tg != 5 {
		t.Fatalf("last base-set 0/3 + anthem +2/+2 should be 2/5; got %d/%d", p, tg)
	}
}

// (f) THE BUG: switch (7e) applies LAST, after a pump — and is until-EOT, not a
// permanent base swap. A 3/2 with a +1/+0 modification then a switch reads 2/4.
func TestL7_SwitchAfterPump(t *testing.T) {
	gs := l7Game()
	c := l7Creature(gs, 0, "Fluxcharger", 3, 2)
	c.Modifications = append(c.Modifications, Modification{Power: 1, Toughness: 0}) // +1/+0 (7d)
	gs.InvalidateCharacteristicsCache()
	if p, tg := ptOf(gs, c); p != 4 || tg != 2 {
		t.Fatalf("precondition: 3/2 + 1/0 = 4/2; got %d/%d", p, tg)
	}
	// Switch P/T (the modkind path) — must be a 7e continuous switch applied
	// AFTER the +1/+0 pump: 4/2 → 2/4.
	ResolveEffect(gs, c, &gameast.ModificationEffect{ModKind: "switch_pt_self"})
	gs.InvalidateCharacteristicsCache()
	if p, tg := ptOf(gs, c); p != 2 || tg != 4 {
		t.Fatalf("switch must apply LAST (7e) after the pump: want 2/4; got %d/%d", p, tg)
	}
	// The base P/T must NOT have been mutated (switch is layer-only).
	if c.Card.BasePower != 3 || c.Card.BaseToughness != 2 {
		t.Fatalf("switch must not mutate printed base P/T; got base %d/%d", c.Card.BasePower, c.Card.BaseToughness)
	}
}

// (g) Humility-style set 1/1 composes with a counter and an external anthem.
func TestL7_HumilitySetCounterAnthem(t *testing.T) {
	gs := l7Game()
	c := l7Creature(gs, 0, "Big Dude", 7, 7)
	RegisterSetPT(gs, c, 1, 1, DurationPermanent, "Humility", "test-humility") // 7b set 1/1
	c.Counters["+1/+1"] = 1                                                    // 7c +1/+1
	src := c
	registerAnthemPT(gs, c, 1, 1, "test-glorious-anthem", func(_ *GameState, t *Permanent) bool { return t.Controller == src.Controller })
	gs.InvalidateCharacteristicsCache()
	// 1/1 (set) → anthem +1/+1 → counter +1/+1 = 3/3
	if p, tg := ptOf(gs, c); p != 3 || tg != 3 {
		t.Fatalf("Humility 1/1 + counter + anthem should be 3/3; got %d/%d", p, tg)
	}
}
