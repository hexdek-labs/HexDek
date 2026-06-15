package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func doMonstrosity(gs *GameState, src *Permanent, n int) {
	ResolveEffect(gs, src, &gameast.ModificationEffect{ModKind: "monstrosity", Args: []interface{}{float64(n)}})
}
func doAdapt(gs *GameState, src *Permanent, n int) {
	ResolveEffect(gs, src, &gameast.ModificationEffect{ModKind: "adapt", Args: []interface{}{float64(n)}})
}

// (a) Monstrosity N puts N +1/+1 counters and sets the durable monstrous flag.
func TestMonstrosity_BecomesMonstrous(t *testing.T) {
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Polukranos", 5, 5, "creature")
	doMonstrosity(gs, c, 3)
	if c.Counters["+1/+1"] != 3 {
		t.Fatalf("monstrosity 3 should place 3 counters, got %d", c.Counters["+1/+1"])
	}
	if c.Flags["monstrous"] != 1 {
		t.Fatal("creature should be marked monstrous")
	}
}

// (b) Re-activating monstrosity on an already-monstrous creature does nothing.
func TestMonstrosity_AlreadyMonstrousNoOp(t *testing.T) {
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Polukranos", 5, 5, "creature")
	doMonstrosity(gs, c, 3)
	doMonstrosity(gs, c, 3) // second time: no effect
	if c.Counters["+1/+1"] != 3 {
		t.Fatalf("a second monstrosity must not add more counters, got %d", c.Counters["+1/+1"])
	}
}

// (c) "When this becomes monstrous" fires exactly once, on the flip.
func TestMonstrosity_BecomesMonstrousTriggerFiresOnce(t *testing.T) {
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "monstrous" {
			fired++
		}
	}
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Stormbreath Dragon", 4, 4, "creature")
	doMonstrosity(gs, c, 3)
	doMonstrosity(gs, c, 3) // already monstrous — must not re-fire
	if fired != 1 {
		t.Fatalf("becomes-monstrous trigger should fire exactly once, fired %d", fired)
	}
}

// (d) Adapt N: no +1/+1 counters → N counters; already has one → nothing.
func TestAdapt_OnlyWhenNoPlusCounters(t *testing.T) {
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Pelt Collector", 1, 1, "creature")
	doAdapt(gs, c, 2)
	if c.Counters["+1/+1"] != 2 {
		t.Fatalf("adapt 2 on a counterless creature should place 2, got %d", c.Counters["+1/+1"])
	}
	doAdapt(gs, c, 2) // already has +1/+1 → no effect
	if c.Counters["+1/+1"] != 2 {
		t.Fatalf("adapt must do nothing when the creature already has a +1/+1 counter, got %d", c.Counters["+1/+1"])
	}
}

// (g) Adapt's "no counters" check looks ONLY at +1/+1 counters.
func TestAdapt_OtherCounterKindsDoNotBlock(t *testing.T) {
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Adapter", 2, 2, "creature")
	c.AddCounter("stun", 1) // a different counter kind
	doAdapt(gs, c, 2)
	if c.Counters["+1/+1"] != 2 {
		t.Fatalf("a stun counter must not block adapt, expected 2 +1/+1, got %d", c.Counters["+1/+1"])
	}
}

// (e) Both interact with +1/+1 doublers.
func TestMonstrosityAdapt_DoublersApply(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)

	m := addBattlefield(gs, 0, "Monster", 5, 5, "creature")
	doMonstrosity(gs, m, 3)
	if m.Counters["+1/+1"] != 6 {
		t.Fatalf("monstrosity 3 with Doubling Season should place 6, got %d", m.Counters["+1/+1"])
	}

	a := addBattlefield(gs, 0, "Adapter", 2, 2, "creature")
	doAdapt(gs, a, 2)
	if a.Counters["+1/+1"] != 4 {
		t.Fatalf("adapt 2 with Doubling Season should place 4, got %d", a.Counters["+1/+1"])
	}
}
