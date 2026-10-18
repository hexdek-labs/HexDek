package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r186 — the per-card half: these drive the REAL fireTrigger, so a
// control that deletes a production call site goes red somewhere
// (r163's lesson, restated by r182's per-card file).

func r186percardReset(t *testing.T) {
	t.Helper()
	reset := func() {
		gameengine.ResetDepthCapForTest()
		gameengine.ResetCascadeShapeForTest()
		gameengine.ResetTriggerDepthForTest()
		gameengine.ResetLoopGuardCountsForTest()
		gameengine.ResetLoopGuardLabelsForTest()
	}
	reset()
	t.Cleanup(reset)
}

// r186brancher registers a handler on evt that re-fires evt TWICE.
// One swallow per cascade is not enough to classify anything, so the
// branching factor is what makes the depth-9 frontier wide: 2^7 = 128
// nodes at depth 8, each attempting two dispatches, = 256 swallows.
func r186brancher(t *testing.T, gs *gameengine.GameState, name, evt string) {
	t.Helper()
	r := Global()
	r.OnTrigger(name, evt, func(g *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fireTrigger(g, evt, map[string]interface{}{"seat": 0})
		fireTrigger(g, evt, map[string]interface{}{"seat": 0})
	})
	t.Cleanup(func() {
		r.mu.Lock()
		delete(r.onTrigger, normalizeName(name))
		r.mu.Unlock()
	})
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
		Card: &gameengine.Card{Name: name, Types: []string{"creature"},
			BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0,
	})
}

// r186pad adds n inert 2/2s with no registered handler. They widen
// the board without widening the cascade, which is the only way to
// drive the two axes independently through the real dispatcher.
func r186pad(gs *gameengine.GameState, n int) {
	for i := 0; i < n; i++ {
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
			Card: &gameengine.Card{Name: "ZZ R186 Inert", Types: []string{"creature"},
				BasePower: 2, BaseToughness: 2},
			Controller: 0, Owner: 0,
		})
	}
}

// Unbounded re-entry through the real dispatcher: 256 swallows against
// a board of one. Nothing on a one-permanent board produces 256
// dispatches, so the board axis must say so.
func TestR186_FireTriggerClassifiesReentry(t *testing.T) {
	r186percardReset(t)
	gs := newTestGS(2)
	gs.EventPolicy = gameengine.EventLogNone

	r186brancher(t, gs, "ZZ R186 Branch", "creature_dies")
	fireTrigger(gs, "creature_dies", map[string]interface{}{"seat": 0})

	swallowed := gs.DepthCapSwallowedCount()
	if swallowed != 256 {
		t.Fatalf("swallowed %d, want 256 — this test is not exercising "+
			"what it thinks", swallowed)
	}
	shape, distinct, domPct, boardMax := gs.DepthCapPeakCascadeShape()
	if shape != "loop1" {
		t.Fatalf("shape = %q, want \"loop1\" (256 swallows, board %d)",
			shape, boardMax)
	}
	if distinct != 1 || domPct != 100 {
		t.Fatalf("distinct=%d dom=%d%%, want 1/100", distinct, domPct)
	}
	if boardMax != 1 {
		t.Fatalf("boardMax = %d, want 1 — the production call site does "+
			"not sample the real battlefield", boardMax)
	}
}

// The SAME cascade, the same event name, the same swallow count — on a
// board wide enough to have supplied every one of those dispatches.
// This is (a) versus (c) driven through the real dispatcher, and the
// only thing that differs is the board.
func TestR186_FireTriggerClassifiesAWideBoardAsAWave(t *testing.T) {
	r186percardReset(t)
	gs := newTestGS(2)
	gs.EventPolicy = gameengine.EventLogNone

	r186brancher(t, gs, "ZZ R186 Wide", "creature_dies")
	r186pad(gs, 300)
	fireTrigger(gs, "creature_dies", map[string]interface{}{"seat": 0})

	if got := gs.DepthCapSwallowedCount(); got != 256 {
		t.Fatalf("swallowed %d, want 256 — the padding changed the cascade, "+
			"not just the board", got)
	}
	shape, distinct, _, boardMax := gs.DepthCapPeakCascadeShape()
	if boardMax != 301 {
		t.Fatalf("boardMax = %d, want 301", boardMax)
	}
	if distinct != 1 {
		t.Fatalf("distinct = %d, want 1 — identical to the loop case, "+
			"which is exactly why the name axis cannot separate them",
			distinct)
	}
	if shape != "wave" {
		t.Fatalf("shape = %q, want \"wave\": 256 swallows on a board of "+
			"301 is bounded", shape)
	}
}

