package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Route should match classifyDecision exactly when the caller passes
// confidenceNone (no signal) — the existing behavior must be preserved.
func TestRoute_NoConfidenceMatchesClassify(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(*testing.T) (*YggdrasilHat, *gameengine.GameState)
		expect DecisionTier
	}{
		{
			name: "budget-0 → Mjolnir",
			setup: func(t *testing.T) (*YggdrasilHat, *gameengine.GameState) {
				return newTierHat(0), newTierTestGame(t, 4)
			},
			expect: TierMjolnir,
		},
		{
			name: "evaluator budget → Gungnir",
			setup: func(t *testing.T) (*YggdrasilHat, *gameengine.GameState) {
				return newTierHat(100), newTierTestGame(t, 4)
			},
			expect: TierGungnir,
		},
		{
			name: "rollout budget + runner → Ragnarok",
			setup: func(t *testing.T) (*YggdrasilHat, *gameengine.GameState) {
				h := newTierHat(rolloutBudgetGe)
				h.TurnRunner = func(*gameengine.GameState) {}
				return h, newTierTestGame(t, 4)
			},
			expect: TierRagnarok,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, gs := c.setup(t)
			if got := h.Route(gs, confidenceNone); got != c.expect {
				t.Fatalf("Route(no-conf)=%s, want %s", got, c.expect)
			}
			// And it must equal a fresh classifyDecision call, modulo
			// the recorded tier counter.
			h2, gs2 := c.setup(t)
			if want := h2.classifyDecision(gs2); want != c.expect {
				t.Fatalf("classifyDecision=%s, want %s (test setup wrong)", want, c.expect)
			}
		})
	}
}

// Low confidence + budget that allows promotion should bump
// Mjolnir → Gungnir — but a budget-0 (permanent-Mjolnir) hat must NOT.
func TestRoute_LowConfidencePromotesMjolnirToGungnir(t *testing.T) {
	// r61 PR-8: a complex board no longer base-classifies as Mjolnir
	// (graceful degrade → Gungnir), so the permanent-Mjolnir base used
	// here is the budget-0 path. Budget=0 is permanent, so even at low
	// confidence the Mjolnir → Gungnir promotion must NOT fire (the guard
	// `h.Budget > 0` blocks it).
	h0 := newTierHat(0)
	gs2 := newTierTestGame(t, 4)
	if base := h0.classifyDecision(gs2); base != TierMjolnir {
		t.Fatalf("test setup: budget-0 hat should base-classify as Mjolnir; got %s", base)
	}
	if got := h0.Route(gs2, 0.1); got != TierMjolnir {
		t.Fatalf("low conf on budget-0 hat: got %s, want Mjolnir (budget guards)", got)
	}

	// A complex non-high-stakes board now degrades to Gungnir (not
	// Mjolnir). Low confidence must NOT promote it further to Ragnarok —
	// the budget taper's perf bound must hold even under the promotion
	// path (Route guards Gungnir → Ragnarok with the complexity gate).
	hC := newTierHat(rolloutBudgetGe)
	hC.TurnRunner = func(*gameengine.GameState) {}
	gsC := newTierTestGame(t, 4)
	per := adaptiveBudgetComplexityThreshold/len(gsC.Seats) + 2
	for _, s := range gsC.Seats {
		for i := 0; i < per; i++ {
			s.Battlefield = append(s.Battlefield, &gameengine.Permanent{
				Card: &gameengine.Card{Name: "filler"}, Owner: s.Idx, Controller: s.Idx,
			})
		}
	}
	if base := hC.classifyDecision(gsC); base != TierGungnir {
		t.Fatalf("test setup: complex board should degrade to Gungnir; got %s", base)
	}
	if got := hC.Route(gsC, 0.1); got != TierGungnir {
		t.Fatalf("low conf on complex board: got %s, want Gungnir (rollout perf-gated)", got)
	}

	// A non-complex, budget>0 hat already classifies as Gungnir; no-conf
	// must NOT promote it.
	hG := newTierHat(100)
	gs3 := newTierTestGame(t, 4)
	if got := hG.Route(gs3, confidenceNone); got != TierGungnir {
		t.Fatalf("no-conf base-Gungnir: got %s, want Gungnir", got)
	}
}

