package gameengine

// R60 — §614 / §616 replacement-effect chain edge cases.
//
// The r41 suite (`replacement_loop_r41_test.go`) pins termination for
// §613.8 continuous-effect dependency loops at the layer system. This
// file pins the *replacement-event* side of "loop / chain" handling:
//
//   - §616.1f safety cap actually engages on a saturated chain
//     (many distinct §614 replacements, all applicable, can't be
//     short-circuited by the §614.5 applied-once guard alone).
//
//   - §614.6 modified-event re-applicability: a replacement that
//     mutates ev.Payload must expose downstream replacements whose
//     `Applies` predicate only matches the post-mutation event —
//     i.e. `Applies` is re-evaluated every iteration, not memoized.
//
// Rule citations:
//   §614.5  — applied-once
//   §614.6  — modified event carries forward
//   §616.1  — category ordering (self-replacement before other)
//   §616.1f — iterate until no replacement applies (engine caps at 64)

import (
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Saturated chain — 80 distinct adder replacements should hit the §616.1f
// safety cap, not loop forever. Each registered effect has a unique
// HandlerID, so the §614.5 applied-once guard does NOT short-circuit the
// chain (that guard only suppresses re-application of the same handler).
// -----------------------------------------------------------------------------

func TestR60_SaturatedReplacementChain_HitsIterationCap(t *testing.T) {
	gs := newFixtureGame(t)

	const N = 80 // > maxReplacementIterations (64)
	for i := 0; i < N; i++ {
		id := "test:adder:" + itoaRepl(i)
		gs.RegisterReplacement(&ReplacementEffect{
			EventType: "would_gain_life",
			HandlerID: id,
			Category:  CategoryOther,
			Timestamp: i, // deterministic pick order
			Applies:   func(_ *GameState, ev *ReplEvent) bool { return ev.Count() > 0 },
			ApplyFn:   func(_ *GameState, ev *ReplEvent) { ev.SetCount(ev.Count() + 1) },
		})
	}

	ev := NewReplEvent("would_gain_life")
	ev.TargetSeat = 0
	ev.SetCount(1)

	runWithTimeout(t, 2*time.Second, "saturated replacement chain", func() {
		FireEvent(gs, ev)
	})

	// Cap engages at exactly maxReplacementIterations: 1 (initial) + 64 (applied) = 65.
	want := 1 + maxReplacementIterations
	if ev.Count() != want {
		t.Fatalf("expected count %d after §616.1f cap, got %d", want, ev.Count())
	}
	if ev.Cancelled {
		t.Fatal("event should not have been cancelled")
	}
	if countEvents(gs, "replacement_iteration_cap") != 1 {
		t.Fatalf("expected exactly one replacement_iteration_cap log, got %d",
			countEvents(gs, "replacement_iteration_cap"))
	}
	cap := lastEventOfKind(gs, "replacement_iteration_cap")
	if cap.Source != "would_gain_life" {
		t.Errorf("cap event Source should be event type, got %q", cap.Source)
	}
	if cap.Details == nil || cap.Details["rule"] != "616.1f" {
		t.Errorf("cap event should cite rule 616.1f, got Details=%v", cap.Details)
	}
}

// -----------------------------------------------------------------------------
// "Instead" chain re-applicability — replacement A mutates the event
// payload such that replacement B (initially inapplicable) becomes
// applicable on the next iteration. Tests that pickReplacement re-runs
// `Applies` per iteration rather than caching the first-pass result.
//
// Concrete shape: a self-replacement promotes counter_type "loyalty" →
// "+1/+1". Hardened Scales only matches "+1/+1" — without per-iteration
// re-evaluation it would never fire on this event.
// -----------------------------------------------------------------------------

func TestR60_InsteadChain_PromotedCounterTypeFiresDownstream(t *testing.T) {
	gs := newFixtureGame(t)
	scales := registerAt(gs, 0, "Hardened Scales", 0, 0, "enchantment")
	if scales == nil {
		t.Fatal("Hardened Scales should register")
	}

	// Self-replacement (§616.1a) runs before "other" — it rewrites the
	// counter type so the downstream Hardened Scales handler now matches.
	gs.RegisterReplacement(&ReplacementEffect{
		EventType: "would_put_counter",
		HandlerID: "test:promote_loyalty_to_p1p1",
		Category:  CategorySelfReplacement,
		Timestamp: 0,
		Applies: func(_ *GameState, ev *ReplEvent) bool {
			return ev.String("counter_type") == "loyalty"
		},
		ApplyFn: func(_ *GameState, ev *ReplEvent) {
			ev.Payload["counter_type"] = "+1/+1"
		},
	})

	target := addBattlefield(gs, 0, "Walking Ballista", 0, 0, "creature", "artifact")

	runWithTimeout(t, 1*time.Second, "instead chain re-applicability", func() {
		n, cancelled := FirePutCounterEvent(gs, target, "loyalty", 2, nil)
		if cancelled {
			t.Fatal("event should not be cancelled")
		}
		// Promote runs first (no count change), then Scales adds 1 → 3.
		// If Applies were cached from the first pass Scales would skip and
		// count would stay at 2.
		if n != 3 {
			t.Fatalf("expected promote+scales chain → 3 counters, got %d", n)
		}
	})

	// Verify both fired, in the correct category order: self-replacement
	// must precede the "other"-category Hardened Scales.
	saw := []string{}
	for _, e := range gs.EventLog {
		if e.Kind != "replacement_applied" {
			continue
		}
		saw = append(saw, e.Source)
	}
	if len(saw) < 1 || saw[len(saw)-1] != "Hardened Scales" {
		t.Errorf("Hardened Scales must apply LAST (after self-replacement promote); got %v", saw)
	}

	// Idempotency: a second fire on a separate event must produce the same
	// answer — AppliedIDs is per-event, so both effects re-engage.
	n2, _ := FirePutCounterEvent(gs, target, "loyalty", 2, nil)
	if n2 != 3 {
		t.Fatalf("second fire should also yield 3, got %d", n2)
	}
}