// KNOWN LIMIT, PINNED ON PURPOSE. A strictly alternating two-event
// cycle (A fires B fires A ...) is classified loop1, NOT loopN: every
// depth-9 attempt comes from a depth-8 node, all of which share the
// same parity, so only ONE of the two event names is ever swallowed.
// The composition is over SWALLOWED events, not over the chain.
//
// This test exists so that nobody reads loop1 as "self-feeding". It
// means "unbounded re-entry whose swallowed dispatches all carried one
// event name", which a mutual pair satisfies whenever its branches are
// the same length.
func TestR186_StrictAlternationIsIndistinguishableFromSelfFeeding(t *testing.T) {
	r186percardReset(t)
	gs := newTestGS(2)
	gs.EventPolicy = gameengine.EventLogNone

	r := Global()
	const nameA, nameB = "ZZ R186 Alt A", "ZZ R186 Alt B"
	r.OnTrigger(nameA, "creature_dies", func(g *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fireTrigger(g, "nonland_permanent_etb", map[string]interface{}{"seat": 0})
		fireTrigger(g, "nonland_permanent_etb", map[string]interface{}{"seat": 0})
	})
	r.OnTrigger(nameB, "nonland_permanent_etb", func(g *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fireTrigger(g, "creature_dies", map[string]interface{}{"seat": 0})
		fireTrigger(g, "creature_dies", map[string]interface{}{"seat": 0})
	})
	t.Cleanup(func() {
		r.mu.Lock()
		delete(r.onTrigger, normalizeName(nameA))
		delete(r.onTrigger, normalizeName(nameB))
		r.mu.Unlock()
	})
	for _, n := range []string{nameA, nameB} {
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
			Card: &gameengine.Card{Name: n, Types: []string{"creature"},
				BasePower: 2, BaseToughness: 2},
			Controller: 0, Owner: 0,
		})
	}

	fireTrigger(gs, "creature_dies", map[string]interface{}{"seat": 0})

	shape, distinct, domPct, _ := gs.DepthCapPeakCascadeShape()
	if distinct != 1 || domPct != 100 {
		t.Fatalf("distinct=%d dom=%d%%, want 1/100 — if this ever reports "+
			"2, the frontier stopped being uniform-depth and the header's "+
			"stated limitation needs rewriting, not this test", distinct, domPct)
	}
	if shape != "loop1" {
		t.Fatalf("shape = %q, want \"loop1\" — a strict alternation is "+
			"reported as one-name re-entry, and the shape names say "+
			"\"loop1/loopN\" rather than \"self/mutual\" for exactly "+
			"this reason", shape)
	}
}

// The normal path must stay silent: no cascade classified, no field
// rendered, on a cascade that never hits the cap.
func TestR186_ShallowCascadeClassifiesNothing(t *testing.T) {
	r186percardReset(t)
	gs := newTestGS(2)
	gs.EventPolicy = gameengine.EventLogNone

	r := Global()
	const name = "ZZ R186 Shallow"
	r.OnTrigger(name, "creature_dies", func(g *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {})
	t.Cleanup(func() {
		r.mu.Lock()
		delete(r.onTrigger, normalizeName(name))
		r.mu.Unlock()
	})
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
		Card: &gameengine.Card{Name: name, Types: []string{"creature"},
			BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0,
	})

	fireTrigger(gs, "creature_dies", map[string]interface{}{"seat": 0})

	if shape, _, _, _ := gs.DepthCapPeakCascadeShape(); shape != "" {
		t.Fatalf("shape = %q on a cascade that never capped", shape)
	}
	if s := gameengine.DepthCapShapeSummary(); s != "" {
		t.Fatalf("capshape summary = %q, want empty", s)
	}
}
