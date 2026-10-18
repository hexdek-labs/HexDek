package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r182 — the per-card half: these drive the REAL fireTrigger. One pins
// the BEHAVIOUR the instrumentation must not disturb; one pins that
// the production call sites are actually wired, which the unit tests
// in internal/gameengine cannot tell (r163's lesson — a control that
// deletes the call site must go red somewhere).

// r182recursor registers a handler on evt that re-fires evt once,
// records every nesting depth at which its body actually ran, and
// unregisters itself at test end.
func r182recursor(t *testing.T, gs *gameengine.GameState, name, evt string) *[]int {
	t.Helper()
	ran := &[]int{}
	r := Global()
	r.OnTrigger(name, evt, func(g *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		*ran = append(*ran, g.Flags["trigger_depth"])
		fireTrigger(g, evt, map[string]interface{}{"seat": 0})
	})
	t.Cleanup(func() {
		r.mu.Lock()
		delete(r.onTrigger, normalizeName(name))
		r.mu.Unlock()
	})
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
		// 2/2 and not a bare 0/0: a toughness-0 creature is destroyed
		// by SBAs during the first cascade's inline resolution, which
		// leaves the battlefield empty and makes a second cascade
		// silently dispatch nothing.
		Card: &gameengine.Card{Name: name, Types: []string{"creature"},
			BasePower: 2, BaseToughness: 2},
		Controller: 0,
		Owner:      0,
	})
	return ran
}

func r182percardReset(t *testing.T) {
	t.Helper()
	reset := func() {
		gameengine.ResetDepthCapForTest()
		gameengine.ResetTriggerDepthForTest()
		gameengine.ResetLoopGuardCountsForTest()
		gameengine.ResetLoopGuardLabelsForTest()
	}
	reset()
	t.Cleanup(reset)
}

// TestR182_CapTruncatesAtExactlyTheSamePoint is the behaviour lock.
// An unbounded linear recursion runs its handler at depths 1..8 and
// nowhere else: the dispatch at depth 9 is refused before any handler
// is looked up. The instrumentation must not let one extra handler
// run, nor stop one that runs today.
//
// MUTATION CONTROL: lowering the cap from 8 to 7 turns this red.
func TestR182_CapTruncatesAtExactlyTheSamePoint(t *testing.T) {
	r182percardReset(t)
	gs := newTestGS(2)
	gs.EventPolicy = gameengine.EventLogNone // what the grinder runs

	ran := r182recursor(t, gs, "ZZ R182 Linear", "creature_dies")
	fireTrigger(gs, "creature_dies", map[string]interface{}{"seat": 0})

	if len(*ran) != 8 {
		t.Fatalf("handler ran %d times, want exactly 8 — the cap refuses "+
			"the dispatch at depth 9, so depths 1..8 dispatch and nothing "+
			"deeper does (depths seen: %v)", len(*ran), *ran)
	}
	for i, d := range *ran {
		if d != i+1 {
			t.Fatalf("handler run %d happened at nesting depth %d, want %d (%v)",
				i, d, i+1, *ran)
		}
	}
	// Every increment is paired with its deferred decrement: r182 reads
	// the depth into a local instead of re-reading the map three times,
	// and must not disturb the unwind.
	if got := gs.Flags["trigger_depth"]; got != 0 {
		t.Fatalf("trigger_depth left at %d after the cascade unwound, want 0", got)
	}
	// r163's curve must be unchanged too: every level 1..9 reached once,
	// nothing at 10.
	h := gameengine.TriggerDepthHistogram()
	for d := 1; d <= 9; d++ {
		if h[d] != 1 {
			t.Fatalf("depth %d counted %d, want 1 (%v)", d, h[d], h)
		}
	}
	if n, ok := h[10]; ok {
		t.Fatalf("depth 10 recorded (%d); fireTrigger returns at >8 without "+
			"dispatching, so nothing can nest deeper", n)
	}
}