// Low confidence + rollout-capable budget should bump
// Gungnir → Ragnarok when classify returns Gungnir.
func TestRoute_LowConfidencePromotesGungnirToRagnarok(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(rolloutBudgetGe)
	h.TurnRunner = func(*gameengine.GameState) {}
	// Hat with rollout budget normally classifies as Ragnarok. Force
	// classify to Gungnir by exhausting the turn budget below the
	// rollout cost, but not to zero.
	h.TurnBudget = rolloutEvalCost + 1
	h.spendTurnBudget(gs, 2) // remaining = rolloutEvalCost - 1
	if h.classifyDecision(gs) != TierGungnir {
		t.Fatalf("test setup: classify should be Gungnir, got %s", h.classifyDecision(gs))
	}
	// Low confidence shouldn't override the turn-budget gate either —
	// remaining < rolloutEvalCost means we can't afford a rollout.
	if got := h.Route(gs, 0.1); got != TierGungnir {
		t.Fatalf("low conf with insufficient turn-budget: got %s, want Gungnir", got)
	}
}

// confidence ≥ confidenceNone (the sentinel) must never promote.
func TestRoute_HighConfidenceNeverPromotes(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	if got := h.Route(gs, confidenceNone); got != TierGungnir {
		t.Fatalf("got %s, want Gungnir (no promotion at conf=1.0)", got)
	}
	if got := h.Route(gs, 2.0); got != TierGungnir {
		t.Fatalf("got %s, want Gungnir (no promotion above 1.0)", got)
	}
}

// Route increments the tier counter exactly once per call.
func TestRoute_RecordsTierExactlyOnce(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	before := h.MjolnirStats().Total()
	tier := h.Route(gs, confidenceNone)
	after := h.MjolnirStats().Total()
	if after-before != 1 {
		t.Fatalf("Route should record one tier: before=%d after=%d", before, after)
	}
	if tier != TierGungnir {
		t.Fatalf("returned tier %s, want Gungnir", tier)
	}
}

// DispatchTier picks the closure for the chosen tier.
func TestDispatchTier_PicksCorrectClosure(t *testing.T) {
	mj := func() string { return "mjolnir" }
	gn := func() string { return "gungnir" }
	rg := func() string { return "ragnarok" }

	if got := DispatchTier(TierMjolnir, mj, gn, rg); got != "mjolnir" {
		t.Fatalf("Mjolnir→%q, want mjolnir", got)
	}
	if got := DispatchTier(TierGungnir, mj, gn, rg); got != "gungnir" {
		t.Fatalf("Gungnir→%q, want gungnir", got)
	}
	if got := DispatchTier(TierRagnarok, mj, gn, rg); got != "ragnarok" {
		t.Fatalf("Ragnarok→%q, want ragnarok", got)
	}
}

// DispatchTier degrades when a higher-tier closure is nil.
func TestDispatchTier_DegradesOnNil(t *testing.T) {
	mj := func() int { return 1 }
	gn := func() int { return 2 }

	// Ragnarok requested, only Mjolnir + Gungnir provided → Gungnir.
	if got := DispatchTier(TierRagnarok, mj, gn, nil); got != 2 {
		t.Fatalf("Ragnarok degrade with no rg: got %d, want 2 (gungnir)", got)
	}
	// Ragnarok requested, only Mjolnir provided → Mjolnir.
	if got := DispatchTier(TierRagnarok, mj, nil, nil); got != 1 {
		t.Fatalf("Ragnarok degrade with only mj: got %d, want 1", got)
	}
	// Gungnir requested, no Gungnir closure → Mjolnir.
	if got := DispatchTier(TierGungnir, mj, nil, nil); got != 1 {
		t.Fatalf("Gungnir degrade: got %d, want 1", got)
	}
}

// DispatchTier returns the zero value when even Mjolnir is nil — the
// only path that should never happen in production but we keep it safe.
func TestDispatchTier_AllNilReturnsZero(t *testing.T) {
	if got := DispatchTier[int](TierMjolnir, nil, nil, nil); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
	type result struct{ ok bool }
	if got := DispatchTier[result](TierGungnir, nil, nil, nil); (got != result{}) {
		t.Fatalf("got %+v, want zero", got)
	}
}
