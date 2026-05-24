package gameengine

// R41 — replacement / continuous-effect dependency loop purification.
//
// Humility (creatures lose abilities, become 1/1) and Opalescence
// (every other non-Aura enchantment is a creature with P/T = CMC)
// form the canonical §613.8 circular dependency:
//
//   - Opalescence (layer 4) makes Humility a creature.
//   - That, in turn, would make Humility's own layer-6 strip apply
//     to Humility itself, which would erase the very ability that
//     enabled the layer-6 effect — a textbook dependency cycle.
//
// §613.8b prescribes timestamp order when dependencies form a cycle,
// so the engine MUST NOT attempt to keep re-resolving until a fixed
// point — that path leads to unbounded recursion / stack overflow.
//
// The existing layers_test.go covers the static expected final
// characteristics. This file adds the *termination & stability*
// regression suite: the engine should remain stable across cache
// invalidations, repeated queries, deep stacks of paradox
// enchantments, and register/unregister churn — without ever
// blowing the stack or hanging.
//
// Rule citations:
//   §613.1   — layer ordering
//   §613.8a  — dependency reordering within a layer
//   §613.8b  — circular dependency → timestamp fallback
//   §613.7   — timestamp tiebreak

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// runWithTimeout fails the test if fn does not return within d. Used
// to catch any infinite-recursion regression in the dependency
// solver — a hang here would otherwise just exhaust the test runner.
func runWithTimeout(t *testing.T, d time.Duration, label string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	var once sync.Once
	go func() {
		defer once.Do(func() { close(done) })
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		buf := make([]byte, 1<<14)
		n := runtime.Stack(buf, false)
		t.Fatalf("%s did not terminate within %s — possible §613.8 dependency loop\n%s",
			label, d, buf[:n])
	}
}

// -----------------------------------------------------------------------------
// Termination: repeated queries under the paradox must return promptly
// and identically.
// -----------------------------------------------------------------------------

func TestR41_HumilityOpalescence_RepeatedQueriesTerminate(t *testing.T) {
	gs := newFixtureGame(t)
	hum := layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	hum.Flags["cmc"] = 4
	opal := layerAt(gs, 0, "Opalescence", 0, 0, "enchantment")
	opal.Flags["cmc"] = 4
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	runWithTimeout(t, 2*time.Second, "repeated-query paradox", func() {
		for i := 0; i < 5000; i++ {
			_ = GetEffectiveCharacteristics(gs, hum)
			_ = GetEffectiveCharacteristics(gs, opal)
			_ = GetEffectiveCharacteristics(gs, bear)
		}
	})

	// Stability: final state must match the canonical paradox resolution.
	hc := GetEffectiveCharacteristics(gs, hum)
	if hc.Power != 4 || hc.Toughness != 4 {
		t.Errorf("Humility under Opalescence: expected 4/4, got %d/%d", hc.Power, hc.Toughness)
	}
	if len(hc.Abilities) != 0 {
		t.Errorf("Humility should still self-strip abilities, got %d", len(hc.Abilities))
	}
	oc := GetEffectiveCharacteristics(gs, opal)
	if charsHaveType(oc.Types, "creature") {
		t.Errorf("Opalescence self-exclusion broken after repeated queries; types=%v", oc.Types)
	}
	bc := GetEffectiveCharacteristics(gs, bear)
	if bc.Power != 1 || bc.Toughness != 1 {
		t.Errorf("Bears under Humility: expected 1/1, got %d/%d", bc.Power, bc.Toughness)
	}
}

// -----------------------------------------------------------------------------
// Cache-epoch churn: bumping the cache epoch between every query forces
// a full recompute on each call. The paradox must not cascade into
// runaway recursion when no entry is memoized.
// -----------------------------------------------------------------------------

func TestR41_HumilityOpalescence_CacheChurnTerminates(t *testing.T) {
	gs := newFixtureGame(t)
	hum := layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	hum.Flags["cmc"] = 4
	opal := layerAt(gs, 0, "Opalescence", 0, 0, "enchantment")
	opal.Flags["cmc"] = 4

	runWithTimeout(t, 2*time.Second, "cache-churn paradox", func() {
		for i := 0; i < 2000; i++ {
			gs.InvalidateCharacteristicsCache()
			_ = GetEffectiveCharacteristics(gs, hum)
			_ = GetEffectiveCharacteristics(gs, opal)
		}
	})

	// Idempotency: every iteration above must compute the same answer.
	hc := GetEffectiveCharacteristics(gs, hum)
	if hc.Power != 4 || hc.Toughness != 4 {
		t.Errorf("Humility 4/4 invariant broke under cache churn: %d/%d", hc.Power, hc.Toughness)
	}
}

// -----------------------------------------------------------------------------
// Register/unregister churn: tearing down and re-registering the paradox
// many times must not leak effects or destabilize the dependency graph.
// -----------------------------------------------------------------------------

