package hat

import "github.com/hexdek/hexdek/internal/gameengine"

// Decision routing — Mjolnir / Gungnir / Ragnarok.
//
// classifyDecision (in yggdrasil.go) is the existing complexity-only
// scorer: it considers the hat's hard budget, total board permanents,
// per-turn evaluation budget, and combo-assembly state. This file
// adds two pieces of formal infrastructure that the engine has so far
// been doing ad hoc at every Choose* call site:
//
//   1. Route(gs, confidence)   — single-call classify + record + return,
//                                with optional confidence-based tier
//                                promotion. Replaces the boilerplate
//                                `recordDecisionTier(classifyDecision(gs))`
//                                pattern.
//
//   2. DispatchTier[T any](...) — generic per-tier dispatcher so a
//                                decision site can declare three closures
//                                (one per tier) and let the router pick.
//
// The composite scoring follows the user-facing contract:
//   * Mjolnir (~90%): budget-0 heuristic, exhausted budgets, complex
//                     boards (opponents have set the pace).
//   * Gungnir (~9%):  evaluator + UCB1 — the meat of complex but
//                     non-terminal decisions.
//   * Ragnarok (~1%): MCTS rollouts — game-deciding turns, combo windows.

const (
	// confidencePromoteFloor: a caller-supplied confidence below this
	// threshold promotes a base-Mjolnir result to Gungnir when budget
	// allows. The caller should pass 0 (or any value ≥ confidenceNone)
	// when they have no confidence signal — promotion is opt-in.
	confidencePromoteFloor = 0.50

	// confidenceForceRagnarokFloor: very low confidence promotes a
	// base-Gungnir result to Ragnarok when budget allows.
	confidenceForceRagnarokFloor = 0.25

	// confidenceNone is the sentinel passed by call sites that have no
	// confidence signal. Anything ≥ confidenceNone disables promotion
	// so the route matches classifyDecision exactly.
	confidenceNone = 1.0
)

// Route classifies the current decision context, optionally promotes
// the resulting tier when caller confidence is low, records the chosen
// tier in the per-game counters, and returns it.
//
// `confidence` is the caller's [0..1] estimate of how sure the cheap
// tier (Mjolnir) would be on this decision. Examples:
//
//   * Cast-from-hand with one obvious top candidate: confidence ≈ 0.9.
//   * Combat block where two distinct line-ups look ~equal: ≈ 0.4.
//   * No signal: pass confidenceNone (or any value ≥ 1.0) — the result
//     matches classifyDecision exactly. This is the safe default for
//     call sites that haven't been instrumented yet.
//
// Promotion only goes upward and only when the structural budget allows
// it — we never invent a TurnRunner or override turn-budget exhaustion.
func (h *YggdrasilHat) Route(gs *gameengine.GameState, confidence float64) DecisionTier {
	if h == nil {
		return TierMjolnir
	}
	base := h.classifyDecision(gs)
	final := base

	// Promote on low confidence, but only when the next tier up is
	// structurally reachable from the current hat's budget. The
	// classifyDecision result already encodes those structural caps,
	// so we promote one step at a time and re-check via classifyDecision-
	// equivalent gates inline.
	if confidence < confidencePromoteFloor && base == TierMjolnir {
		// Mjolnir → Gungnir requires budget > 0. Mjolnir is also the
		// *deliberate* result on complex boards / exhausted turn
		// budgets — those callers know they can't afford more compute,
		// so we don't override.
		if h.Budget > 0 && !h.boardComplexityForcesMjolnir(gs) && !h.turnBudgetExhausted(gs) {
			final = TierGungnir
		}
	}
	if confidence < confidenceForceRagnarokFloor && final == TierGungnir {
		// Gungnir → Ragnarok requires the rollout precondition (budget
		// ≥ rolloutBudgetGe + TurnRunner set) and enough turn budget
		// for at least one rollout's eval cost. r61 PR-8: a complex non-
		// high-stakes board now base-classifies as Gungnir (graceful
		// degrade) rather than Mjolnir, so guard the rollout promotion
		// with the same complexity gate — we must NOT re-enable expensive
		// rollouts on a huge board via the low-confidence promotion path,
		// or the budget taper's perf bound is defeated.
		if h.Budget >= rolloutBudgetGe && h.TurnRunner != nil &&
			!h.boardComplexityForcesMjolnir(gs) {
			if h.TurnBudget <= 0 || h.turnRemaining(gs) >= rolloutEvalCost {
				final = TierRagnarok
			}
		}
	}

	h.recordDecisionTier(final)
	return final
}

// boardComplexityForcesMjolnir mirrors the gate inside classifyDecision
// so Route can check it without re-running the full classifier.
func (h *YggdrasilHat) boardComplexityForcesMjolnir(gs *gameengine.GameState) bool {
	if gs == nil {
		return false
	}
	if h.comboAssembling(gs) {
		return false
	}
	total := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		total += len(s.Battlefield)
	}
	return total >= adaptiveBudgetComplexityThreshold
}

// turnBudgetExhausted reports whether the per-turn evaluation budget
// has been spent down to zero (and we shouldn't promote past Mjolnir).
// Combo-assembly state overrides exhaustion — we always pay for the
// winning turn.
func (h *YggdrasilHat) turnBudgetExhausted(gs *gameengine.GameState) bool {
	if h.TurnBudget <= 0 {
		return false
	}
	if h.comboAssembling(gs) {
		return false
	}
	return h.turnRemaining(gs) <= 0
}

// DispatchTier picks the right per-tier closure and returns its result.
// Generic over the return type so a decision site can keep its native
// shape (a *Card, a []*Permanent, a Target, etc.) without boxing.
//
// Use:
//
//	tier := h.Route(gs, confidence)
//	chosen := DispatchTier(tier,
//	    func() *Card { return mjolnirHeuristic(gs)    },
//	    func() *Card { return gungnirEvaluator(gs)    },
//	    func() *Card { return ragnarokRollout(gs)     },
//	)
//
// A nil closure for a tier degrades down: Ragnarok→Gungnir→Mjolnir.
// The Mjolnir closure must be non-nil (it's the cheap path that always
// has to work).
func DispatchTier[T any](tier DecisionTier, mjolnir, gungnir, ragnarok func() T) T {
	switch tier {
	case TierRagnarok:
		if ragnarok != nil {
			return ragnarok()
		}
		fallthrough
	case TierGungnir:
		if gungnir != nil {
			return gungnir()
		}
		fallthrough
	default:
		if mjolnir == nil {
			var zero T
			return zero
		}
		return mjolnir()
	}
}
