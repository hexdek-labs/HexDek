package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Consolidated sweep regressions (CR §122 / §616): every keyword counter-placer
// must route through the doubler chain AND fire counter_placed; the token arm
// of fabricate must route through the token-doubler chain. Each test installs
// Doubling Season and asserts the count is doubled (and, for one, that the
// counter_placed trigger fires).

func servoCount(gs *GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Servo Token" {
			n++
		}
	}
	return n
}

func TestSweep_Bolster_DoublerAndTrigger(t *testing.T) {
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	cpFired := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "counter_placed" {
			cpFired++
		}
	}
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	c := addBattlefield(gs, 0, "Weakling", 1, 1, "creature")
	ApplyBolster(gs, 0, 1)
	if c.Counters["+1/+1"] != 2 {
		t.Fatalf("bolster 1 + Doubling Season → 2 counters, got %d", c.Counters["+1/+1"])
	}
	if cpFired == 0 {
		t.Fatal("bolster should fire counter_placed")
	}
}

func TestSweep_Support_Doubler(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	a := addBattlefield(gs, 0, "A", 1, 1, "creature")
	b := addBattlefield(gs, 0, "B", 1, 1, "creature")
	ApplySupport(gs, 0, 2)
	if a.Counters["+1/+1"] != 2 || b.Counters["+1/+1"] != 2 {
		t.Fatalf("support + Doubling Season → 2 counters each, got %d / %d", a.Counters["+1/+1"], b.Counters["+1/+1"])
	}
}

func TestSweep_Reinforce_Doubler(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	card := &Card{Name: "Reinforcer", Types: []string{"instant"}}
	gs.Seats[0].Hand = []*Card{card}
	target := addBattlefield(gs, 0, "Target", 1, 1, "creature")
	if !ActivateReinforce(gs, 0, card, target, 2, 0) {
		t.Fatal("reinforce should succeed")
	}
	if target.Counters["+1/+1"] != 4 {
		t.Fatalf("reinforce 2 + Doubling Season → 4 counters, got %d", target.Counters["+1/+1"])
	}
}

func TestSweep_ModularAndGraftETB_Doubler(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	m := addBattlefield(gs, 0, "Arcbound", 0, 0, "artifact", "creature")
	ApplyModularETB(gs, m, 3)
	if m.Counters["+1/+1"] != 6 {
		t.Fatalf("modular ETB 3 + Doubling Season → 6, got %d", m.Counters["+1/+1"])
	}
	g := addBattlefield(gs, 0, "Grafter", 0, 0, "creature")
	ApplyGraftETB(gs, g, 2)
	if g.Counters["+1/+1"] != 4 {
		t.Fatalf("graft ETB 2 + Doubling Season → 4, got %d", g.Counters["+1/+1"])
	}
}

func TestSweep_FabricateCountersAndTokens_Doubler(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)

	cm := addBattlefield(gs, 0, "Fabricator-C", 2, 2, "artifact", "creature")
	ApplyFabricate(gs, cm, 1, true) // counters arm
	if cm.Counters["+1/+1"] != 2 {
		t.Fatalf("fabricate 1 (counters) + Doubling Season → 2, got %d", cm.Counters["+1/+1"])
	}

	tm := addBattlefield(gs, 0, "Fabricator-T", 2, 2, "artifact", "creature")
	ApplyFabricate(gs, tm, 1, false) // tokens arm
	if servoCount(gs, 0) != 2 {
		t.Fatalf("fabricate 1 (tokens) + Doubling Season → 2 servos, got %d", servoCount(gs, 0))
	}
}

// enters-with-N counters (the effect-side etb_with_counters twin) is doubled.
func TestSweep_EntersWithCounters_Doubler(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	hydra := addBattlefield(gs, 0, "Hydra", 0, 0, "creature")
	ResolveEffect(gs, hydra, &gameast.ModificationEffect{
		ModKind: "etb_with_counters",
		Args:    []interface{}{float64(4)},
	})
	if hydra.Counters["+1/+1"] != 8 {
		t.Fatalf("enters with 4 +1/+1 + Doubling Season → 8, got %d", hydra.Counters["+1/+1"])
	}
}