func TestR41_HumilityOpalescence_RegisterChurn(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	runWithTimeout(t, 3*time.Second, "register churn", func() {
		for i := 0; i < 200; i++ {
			hum := addBattlefield(gs, 0, "Humility", 0, 0, "enchantment")
			hum.Flags["cmc"] = 4
			RegisterContinuousEffectsForPermanent(gs, hum)
			opal := addBattlefield(gs, 0, "Opalescence", 0, 0, "enchantment")
			opal.Flags["cmc"] = 4
			RegisterContinuousEffectsForPermanent(gs, opal)

			_ = GetEffectiveCharacteristics(gs, hum)
			_ = GetEffectiveCharacteristics(gs, opal)
			_ = GetEffectiveCharacteristics(gs, bear)

			gs.UnregisterContinuousEffectsForPermanent(hum)
			gs.UnregisterContinuousEffectsForPermanent(opal)
			// Drop the permanents from the battlefield so the next cycle
			// doesn't accumulate stale Humility/Opalescence pointers that
			// no longer have continuous effects backing them.
			gs.Seats[0].Battlefield = gs.Seats[0].Battlefield[:1]
		}
	})

	// With both halves of the paradox removed, the bear should be back
	// to its printed 2/2.
	bc := GetEffectiveCharacteristics(gs, bear)
	if bc.Power != 2 || bc.Toughness != 2 {
		t.Errorf("Bears after final unregister: expected 2/2, got %d/%d", bc.Power, bc.Toughness)
	}
}

// -----------------------------------------------------------------------------
// N-stack paradox: multiple Humilities + multiple Opalescences on the
// battlefield form an N×M dependency graph. The single-copy paradox
// resolution (each Opalescence self-excludes) breaks down here: with
// 2+ Opalescences, every Opalescence is "other" to the others and so
// all of them become creatures. The point of this test is purely
// engine stability — termination, idempotency, deterministic answer,
// and an absent stack blow-up.
// -----------------------------------------------------------------------------

func TestR41_HumilityOpalescence_DeepStack(t *testing.T) {
	gs := newFixtureGame(t)

	const N = 12
	hums := make([]*Permanent, 0, N)
	opals := make([]*Permanent, 0, N)
	for i := 0; i < N; i++ {
		h := layerAt(gs, i%2, "Humility", 0, 0, "enchantment")
		h.Flags["cmc"] = 4
		hums = append(hums, h)
		o := layerAt(gs, i%2, "Opalescence", 0, 0, "enchantment")
		o.Flags["cmc"] = 4
		opals = append(opals, o)
	}
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	runWithTimeout(t, 3*time.Second, "deep-stack paradox", func() {
		for _, h := range hums {
			_ = GetEffectiveCharacteristics(gs, h)
		}
		for _, o := range opals {
			_ = GetEffectiveCharacteristics(gs, o)
		}
		_ = GetEffectiveCharacteristics(gs, bear)
	})

	// The one invariant that survives the deep stack: Humility still
	// strips abilities + pins non-enchantment creatures to 1/1, because
	// no Opalescence L7b matches the bear (predicate fails). If the
	// dependency solver were oscillating, this would jitter.
	bc := GetEffectiveCharacteristics(gs, bear)
	if bc.Power != 1 || bc.Toughness != 1 {
		t.Errorf("Bears under deep stack: expected 1/1, got %d/%d", bc.Power, bc.Toughness)
	}
	if len(bc.Abilities) != 0 {
		t.Errorf("Bears under deep stack should have no abilities, got %d", len(bc.Abilities))
	}

	// Determinism: a second pass under a fresh cache epoch must produce
	// byte-identical P/T for every permanent. A dependency-cycle bug
	// would manifest as iteration-order-dependent outputs here.
	want := map[*Permanent][2]int{}
	for _, p := range append(append([]*Permanent{}, hums...), opals...) {
		c := GetEffectiveCharacteristics(gs, p)
		want[p] = [2]int{c.Power, c.Toughness}
	}
	for i := 0; i < 3; i++ {
		gs.InvalidateCharacteristicsCache()
		for p, exp := range want {
			c := GetEffectiveCharacteristics(gs, p)
			if c.Power != exp[0] || c.Toughness != exp[1] {
				t.Fatalf("deep-stack determinism broke on pass %d for %s: was %d/%d now %d/%d",
					i, p.Card.Name, exp[0], exp[1], c.Power, c.Toughness)
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Stack-depth probe: capture goroutine stack size before/after a query
// under the paradox. A regression that re-enters GetEffectiveCharacteristics
// recursively per dependency layer would manifest as the stack growing
// proportional to the number of effects. We don't pin a hard byte limit
// (Go's stack-grow heuristics are version-sensitive) — but we do require
// the function to RETURN at all, and we sanity-check the post-call stack
// is within a generous bound (16 MiB) of pre-call.
// -----------------------------------------------------------------------------

func TestR41_HumilityOpalescence_BoundedStackGrowth(t *testing.T) {
	gs := newFixtureGame(t)
	hum := layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	hum.Flags["cmc"] = 4
	opal := layerAt(gs, 0, "Opalescence", 0, 0, "enchantment")
	opal.Flags["cmc"] = 4
	for i := 0; i < 20; i++ {
		e := addBattlefield(gs, 0, "Sylvan Library", 0, 0, "enchantment")
		e.Flags["cmc"] = 3
		RegisterContinuousEffectsForPermanent(gs, e)
	}

	var pre, post runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&pre)

	runWithTimeout(t, 2*time.Second, "bounded-stack paradox", func() {
		for _, p := range gs.Seats[0].Battlefield {
			_ = GetEffectiveCharacteristics(gs, p)
		}
	})

	runtime.GC()
	runtime.ReadMemStats(&post)

	// A stack-overflow-or-near regression would balloon StackInuse far
	// beyond this. 16 MiB is generous; healthy paths stay well under 1 MiB.
	const maxGrowth uint64 = 16 << 20
	if post.StackInuse > pre.StackInuse && post.StackInuse-pre.StackInuse > maxGrowth {
		t.Errorf("StackInuse grew %d bytes under paradox queries (pre=%d post=%d) — possible recursion regression",
			post.StackInuse-pre.StackInuse, pre.StackInuse, post.StackInuse)
	}
}