// TestR182_FireTriggerFeedsTheSwallowCounters proves the production
// call sites exist — both of them. It deliberately does NOT assert how
// many dispatches the engine's own inline resolution generates beyond
// what this scenario pins; it asserts the RELATIONSHIPS that make the
// new columns mean something.
//
// Two separate TOP-LEVEL cascades, each swallowing one dispatch at
// depth 9:
//   - the per-game total spans both cascades (2),
//   - the per-cascade peak does not (1),
//   - the per-GAME event attribution counts this game once (1) where
//     r162's per-DISPATCH attribution counts it twice.
//
// MUTATION CONTROLS: deleting gs.NoteDepthCapSwallow from fireTrigger,
// or making it not increment, turns the total red; deleting the
// `depth == 1 -> NoteCascadeRoot` wiring turns the per-cascade peak
// red, because it then just tracks the per-game total.
func TestR182_FireTriggerFeedsTheSwallowCounters(t *testing.T) {
	r182percardReset(t)
	gs := newTestGS(2)
	gs.EventPolicy = gameengine.EventLogNone

	r182recursor(t, gs, "ZZ R182 Wired", "creature_dies")
	fireTrigger(gs, "creature_dies", map[string]interface{}{"seat": 0})
	fireTrigger(gs, "creature_dies", map[string]interface{}{"seat": 0})

	guard := gameengine.LoopGuardCounts()["percard_dispatch_depth_cap"]
	if guard != 2 {
		t.Fatalf("the depth cap fired %d times, want 2 (once per cascade) — "+
			"this test is not exercising what it thinks", guard)
	}
	games, mixed, maxPerGame, cascadeMax, buckets := gameengine.DepthCapStats()
	if games != 1 {
		t.Fatalf("games with a swallow = %d, want 1 — the production call "+
			"site in fireTrigger is not wired", games)
	}
	if maxPerGame != guard {
		t.Fatalf("per-game peak = %d but the window guard count is %d; with "+
			"one game those must agree, or the per-game counter is missing "+
			"swallows", maxPerGame, guard)
	}
	if cascadeMax != 1 {
		t.Fatalf("per-cascade peak = %d, want 1 — each top-level cascade "+
			"swallowed exactly one dispatch, so a peak equal to the "+
			"per-game total (%d) means the cascade counter never resets "+
			"and the column is redundant", cascadeMax, maxPerGame)
	}
	if mixed != 0 {
		t.Fatalf("mixed = %d, want 0 — both swallows were creature_dies", mixed)
	}
	if buckets[1] != 1 || buckets[2] != 1 {
		t.Fatalf("the >=1 and >=2 buckets must each hold this game (%v)", buckets)
	}

	// Per-GAME attribution: one entry, count 1, naming the event the
	// game's first swallow rode in on.
	if got := gameengine.LoopGuardLabelsFor(gameengine.DepthCapFirstEventGuard); got != "creature_dies:1" {
		t.Fatalf("per-game first-event attribution = %q, want %q",
			got, "creature_dies:1")
	}
	// Per-DISPATCH attribution (r162) sees the same game twice. If the
	// two columns ever agree, one of them is redundant.
	if got := gameengine.LoopGuardLabelsFor("percard_dispatch_depth_cap"); got != "creature_dies:2" {
		t.Fatalf("per-dispatch attribution = %q, want %q — the two columns "+
			"must not be measuring the same thing", got, "creature_dies:2")
	}

	if gameengine.DepthCapSummary() == "" {
		t.Fatal("the cap fired but the grinder field renders empty")
	}
}

// TestR182_NoSwallowsRecordedForAShallowCascade — the normal path must
// stay silent, so the grinder line is byte-identical on the vast
// majority of windows.
func TestR182_NoSwallowsRecordedForAShallowCascade(t *testing.T) {
	r182percardReset(t)
	gs := newTestGS(2)
	gs.EventPolicy = gameengine.EventLogNone

	r := Global()
	const name = "ZZ R182 Shallow"
	r.OnTrigger(name, "creature_dies", func(g *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {})
	t.Cleanup(func() {
		r.mu.Lock()
		delete(r.onTrigger, normalizeName(name))
		r.mu.Unlock()
	})
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
		// 2/2 and not a bare 0/0: a toughness-0 creature is destroyed
		// by SBAs during the first cascade's inline resolution, which
		// leaves the battlefield empty and makes a second cascade
		// silently dispatch nothing.
		Card: &gameengine.Card{Name: name, Types: []string{"creature"},
			BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0,
	})

	fireTrigger(gs, "creature_dies", map[string]interface{}{"seat": 0})

	if games, _, _, _, _ := gameengine.DepthCapStats(); games != 0 {
		t.Fatalf("%d games recorded a swallow on a cascade that never hit "+
			"the cap", games)
	}
	if s := gameengine.DepthCapSummary(); s != "" {
		t.Fatalf("summary = %q, want empty", s)
	}
}
